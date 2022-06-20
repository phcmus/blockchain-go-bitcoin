package blockchain

import (
	"bytes"
	"encoding/gob"

	"github.com/phnaharris/harris-blockchain-token/wallet"
)

type TxInput struct {
	ID        []byte // ID of transaction that have output to reference
	Out       int    // index of transaction output
	Signature []byte // Sender signature - must be equal to transaction output PubKeyHash
	PubKey    []byte // == out.PubKeyHash
}

type TxOutput struct {
	Value      int    // value of transaction output
	PubKeyHash []byte // PubKeyHash of people who can unlock this output == address
}

type TxOutputs struct {
	Outputs []TxOutput
}

func (out *TxOutput) Lock(address []byte) {
	// lock a TxOutput to an address
	versionHashed := wallet.Base58Decode(address)
	pubKeyHash := versionHashed[1 : len(versionHashed)-wallet.ChecksumLength]
	out.PubKeyHash = pubKeyHash
}

func (out *TxOutput) IsLockedWithKey(pubKeyHash []byte) bool {
	return bytes.Equal(out.PubKeyHash, pubKeyHash)
}

func NewTxOutput(value int, address string) *TxOutput {
	out := &TxOutput{value, nil}
	out.Lock([]byte(address))
	return out
}

// func (out TxOutput) Serialize() []byte {
// 	var buffer bytes.Buffer
// 	encode := gob.NewEncoder(&buffer)
// 	err := encode.Encode(out)
// 	Handle(err)
// 	return buffer.Bytes()
// }

func (outs TxOutputs) Serialize() []byte {
	var buffer bytes.Buffer
	encode := gob.NewEncoder(&buffer)
	err := encode.Encode(outs)
	Handle(err)
	return buffer.Bytes()
}

// func DeserializeTxOutput(data []byte) TxOutput {
// 	var out TxOutput
// 	decode := gob.NewDecoder(bytes.NewReader(data))
// 	err := decode.Decode(&out)
// 	Handle(err)
// 	return out
// }

func DeserializeTxOutputs(data []byte) TxOutputs {
	var outs TxOutputs
	decode := gob.NewDecoder(bytes.NewReader(data))
	err := decode.Decode(&outs)
	Handle(err)
	return outs
}
