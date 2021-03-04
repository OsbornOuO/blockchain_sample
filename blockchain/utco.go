package blockchain

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/dgraph-io/badger"
)

var (
	utxoPrefix   = []byte("utxo-")
	prefixLength = len(utxoPrefix)
)

// UTXOSet Unspent transaction output
type UTXOSet struct {
	BlockChain *BlockChain
}

func (u *UTXOSet) DeleteByPrefix(prefix []byte) {
	deleteKeys := func(keysForDelete [][]byte) error {
		if err := u.BlockChain.Database.Update(func(txn *badger.Txn) error {
			for _, key := range keysForDelete {
				if err := txn.Delete(key); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
		return nil
	}

	collectSize := 100000
	u.BlockChain.Database.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		keysForDelete := make([][]byte, 0, collectSize)
		keysCollected := 0

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().KeyCopy(nil)
			keysForDelete = append(keysForDelete, key)
			keysCollected++
			if keysCollected == collectSize {
				if err := deleteKeys(keysForDelete); err != nil {
					log.Panic("fail to delete keys", err)
				}
				keysForDelete = make([][]byte, 0, collectSize)
				keysCollected = 0
			}
		}
		if keysCollected > 0 {
			if err := deleteKeys(keysForDelete); err != nil {
				log.Panic(err)
			}
		}

		return nil
	})
}

func (u UTXOSet) ReIndex() {
	db := u.BlockChain.Database

	u.DeleteByPrefix(utxoPrefix)

	UTXO := u.BlockChain.FindUTXO()

	err := db.Update(func(txn *badger.Txn) error {
		for txID, outs := range UTXO {
			key, err := hex.DecodeString(txID)
			ErrHandler(err)

			key = append(utxoPrefix, key...)
			err = txn.Set(key, outs.Serialize())
			ErrHandler(err)
		}
		return nil
	})
	ErrHandler(err)
}

func (u *UTXOSet) Update(block *Block) {
	db := u.BlockChain.Database

	err := db.Update(func(txn *badger.Txn) error {
		for _, tx := range block.Transaction {
			if tx.IsCoinbase() == false {
				for _, in := range tx.Inputs {
					updateOuts := TXOutputs{}
					inID := append(utxoPrefix, in.ID...)
					fmt.Printf("update key is %s\n", inID)
					item, err := txn.Get(inID)
					ErrHandler(err)
					var v []byte
					err = item.Value(func(val []byte) error {
						v = val
						return nil
					})

					outs := DeserializeOutputs(v)
					for outIDx, out := range outs.Outputs {
						if outIDx != in.Out {
							updateOuts.Outputs = append(updateOuts.Outputs, out)
						}
					}
					if len(updateOuts.Outputs) == 0 {
						if err := txn.Delete(inID); err != nil {
							log.Panic(err)
						}
					} else {
						if err := txn.Set(inID, updateOuts.Serialize()); err != nil {
							log.Panic(err)
						}
					}
				}

				newOutputs := TXOutputs{}
				for _, out := range tx.Outputs {
					newOutputs.Outputs = append(newOutputs.Outputs, out)
				}

				txID := append(utxoPrefix, tx.ID...)
				if err := txn.Set(txID, newOutputs.Serialize()); err != nil {
					log.Panic(err)
				}
			}
		}

		return nil
	})
	ErrHandler(err)
}

func (u UTXOSet) CountTransactions() int {
	db := u.BlockChain.Database
	counter := 0

	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions

		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(utxoPrefix); it.ValidForPrefix(utxoPrefix); it.Next() {
			counter++
		}

		return nil
	})
	ErrHandler(err)

	return counter
}

// FindUnspentTransactions ...
func (u UTXOSet) FindUnspentTransactions(pubKeyHash []byte) []TxOutput {
	var UTXOs []TxOutput

	db := u.BlockChain.Database

	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(utxoPrefix); it.ValidForPrefix(utxoPrefix); it.Next() {
			item := it.Item()
			var v []byte
			err := item.Value(func(val []byte) error {
				v = val
				return nil
			})
			ErrHandler(err)
			outs := DeserializeOutputs(v)

			for _, out := range outs.Outputs {
				if out.IsLockedWithKey(pubKeyHash) {
					UTXOs = append(UTXOs, out)
				}
			}

		}

		return nil
	})
	ErrHandler(err)

	return UTXOs
}

func (u UTXOSet) FindSpendableOutputs(pubKeyHash []byte, amount int) (int, map[string][]int) {
	unspentOuts := make(map[string][]int)
	accumulated := 0

	db := u.BlockChain.Database

	err := db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(utxoPrefix); it.ValidForPrefix(utxoPrefix); it.Next() {
			item := it.Item()
			k := item.Key()
			var v []byte
			err := item.Value(func(val []byte) error {
				v = val
				return nil
			})
			ErrHandler(err)

			k = bytes.TrimPrefix(k, utxoPrefix)
			txID := hex.EncodeToString(k)
			outs := DeserializeOutputs(v)

			for outIDx, out := range outs.Outputs {
				if out.IsLockedWithKey(pubKeyHash) && accumulated < amount {
					accumulated += out.Value
					unspentOuts[txID] = append(unspentOuts[txID], outIDx)
				}
			}
		}

		return nil
	})
	ErrHandler(err)

	return accumulated, unspentOuts
}
