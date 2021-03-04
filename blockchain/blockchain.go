package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dgraph-io/badger"
	"github.com/rs/zerolog/log"
)

const (
	dbPath      = "./tmp/blocks_%s"
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

// InitBlockChain 初始化 block chain
func InitBlockChain(address, nodeID string) *BlockChain {
	var lastHash []byte
	path := fmt.Sprintf(dbPath, nodeID)
	if DBExists(path) {
		fmt.Println("Blockchain already exists")
		runtime.Goexit()
	}

	opts := badger.DefaultOptions(path)
	opts.Dir = path
	opts.ValueDir = path
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
func ContinueBlockChain(nodeID string) *BlockChain {
	path := fmt.Sprintf(dbPath, nodeID)
	fmt.Println(path)
	if DBExists(path) == false {
		fmt.Println("No existing blockchain found, create one!")
		runtime.Goexit()
	}

	var lastHash []byte

	opt := badger.DefaultOptions(path)
	opt.Dir = path
	opt.ValueDir = path

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

// MineBlock 新增區塊到鏈中
func (chain *BlockChain) MineBlock(txs []*Transaction) *Block {
	var (
		lastHeight int
		lastHash   []byte
	)

	fmt.Println("start to mine block")

	for _, tx := range txs {
		if chain.VerifyTransaction(tx) != true {
			log.Panic().Msg("invalid transaction")
		}
	}

	err := chain.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(lastHashKey)
		if err != nil {
			log.Error().Msgf("fail to get badger db, %s", err.Error())
		}
		err = item.Value(func(val []byte) error {
			lastHash = val
			return nil
		})

		item, err = txn.Get(lastHash)
		ErrHandler(err)
		var lastBlockData []byte
		item.Value(func(val []byte) error {
			lastBlockData = val
			return nil
		})
		lastBlock := Deserialize(lastBlockData)
		lastHeight = lastBlock.Height

		return err
	})
	ErrHandler(err)

	fmt.Println("start to creat block")
	newBlock := CreateBlock(txs, lastHash, lastHeight+1)
	err = chain.Database.Update(func(txn *badger.Txn) error {
		err := txn.Set(newBlock.Hash, newBlock.Serialize())
		ErrHandler(err)
		err = txn.Set(lastHashKey, newBlock.Hash)
		chain.LastHash = newBlock.Hash
		return err
	})
	ErrHandler(err)

	fmt.Println("end to creat block")

	return newBlock
}
func (chain *BlockChain) AddBlock(block *Block) {
	err := chain.Database.Update(func(txn *badger.Txn) error {
		if _, err := txn.Get(block.Hash); err != nil {
			return nil
		}

		blockData := block.Serialize()
		err := txn.Set(block.Hash, blockData)
		ErrHandler(err)

		item, err := txn.Get(lastHashKey)
		ErrHandler(err)

		var (
			lastHash      []byte
			lastBlockData []byte
		)
		err = item.Value(func(val []byte) error {
			lastHash = val
			return nil
		})
		ErrHandler(err)

		item, err = txn.Get(lastHash)
		err = item.Value(func(val []byte) error {
			lastBlockData = val
			return nil
		})
		ErrHandler(err)

		lastBlock := Deserialize(lastBlockData)

		if block.Height > lastBlock.Height {
			err := txn.Set(lastHashKey, block.Hash)
			ErrHandler(err)

			chain.LastHash = block.Hash
		}

		return nil
	})
	ErrHandler(err)

	return
}

func (chain *BlockChain) GetBlock(blockHash []byte) (Block, error) {
	var block Block

	err := chain.Database.View(func(txn *badger.Txn) error {
		if item, err := txn.Get(blockHash); err != nil {
			return errors.New("Block is not found")
		} else {
			var blockData []byte
			item.Value(func(val []byte) error {
				blockData = val
				return nil
			})

			block = *Deserialize(blockData)
		}
		return nil
	})
	if err != nil {
		return block, err
	}

	return block, nil
}

func (chain *BlockChain) GetBlockhashes() [][]byte {
	var blocks [][]byte

	iter := chain.Iterator()

	for {
		block := iter.Next()
		blocks = append(blocks, block.Hash)
		if len(block.PrevHash) == 0 {
			break
		}
	}

	return blocks
}

func (chain *BlockChain) GetBestHeight() int {
	var lastBlock Block

	err := chain.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(lastHashKey)
		ErrHandler(err)
		var (
			lastHash      []byte
			lastBlockData []byte
		)

		item.Value(func(val []byte) error {
			lastHash = val
			return nil
		})

		item, err = txn.Get(lastHash)
		ErrHandler(err)
		item.Value(func(val []byte) error {
			lastBlockData = val
			return nil
		})

		lastBlock = *Deserialize(lastBlockData)

		return nil
	})
	ErrHandler(err)

	return lastBlock.Height
}

func DBExists(path string) bool {
	if _, err := os.Stat(path + "/MANIFEST"); os.IsNotExist(err) {
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

func (chain *BlockChain) VerifyTransaction(tx *Transaction) bool {
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

func retry(dir string, originalOpts badger.Options) (*badger.DB, error) {
	lockPath := filepath.Join(dir, "LOCK")
	if err := os.Remove(lockPath); err != nil {
		return nil, fmt.Errorf("remove LOCK: %s", err)
	}

	retryOpts := originalOpts
	retryOpts.Truncate = true

	db, err := badger.Open(retryOpts)
	return db, err
}

func openDB(dir string, opts badger.Options) (*badger.DB, error) {
	if db, err := badger.Open(opts); err != nil {
		if strings.Contains(err.Error(), "LOCK"); err == nil {
			if db, err := retry(dir, opts); err == nil {
				log.Debug().Msgf("database unlocked, value log truncated")
				return db, nil
			}
			log.Debug().Msgf("could not unlock database:", err)
		}
		return nil, err
	} else {
		return db, nil
	}
}
