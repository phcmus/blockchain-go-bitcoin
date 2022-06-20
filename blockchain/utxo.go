package blockchain

import (
	"bytes"
	"encoding/hex"

	"github.com/dgraph-io/badger"
)

type UTXOSet struct {
	Chain *Blockchain
}

var (
	utxoPrefix = []byte("utxo-")
	// prefixLength = len(utxoPrefix)
)

func (u UTXOSet) FindSpendableOutputs(pubKeyHash []byte, amount int) (int, map[string][]int) {
	unspentOuts := make(map[string][]int) // map[txID] list index
	accumulated := 0
	db := u.Chain.Database

	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(utxoPrefix); it.ValidForPrefix(utxoPrefix); it.Next() {
			key := it.Item().KeyCopy(nil)
			value, err := it.Item().ValueCopy(nil)
			Handle(err)

			txID := hex.EncodeToString(bytes.TrimPrefix(key, utxoPrefix))
			outs := DeserializeTxOutputs(value)

			for outIdx, out := range outs.Outputs {
				if out.IsLockedWithKey(pubKeyHash) && accumulated < amount {
					accumulated += out.Value
					unspentOuts[txID] = append(unspentOuts[txID], outIdx)
				}
				if accumulated >= amount {
					// break faster
					return nil
				}
			}
		}
		return nil
	})
	Handle(err)

	return accumulated, unspentOuts
}

func (u UTXOSet) FindUnspentTransactions(pubKeyHash []byte) []TxOutput {
	var UTXOs []TxOutput

	err := u.Chain.Database.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(utxoPrefix); it.ValidForPrefix(utxoPrefix); it.Next() {
			value, err := it.Item().ValueCopy(nil)
			Handle(err)

			outs := DeserializeTxOutputs(value)
			for _, out := range outs.Outputs {
				if out.IsLockedWithKey(pubKeyHash) {
					UTXOs = append(UTXOs, out)
				}
			}
		}
		return nil
	})
	Handle(err)

	return UTXOs
}

func (u UTXOSet) CountTransactions() int {
	count := 0
	err := u.Chain.Database.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(utxoPrefix); it.ValidForPrefix(utxoPrefix); it.Next() {
			count++
		}
		return nil
	})
	Handle(err)
	return count
}

func (u UTXOSet) Reindex() {
	u.DeleteByPrefix(utxoPrefix)
	utxo := u.Chain.FindUTXO()

	err := u.Chain.Database.Update(func(txn *badger.Txn) error {
		for txID, outs := range utxo {
			key, err := hex.DecodeString(txID)
			Handle(err)
			key = append(utxoPrefix, key...)

			err = txn.Set(key, TxOutputs{outs}.Serialize())
			Handle(err)
		}

		return nil
	})

	Handle(err)
}

func (u UTXOSet) Update(block *Block) {
	err := u.Chain.Database.Update(func(txn *badger.Txn) error {
		for _, tx := range block.Transaction {
			if !tx.IsCoinbase() {
				for _, in := range tx.Inputs {
					inID := append(utxoPrefix, in.ID...)
					item, err := txn.Get(inID)
					Handle(err)
					value, err := item.ValueCopy(nil)
					Handle(err)
					outs := DeserializeTxOutputs(value)

					updatedOutputs := TxOutputs{}
					for outIdx, out := range outs.Outputs {
						if outIdx != in.Out {
							updatedOutputs.Outputs = append(updatedOutputs.Outputs, out)
						}
					}

					if len(updatedOutputs.Outputs) == 0 {
						err = txn.Delete(in.ID)
						Handle(err)
					} else {
						err = txn.Set(inID, updatedOutputs.Serialize())
						Handle(err)
					}
				}
			}

			newOutputs := TxOutputs{}
			newOutputs.Outputs = append(newOutputs.Outputs, tx.Outputs...)

			txID := append(utxoPrefix, tx.ID...)
			err := txn.Set(txID, newOutputs.Serialize())
			Handle(err)
		}
		return nil
	})
	Handle(err)
}

func (u UTXOSet) DeleteByPrefix(prefix []byte) {
	deleteByKeys := func(keys [][]byte) error {
		if err := u.Chain.Database.Update(func(txn *badger.Txn) error {
			for _, key := range keys {
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

	collectMaxsize := 100000

	err := u.Chain.Database.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		keyForDeletes := make([][]byte, 0, collectMaxsize)
		keyCollected := 0

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().KeyCopy(nil)
			keyForDeletes = append(keyForDeletes, key)
			keyCollected++
			if keyCollected == collectMaxsize {
				err := deleteByKeys(keyForDeletes)
				Handle(err)

				keyForDeletes = make([][]byte, 0, collectMaxsize)
				keyCollected = 0
			}
		}

		if keyCollected > 0 {
			err := deleteByKeys(keyForDeletes)
			Handle(err)
		}
		return nil
	})
	Handle(err)
}
