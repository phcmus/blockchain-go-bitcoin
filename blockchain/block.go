package blockchain

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"time"
)

type Block struct {
	Timestamp   int64
	Hash        []byte
	Transaction []*Transaction
	PrevHash    []byte
	Nonce       int
	Height      int
}

func (b *Block) HashTransaction() []byte {
	var txHashes [][]byte

	for _, tx := range b.Transaction {
		txHashes = append(txHashes, tx.Serialize())
	}
	tree := NewMerkleTree(txHashes)

	return tree.RootNode.Data
}

func CreateBlock(txs []*Transaction, prevHash []byte, height int) *Block {
	block := &Block{time.Now().Unix(), []byte{}, txs, prevHash, 0, height}
	pow := NewProof(block)
	nonce, hash := pow.Run()
	fmt.Printf("Nonce: %d.\n", nonce)
	block.Hash = hash
	block.Nonce = nonce
	fmt.Printf("POW: %x.\n", hash)
	fmt.Printf("Block created! Block hash: %x.\n", block.Hash)

	return block
}

func Genesis(coinbase *Transaction) *Block {
	return CreateBlock([]*Transaction{coinbase}, []byte{}, 0)
}

func (b *Block) Serialize() []byte {
	var res bytes.Buffer

	encoder := gob.NewEncoder(&res)
	err := encoder.Encode(b)

	Handle(err)

	return res.Bytes()
}

func DeserializeBlock(data []byte) *Block {
	var block Block

	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&block)

	Handle(err)

	return &block
}
