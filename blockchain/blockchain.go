package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/dgraph-io/badger"
)

const (
	dbPath      = "./tmp/blocks_%s"
	genesisData = "First Transaction from Genesis"
)

type Blockchain struct {
	LastHash []byte
	Database *badger.DB
}

func Handle(err error) {
	if err != nil {
		log.Panic(err)
	}
}

func DBExist(path string) bool {
	// check if db is exist
	if _, err := os.Stat(path + "/MANIFEST"); os.IsNotExist(err) {
		return false
	}
	return true
}

func ContinueBlockchain(nodeID string) *Blockchain {
	path := fmt.Sprintf(dbPath, nodeID)
	if !DBExist(path) {
		fmt.Println("No existing blockchain found! Please create one!")
		runtime.Goexit()
	}

	var lastHash []byte

	opts := badger.DefaultOptions(path)
	db, err := openDB(path, opts)
	Handle(err)

	err = db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		Handle(err)
		lastHash, err = item.ValueCopy(nil)
		return err
	})
	Handle(err)

	chain := &Blockchain{lastHash, db}
	return chain
}

func InitBlockchain(address, nodeId string) *Blockchain {
	// check if another blockchain exist
	path := fmt.Sprintf(dbPath, nodeId)
	if DBExist(path) {
		fmt.Println("Blockchain already exist!")
		runtime.Goexit()
	}

	// create a new database
	var lastHash []byte
	opts := badger.DefaultOptions(path)
	db, err := openDB(path, opts)
	Handle(err)

	err = db.Update(func(txn *badger.Txn) error {
		// mine a genesis block
		cbtx := CoinbaseTx(address, genesisData)
		// fmt.Printf("\n%x\n", cbtx.ID)
		genesis := Genesis(cbtx)
		fmt.Println("Genesis block created!")
		fmt.Printf("%x\n", genesis.Hash)

		err = txn.Set(genesis.Hash, genesis.Serialize())
		Handle(err)
		err = txn.Set([]byte("lh"), genesis.Hash)
		lastHash = genesis.Hash

		return err
	})

	Handle(err)

	return &Blockchain{lastHash, db}
}

func (chain *Blockchain) AddBlock(block *Block) {
	err := chain.Database.Update(func(txn *badger.Txn) error {
		// check if block is exist
		if _, err := txn.Get([]byte(block.Hash)); err == nil {
			// get block success => block exist
			return nil
		}

		// add block data to database
		blockData := block.Serialize()
		err := txn.Set(block.Hash, blockData)
		Handle(err)

		// get last block in database
		item, err := txn.Get([]byte("lh"))
		Handle(err)

		lastHash, err := item.ValueCopy(nil)
		Handle(err)

		item, err = txn.Get([]byte(lastHash))
		Handle(err)

		lastBlockData, err := item.ValueCopy(nil)
		Handle(err)

		lastBlock := DeserializeBlock(lastBlockData)

		// check if block have the max height
		if lastBlock.Height < block.Height {
			// change last hash
			err = txn.Set([]byte("lh"), block.Hash)
			Handle(err)
			chain.LastHash = block.Hash
		}

		return nil
	})
	Handle(err)
}

func (chain *Blockchain) GetBestHeight() int {
	var lastBlock *Block
	err := chain.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		Handle(err)

		lastHash, err := item.ValueCopy(nil)
		Handle(err)

		item, err = txn.Get(lastHash)
		Handle(err)

		lastBlockData, err := item.ValueCopy(nil)
		Handle(err)

		lastBlock = DeserializeBlock(lastBlockData)

		return nil
	})
	Handle(err)
	return lastBlock.Height
}

func (chain *Blockchain) GetBlock(blockHash []byte) (Block, error) {
	var block Block

	err := chain.Database.View(func(txn *badger.Txn) error {
		if item, err := txn.Get(blockHash); err != nil {
			return errors.New("block is not found")
		} else {
			blockData, _ := item.ValueCopy(nil)
			block = *DeserializeBlock(blockData)
		}

		return nil
	})

	return block, err
}

func (chain *Blockchain) GetBlockHashes() [][]byte {
	var blockHashes [][]byte

	iter := BlockchainIterator{chain.LastHash, chain.Database}
	blockHashes = append(blockHashes, iter.CurrentHash)
	for {
		prevBlock := iter.Next()
		blockHashes = append(blockHashes, prevBlock.Hash)
		if len(prevBlock.PrevHash) == 0 {
			break
		}
	}

	return blockHashes
}

func (chain *Blockchain) MineBlock(transactions []*Transaction) *Block {
	// verify transaction in for loop
	for _, tx := range transactions {
		if !chain.VerifyTransaction(tx) {
			log.Panic("Invalid transaction!")
		}
	}

	// get last block + last height in current blockchain
	var lastHash []byte = chain.LastHash
	var lastHeight int
	err := chain.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(lastHash)
		Handle(err)
		lastBlockData, err := item.ValueCopy(nil)
		lastBlock := DeserializeBlock(lastBlockData)
		lastHeight = lastBlock.Height

		return err
	})
	Handle(err)

	// create newBlock with next height and next last hash
	newBlock := CreateBlock(transactions, lastHash, lastHeight+1)

	// update newBlock to database
	err = chain.Database.Update(func(txn *badger.Txn) error {
		err := txn.Set(newBlock.Hash, newBlock.Serialize())
		Handle(err)
		err = txn.Set([]byte("lh"), newBlock.Hash)
		chain.LastHash = newBlock.Hash
		return err
	})
	Handle(err)

	return newBlock
}

func (chain *Blockchain) FindUTXO() map[string][]TxOutput {
	// find unspend transaction output for all transaction
	UTXO := make(map[string][]TxOutput)
	spentTXOs := make(map[string][]int)

	iter := chain.Iterator()

	for {
		currentBlock := iter.Next()

		for _, tx := range currentBlock.Transaction {
			txID := hex.EncodeToString(tx.ID)

		Start:
			for outIdx, out := range tx.Outputs {
				// travers through txOutput to skip the transaction faster
				if spentTXOs[txID] != nil {
					for _, spent := range spentTXOs[txID] {
						if outIdx == spent {
							continue Start
						}
					}
				}
				UTXO[txID] = append(UTXO[txID], out)
			}

			if !tx.IsCoinbase() {
				for _, inTx := range tx.Inputs {
					inTxID := hex.EncodeToString(inTx.ID)
					spentTXOs[inTxID] = append(spentTXOs[inTxID], inTx.Out)
				}
			}
		}

		if len(currentBlock.PrevHash) == 0 {
			break
		}
	}

	return UTXO
}

func (chain *Blockchain) FindTransaction(ID []byte) (Transaction, error) {
	iter := chain.Iterator()

	for {
		currentBlock := iter.Next()

		for _, tx := range currentBlock.Transaction {
			if bytes.Equal(ID, tx.ID) {
				return *tx, nil
			}
		}

		if len(currentBlock.PrevHash) == 0 {
			break
		}
	}

	return Transaction{}, errors.New("transaction does not exist")
}

func (chain *Blockchain) SignTransaction(tx *Transaction, privKey ecdsa.PrivateKey) {
	prevTxs := make(map[string]Transaction)

	for _, in := range tx.Inputs {
		prevTx, err := chain.FindTransaction(in.ID)
		Handle(err)
		prevTxs[hex.EncodeToString(prevTx.ID)] = prevTx
	}

	tx.Sign(&privKey, prevTxs)
}

func (chain *Blockchain) VerifyTransaction(tx *Transaction) bool {
	// check if transaction is coinbase => true
	if tx.IsCoinbase() {
		return true
	}

	prevTxs := make(map[string]Transaction)

	for _, in := range tx.Inputs {
		prevTx, err := chain.FindTransaction(in.ID)
		Handle(err)
		prevTxs[hex.EncodeToString(prevTx.ID)] = prevTx
	}

	return tx.Verify(prevTxs)
}

func openDB(dir string, opts badger.Options) (*badger.DB, error) {
	db, err := badger.Open(opts)
	return db, err
}
