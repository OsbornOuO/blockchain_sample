package blockchain

import (
	"encoding/hex"
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
		err := txn.Set([]byte("lh"), genesis.Hash)
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
		item, err := txn.Get([]byte("lh"))
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
func (chain *BlockChain) AddBlock(txs []*Transaction) {
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

// FindUnspentTransactions ...
func (chain *BlockChain) FindUnspentTransactions(address string) []Transaction {
	var unspentTxs []Transaction

	spentTX0s := make(map[string][]int)

	iter := chain.Iterator()

	for {
		block := iter.Next()

		for _, tx := range block.Transaction {
			txID := hex.EncodeToString(tx.ID)

		Outputs:
			for outIDx, out := range tx.Outputs {
				if spentTX0s[txID] != nil {
					for _, spentOut := range spentTX0s[txID] {
						if spentOut == outIDx {
							continue Outputs
						}
					}
				}

				if out.CanBeUnlocked(address) {
					unspentTxs = append(unspentTxs, *tx)
				}
			}

			if tx.IsCoinbase() == false {
				for _, in := range tx.Inputs {
					if in.CanUnlock(address) {
						inTxID := hex.EncodeToString(in.ID)
						spentTX0s[inTxID] = append(spentTX0s[inTxID], in.Out)
					}
				}
			}
		}

		if len(block.PrevHash) == 0 {
			break
		}
	}

	return unspentTxs
}

// FindUTX0 ...
func (chain *BlockChain) FindUTX0(address string) []TxOutput {
	var utx0s []TxOutput

	unspentTransactions := chain.FindUnspentTransactions(address)
	for _, tx := range unspentTransactions {
		for _, out := range tx.Outputs {
			if out.CanBeUnlocked(address) {
				utx0s = append(utx0s, out)
			}
		}
	}

	return utx0s
}

func (chain *BlockChain) FindSpendableOutputs(address string, amount int) (int, map[string][]int) {
	unspentOuts := make(map[string][]int)
	unspentTxs := chain.FindUnspentTransactions(address)

	accumulated := 0

Work:
	for _, tx := range unspentTxs {
		txID := hex.EncodeToString(tx.ID)

		for outIDx, out := range tx.Outputs {
			if out.CanBeUnlocked(address) && accumulated < amount {
				accumulated += out.Value
				unspentOuts[txID] = append(unspentOuts[txID], outIDx)

				if accumulated >= amount {
					break Work
				}
			}
		}
	}
	return accumulated, unspentOuts
}
