package wallet

import (
	"bytes"
	"crypto/elliptic"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"os"
)

const walletFile = "./tmp/wallets_%s.data"

type Wallets struct {
	Wallets map[string]*Wallet
}

func CreateWallets(nodeId string) (*Wallets, error) {
	// load list of wallet from file
	ws := Wallets{}
	ws.Wallets = make(map[string]*Wallet)

	err := ws.LoadFile(nodeId)

	return &ws, err
}

func (ws *Wallets) AddWallet() string {
	// add wallet to list and return address
	wallet := MakeWallet()
	address := wallet.Address()
	ws.Wallets[string(address)] = wallet
	return string(address)
}

func (ws *Wallets) GetAllAddresses() []string {
	// return all addresses in wallets
	var addresses []string
	for w := range ws.Wallets {
		addresses = append(addresses, w)
	}
	return addresses
}

func (ws *Wallets) GetWallet(address string) Wallet {
	return *ws.Wallets[address]
}

func (ws *Wallets) LoadFile(nodeId string) error {
	// check if file is exist
	walletFile := fmt.Sprintf(walletFile, nodeId)
	if _, err := os.Stat(walletFile); os.IsNotExist(err) {
		return err
	}

	var wallets Wallets

	fileContent, err := ioutil.ReadFile(walletFile)
	if err != nil {
		return err
	}

	gob.Register(elliptic.P256())
	decoder := gob.NewDecoder(bytes.NewReader(fileContent))
	err = decoder.Decode(&wallets)
	if err != nil {
		return err
	}

	ws.Wallets = wallets.Wallets

	return nil
}

func (ws *Wallets) SaveFile(nodeId string) error {
	var content bytes.Buffer
	walletFile := fmt.Sprintf(walletFile, nodeId)

	gob.Register(elliptic.P256())

	encoder := gob.NewEncoder(&content)
	err := encoder.Encode(ws)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(walletFile, content.Bytes(), 0644)
	if err != nil {
		return err
	}

	return nil
}
