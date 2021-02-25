package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/dgraph-io/badger"
	"github.com/rs/zerolog/log"
)

const (
	dbPath      = "./tmp/blocks"
	dbFile      = "./tmp/blocks/MANIFEST"
	genesisData = "First Transaction from Genesis"
)

var (
	lastHashKey = []byte("lh")
)

// BlockChain 區塊鏈
type BlockChain struct {
	LastHash []byte
	Database *badger.DB
}

// BlockChainIterator ...
type BlockChainIterator struct {
	CurrentHash []byte
	Database    *badger.DB
}

// InitBlockChain 初始化 block chain
func InitBlockChain(address string) *BlockChain {
	var lastHash []byte
	if DBExists() {
		fmt.Println("Blockchain already exists")
		runtime.Goexit()
	}

	opts := badger.DefaultOptions(dbPath)
	opts.Dir = dbPath
	opts.ValueDir = dbPath
	db, err := badger.Open(opts)
	if err != nil {
		log.Error().Msgf("fail to open badger db, %s", err.Error())
		return &BlockChain{}
	}

	err = db.Update(func(txn *badger.Txn) error {
		// if _, err := txn.Get(lastHashKey); err == badger.ErrKeyNotFound {
		// 	fmt.Println("No exists blockchain found")

		// 	genesis := Genesis()
		// 	fmt.Println("Genesis proved")

		// 	err = txn.Set(genesis.Hash, genesis.Serialize())

		// 	err = txn.Set(lastHashKey, genesis.Hash)

		// 	lastHash = genesis.Hash

		// 	return err
		// } else {
		// 	item, err := txn.Get(lastHashKey)
		// 	if err != nil {
		// 		log.Error().Msgf("fail to get badger db, %s", err.Error())
		// 	}
		// 	err = item.Value(func(val []byte) error {
		// 		lastHash = val
		// 		return nil
		// 	})
		// 	return err
		// }

		cbtx := CoinbaseTx(address, genesisData)
		genesis := Genesis(cbtx)
		fmt.Println("Genesis created")
		err := txn.Set(genesis.Hash, genesis.Serialize())
		ErrHandler(err)

		err = txn.Set(lastHashKey, genesis.Hash)
		lastHash = genesis.Hash

		return err
	})
	if err != nil {
		log.Error().Msgf("fail to update badger db, %s", err.Error())
	}

	return &BlockChain{lastHash, db}
}

// ContinueBlockChain ...
func ContinueBlockChain(address string) *BlockChain {
	if DBExists() == false {
		fmt.Println("No existing blockchain found, create one!")
		runtime.Goexit()
	}

	var lastHash []byte

	opt := badger.DefaultOptions(dbPath)
	opt.Dir = dbPath
	opt.ValueDir = dbPath

	db, err := badger.Open(opt)
	ErrHandler(err)

	err = db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get(lastHashKey)
		ErrHandler(err)

		err = item.Value(func(val []byte) error {
			lastHash = val
			return nil
		})

		return err
	})
	ErrHandler(err)

	chain := BlockChain{lastHash, db}
	return &chain
}

// AddBlock 新增區塊到鏈中
func (chain *BlockChain) AddBlock(txs []*Transaction) *Block {
	var lastHash []byte
	err := chain.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(lastHashKey)
		if err != nil {
			log.Error().Msgf("fail to get badger db, %s", err.Error())
		}
		err = item.Value(func(val []byte) error {
			lastHash = val
			return nil
		})
		return err
	})
	ErrHandler(err)

	newBlock := CreateBlock(txs, lastHash)
	err = chain.Database.Update(func(txn *badger.Txn) error {
		err := txn.Set(newBlock.Hash, newBlock.Serialize())
		ErrHandler(err)
		err = txn.Set(lastHashKey, newBlock.Hash)
		chain.LastHash = newBlock.Hash
		return err
	})
	ErrHandler(err)

	return newBlock
}

func (chain *BlockChain) Iterator() *BlockChainIterator {
	iter := &BlockChainIterator{
		chain.LastHash,
		chain.Database,
	}
	return iter
}

func (iter *BlockChainIterator) Next() *Block {
	var block *Block

	err := iter.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(iter.CurrentHash)
		ErrHandler(err)
		var encodeBlock []byte
		err = item.Value(func(val []byte) error {
			encodeBlock = val
			return nil
		})
		block = Deserialize(encodeBlock)

		return err
	})
	ErrHandler(err)

	iter.CurrentHash = block.PrevHash
	return block
}

func DBExists() bool {
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		return false
	}
	return true
}

// FindUTXO ...
func (chain *BlockChain) FindUTXO() map[string]TXOutputs {
	var (
		utxo      = make(map[string]TXOutputs)
		spentTXOs = make(map[string][]int)
	)

	iter := chain.Iterator()

	for {
		block := iter.Next()
		for _, tx := range block.Transaction {
			txID := hex.EncodeToString(tx.ID)

		Outputs:
			for outIDx, out := range tx.Outputs {
				if spentTXOs[txID] != nil {
					for _, spentOut := range spentTXOs[txID] {
						if spentOut == outIDx {
							continue Outputs
						}
					}
				}
				outs := utxo[txID]
				outs.Outputs = append(outs.Outputs, out)
				utxo[txID] = outs
			}
			if tx.IsCoinbase() == false {
				for _, in := range tx.Inputs {
					inTXID := hex.EncodeToString(in.ID)
					spentTXOs[inTXID] = append(spentTXOs[inTXID], in.Out)
				}
			}
		}
		if len(block.PrevHash) == 0 {
			break
		}

	}

	return utxo
}

func (chain *BlockChain) SignTransaction(tx *Transaction, privKey ecdsa.PrivateKey) {
	prevTXs := make(map[string]Transaction)

	for _, in := range tx.Inputs {
		prevTX, err := chain.FindTransaction(in.ID)
		ErrHandler(err)
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	tx.Sign(privKey, prevTXs)
}

func (chain *BlockChain) FindTransaction(id []byte) (Transaction, error) {
	iter := chain.Iterator()

	for {
		block := iter.Next()

		for _, tx := range block.Transaction {
			if bytes.Compare(tx.ID, id) == 0 {
				return *tx, nil
			}
		}

		if len(block.PrevHash) == 0 {
			break
		}
	}
	return Transaction{}, errors.New("Transaction is not exists")
}

func (chain *BlockChain) VerifyTransaction(tx *Transaction, privKey ecdsa.PrivateKey) bool {
	if tx.IsCoinbase() {
		return true
	}
	prevTXs := make(map[string]Transaction)

	for _, in := range tx.Inputs {
		prevTx, err := chain.FindTransaction(in.ID)
		ErrHandler(err)

		prevTXs[hex.EncodeToString(prevTx.ID)] = prevTx
	}

	return tx.Verify(prevTXs)
}
