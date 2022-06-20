package blockchain

import (
	"github.com/dgraph-io/badger"
)

type BlockchainIterator struct {
	CurrentHash []byte
	Database    *badger.DB
}

func (chain *Blockchain) Iterator() *BlockchainIterator {
	i := BlockchainIterator{chain.LastHash, chain.Database}
	return &i
}

func (iter *BlockchainIterator) Next() *Block {
	var block *Block

	err := iter.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(iter.CurrentHash)
		Handle(err)

		blockData, err := item.ValueCopy(nil)
		block = DeserializeBlock(blockData)

		return err
	})
	Handle(err)
	iter.CurrentHash = block.PrevHash

	return block
}
