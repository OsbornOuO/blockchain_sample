package blockchain

import "github.com/dgraph-io/badger"

type BlockChainIterator struct {
	CurrentHash []byte
	Database    *badger.DB
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
