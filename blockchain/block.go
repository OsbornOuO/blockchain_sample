package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"runtime/debug"

	"github.com/rs/zerolog/log"
)

// Block 區塊
type Block struct {
	Hash        []byte
	Transaction []*Transaction
	PrevHash    []byte // 上一個 hash
	Nonce       int    // 隨機數
}

// Genesis 創建初始區塊
func Genesis(coinbase *Transaction) *Block {
	return CreateBlock([]*Transaction{coinbase}, []byte{})
}

// HashTransactions 將 transaction 與 prev hash 做一次 hash
func (b *Block) HashTransactions() []byte {
	var txHashes [][]byte
	var txHash [32]byte

	for _, tx := range b.Transaction {
		txHashes = append(txHashes, tx.ID)
	}

	txHash = sha256.Sum256(bytes.Join(txHashes, []byte{}))
	return txHash[:]
}

// CreateBlock 創建區塊
func CreateBlock(txs []*Transaction, prevHash []byte) *Block {
	block := &Block{
		Hash:        []byte{},
		Transaction: txs,
		PrevHash:    prevHash,
		Nonce:       0,
	}
	pow := NewProof(block)
	nonce, hash := pow.Run()
	block.Hash = hash
	block.Nonce = nonce

	return block
}

// Serialize ...
func (b *Block) Serialize() []byte {
	var res bytes.Buffer

	encoder := gob.NewEncoder(&res)
	err := encoder.Encode(b)
	if err != nil {
		log.Error().Msgf("Fail to encode from block")
		return []byte{}
	}

	return res.Bytes()
}

// Deserialize ...
func Deserialize(data []byte) *Block {
	var block Block

	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&block)
	if err != nil {
		log.Error().Msgf("Fail to decode to block")
	}

	return &block
}

// ErrHandler ...
func ErrHandler(err error) {
	if err != nil {
		debug.PrintStack()
		log.Panic().Stack().Err(err).Msgf("err %s", err.Error())
	}
}
