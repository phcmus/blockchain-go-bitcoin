package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/phnaharris/harris-blockchain-token/wallet"
)

const (
	Reward = 20
)

type Transaction struct {
	ID      []byte
	Inputs  []TxInput
	Outputs []TxOutput
}

func (tx *Transaction) Hash() []byte {
	var hash [32]byte

	txCopy := *tx
	txCopy.ID = []byte{}

	hash = sha256.Sum256(txCopy.Serialize())

	return hash[:]
}

func (tx *Transaction) Serialize() []byte {
	var res bytes.Buffer

	encoder := gob.NewEncoder(&res)
	err := encoder.Encode(tx)

	Handle(err)

	return res.Bytes()
}

func DeserializeTransaction(data []byte) Transaction {
	var tx Transaction

	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&tx)

	Handle(err)

	return tx
}

func CoinbaseTx(to, data string) *Transaction {
	var tx *Transaction

	if len(data) == 0 {
		randData := make([]byte, 24)
		_, err := rand.Read(randData)
		Handle(err)
		data = fmt.Sprintf("%x", randData)
	}

	txIn := TxInput{[]byte{}, -1, nil, []byte(data)}
	txOut := NewTxOutput(Reward, to)
	tx = &Transaction{nil, []TxInput{txIn}, []TxOutput{*txOut}}
	tx.ID = tx.Hash()

	return tx
}

func NewTransaction(w *wallet.Wallet, to string, amount int, UTXO *UTXOSet) *Transaction {
	var inputs []TxInput
	var outputs []TxOutput

	pubKeyHash := wallet.PublicKeyHash(w.PublicKey)
	accumulated, validOutputs := UTXO.FindSpendableOutputs(pubKeyHash, amount)

	if accumulated < amount {
		log.Panic("Error: Not enough funds! Please deposit!")
	}

	for txid, outs := range validOutputs {
		txID, err := hex.DecodeString(txid)
		Handle(err)
		for _, out := range outs {
			input := TxInput{txID, out, nil, w.PublicKey}
			inputs = append(inputs, input)
		}
	}

	from := fmt.Sprintf("%s", w.Address())

	outputs = append(outputs, *NewTxOutput(amount, to))
	if accumulated > amount {
		outputs = append(outputs, *NewTxOutput(accumulated-amount, from))
	}

	tx := Transaction{nil, inputs, outputs}
	tx.ID = tx.Hash()
	UTXO.Chain.SignTransaction(&tx, w.PrivateKey)
	return &tx
}

func (tx *Transaction) IsCoinbase() bool {
	// 1 inputTx
	// input.Out == -1
	// input[0].ID == 0

	return len(tx.Inputs) == 1 && len(tx.Inputs[0].ID) == 0 && tx.Inputs[0].Out == -1
}

func (tx *Transaction) Sign(privKey *ecdsa.PrivateKey, prevTxs map[string]Transaction) {
	if tx.IsCoinbase() {
		return
	}

	// validate prevTxs
	for _, in := range tx.Inputs {
		if len(prevTxs[hex.EncodeToString(in.ID)].ID) == 0 {
			Handle(errors.New("previous transaction is not available"))
		}
	}

	txCopied := tx.DeepCopy()

	for inIdx, in := range txCopied.Inputs {
		prevTx := prevTxs[hex.EncodeToString(in.ID)]
		txCopied.Inputs[inIdx].Signature = nil
		txCopied.Inputs[inIdx].PubKey = prevTx.Outputs[in.Out].PubKeyHash

		dataToSign := fmt.Sprintf("%x\n", txCopied)

		r, s, err := ecdsa.Sign(rand.Reader, privKey, []byte(dataToSign))
		Handle(err)

		signature := append(r.Bytes(), s.Bytes()...)
		tx.Inputs[inIdx].Signature = signature
		txCopied.Inputs[inIdx].PubKey = nil
	}
}

func (tx *Transaction) Verify(prevTxs map[string]Transaction) bool {
	if tx.IsCoinbase() {
		return true
	}

	for _, in := range tx.Inputs {
		if len(prevTxs[hex.EncodeToString(in.ID)].ID) == 0 {
			Handle(errors.New("previous transaction is not available"))
		}
	}

	// xem như đã có => sẽ quay lại sau
	txCopy := tx.DeepCopy()
	curve := elliptic.P256()

	for inId, in := range tx.Inputs {
		prevTx := prevTxs[hex.EncodeToString(in.ID)]
		txCopy.Inputs[inId].Signature = nil
		txCopy.Inputs[inId].PubKey = prevTx.Outputs[in.Out].PubKeyHash

		r := big.Int{}
		s := big.Int{}

		sigLen := len(in.Signature)
		r.SetBytes(in.Signature[:(sigLen / 2)])
		s.SetBytes(in.Signature[(sigLen / 2):])

		x := big.Int{}
		y := big.Int{}
		keyLen := len(in.PubKey)
		x.SetBytes(in.PubKey[:(keyLen / 2)])
		y.SetBytes(in.PubKey[(keyLen / 2):])

		dataToVerify := fmt.Sprintf("%x\n", txCopy)

		rawPubKey := ecdsa.PublicKey{Curve: curve, X: &x, Y: &y}
		if !ecdsa.Verify(&rawPubKey, []byte(dataToVerify), &r, &s) {
			return false
		}
		txCopy.Inputs[inId].PubKey = nil
	}

	return true
}

func (tx *Transaction) DeepCopy() *Transaction {
	var txInput []TxInput
	var txOutput []TxOutput

	txID := tx.ID

	for _, in := range tx.Inputs {
		txInput = append(txInput, TxInput{in.ID, in.Out, in.Signature, in.PubKey})
	}

	for _, out := range tx.Outputs {
		txOutput = append(txOutput, TxOutput{out.Value, out.PubKeyHash})
	}

	return &Transaction{txID, txInput, txOutput}
}

func (tx *Transaction) String() string {
	var lines []string

	lines = append(lines, fmt.Sprintf("--- Transaction %x:", tx.ID))
	for i, input := range tx.Inputs {
		lines = append(lines, fmt.Sprintf("     Input %d:", i))
		lines = append(lines, fmt.Sprintf("       TXID:     %x", input.ID))
		lines = append(lines, fmt.Sprintf("       Out:       %d", input.Out))
		lines = append(lines, fmt.Sprintf("       Signature: %x", input.Signature))
		lines = append(lines, fmt.Sprintf("       PubKey:    %x", input.PubKey))
	}

	for i, output := range tx.Outputs {
		lines = append(lines, fmt.Sprintf("     Output %d:", i))
		lines = append(lines, fmt.Sprintf("       Value:  %d", output.Value))
		lines = append(lines, fmt.Sprintf("       Script: %x", output.PubKeyHash))
	}

	return strings.Join(lines, "\n")
}
