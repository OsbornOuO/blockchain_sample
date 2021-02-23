package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"

	"github.com/rs/zerolog/log"
)

// Take the data from the block

// create a counter (nonce) which starts at 0

// create a hash of the data plus the counter

// check the hash to see if it meets a set of requirements

// Requirements:
// The first few bytes must contain 0s

// Difficulty 數字越大 找出相對應的 隨機數越久, 要依照鏈上的礦工數目, 以及區塊產生的速度 作為調整項目
const Difficulty = 18

// ProofOfWork 工作證明
type ProofOfWork struct {
	Block  *Block
	Target *big.Int
}

// Run 開始挖礦
func (pow *ProofOfWork) Run() (int, []byte) {
	var (
		intHash big.Int
		hash    [32]byte
	)

	nonce := 0

	for nonce < math.MaxInt64 {
		data := pow.InitData(nonce)
		hash = sha256.Sum256(data)

		fmt.Printf("\r%x", hash)
		intHash.SetBytes(hash[:])

		if intHash.Cmp(pow.Target) == -1 {
			break
		} else {
			nonce++
		}
	}
	fmt.Println()

	return nonce, hash[:]
}

// Validate ...
func (pow *ProofOfWork) Validate() bool {
	var intHash big.Int

	data := pow.InitData(pow.Block.Nonce)
	hash := sha256.Sum256(data)
	intHash.SetBytes(hash[:])

	return intHash.Cmp(pow.Target) == -1
}

// NewProof 初始化
func NewProof(b *Block) *ProofOfWork {
	target := big.NewInt(1)
	target.Lsh(target, uint(256-Difficulty))

	pow := &ProofOfWork{b, target}

	return pow
}

// InitData 初始化 pow 的 block
func (pow *ProofOfWork) InitData(nonce int) []byte {
	data := bytes.Join(
		[][]byte{
			pow.Block.PrevHash,
			pow.Block.HashTransactions(),
			ToHex(int64(nonce)),
			ToHex(int64(Difficulty)),
		},
		[]byte{},
	)
	return data
}

// ToHex ...
func ToHex(num int64) []byte {
	buff := new(bytes.Buffer)
	err := binary.Write(buff, binary.BigEndian, num)
	if err != nil {
		log.Error().Msgf("error to binary write: %s", err.Error())
		return []byte{}
	}

	return buff.Bytes()
}
