package network

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"runtime"
	"syscall"

	"github.com/phnaharris/harris-blockchain-token/blockchain"
	"github.com/vrecan/death"
)

const (
	protocol      = "tcp"
	version       = 1
	commandLength = 12
)

var (
	nodeAddress     string
	minerAddress    string
	KnownNodes      = []string{"localhost:3000"} // online node -- KnownNode[0] is the genesis node
	blocksInTransit = [][]byte{}
	memoryPool      = make(map[string]blockchain.Transaction)
	maxMemPool      = 2
)

type Addr struct {
	AddrList []string
}

type Block struct {
	AddrFrom string
	Block    []byte
}

type GetBlocks struct {
	AddrFrom string
}

type GetData struct {
	AddrFrom string
	Type     string
	ID       []byte
}

type Inv struct {
	AddrFrom string
	Type     string
	Items    [][]byte
}

type Tx struct {
	AddrFrom    string
	Transaction []byte
}

type Version struct {
	Version    int
	BestHeight int
	AddrFrom   string
}

func CmdToBytes(cmd string) []byte {
	var bytes [commandLength]byte

	for i, c := range cmd {
		bytes[i] = byte(c)
	}

	return bytes[:]
}

func BytesToCmd(bytes []byte) string {
	var cmd []byte

	for _, b := range bytes {
		if b != 0x0 {
			cmd = append(cmd, b)
		}
	}

	return fmt.Sprintf("%s", cmd)
}

func ExtractCmd(request []byte) []byte {
	return request[:commandLength]
}

func RequestBlocks() {
	for _, node := range KnownNodes {
		SendGetBlocks(node)
	}
}

// send function : send request to address => address: target
// handle function : receive data from send function => addrFrom: source of send = target of handle

func SendAddr(address string) {
	addr := Addr{KnownNodes}
	payload := GobEncode(addr)
	request := append(CmdToBytes("addr"), payload...)
	SendData(address, request)
}

func SendBlock(address string, _block *blockchain.Block) {
	block := Block{nodeAddress, _block.Serialize()}
	payload := GobEncode(block)
	request := append(CmdToBytes("block"), payload...)
	SendData(address, request)
}

func SendData(addr string, data []byte) {
	conn, err := net.Dial(protocol, addr)
	if err != nil {
		fmt.Printf("%s is not available!\n", addr)

		// update online node
		var updatedNodes []string
		for _, node := range KnownNodes {
			if node != addr {
				updatedNodes = append(updatedNodes, node)
			}
		}
		KnownNodes = updatedNodes
		return
	}

	defer conn.Close()
	_, err = io.Copy(conn, bytes.NewReader(data))
	Handle(err)
}

func SendInv(address, kind string, items [][]byte) {
	inventory := Inv{nodeAddress, kind, items}
	payload := GobEncode(inventory)
	request := append(CmdToBytes("inv"), payload...)
	SendData(address, request)
}

func SendGetBlocks(address string) {
	getBlocks := GetBlocks{nodeAddress}
	payload := GobEncode(getBlocks)
	request := append(CmdToBytes("getblocks"), payload...)
	SendData(address, request)
}

func SendGetData(address, kind string, id []byte) {
	getData := GetData{nodeAddress, kind, id}
	payload := GobEncode(getData)
	request := append(CmdToBytes("getdata"), payload...)
	SendData(address, request)
}

func SendTx(address string, txn *blockchain.Transaction) {
	tx := Tx{nodeAddress, txn.Serialize()}
	payload := GobEncode(tx)
	request := append(CmdToBytes("tx"), payload...)
	SendData(address, request)
}

func SendVersion(address string, chain *blockchain.Blockchain) {
	ver := Version{version, chain.GetBestHeight(), nodeAddress}
	payload := GobEncode(ver)
	request := append(CmdToBytes("version"), payload...)
	SendData(address, request)
}

func HandleAddr(request []byte) {
	var buffer bytes.Buffer
	var payload Addr

	buffer.Write(request[commandLength:])
	decoder := gob.NewDecoder(&buffer)
	err := decoder.Decode(&payload)
	Handle(err)

	KnownNodes = append(KnownNodes, payload.AddrList...)
	fmt.Printf("There are %d known nodes.\n", len(KnownNodes))
	RequestBlocks()
}

func HandleBlock(request []byte, chain *blockchain.Blockchain) {
	var buffer bytes.Buffer
	var payload Block

	buffer.Write(request[commandLength:])
	decoder := gob.NewDecoder(&buffer)
	err := decoder.Decode(&payload)
	Handle(err)

	blockData := payload.Block
	block := blockchain.DeserializeBlock(blockData)

	fmt.Printf("Receive a block!\n")
	chain.AddBlock(block)
	fmt.Printf("Added block: %x.\n", block.Hash)

	// đoạn này chưa hiểu lắm nhưng thôi để làm sau vậy --> vẽ luồng chạy của code ra giấy sẽ hiểu
	if len(blocksInTransit) > 0 {
		blockHash := blocksInTransit[0]
		SendGetData(payload.AddrFrom, "block", blockHash)
		blocksInTransit = blocksInTransit[1:]
	} else {
		UTXOSet := blockchain.UTXOSet{Chain: chain}
		UTXOSet.Reindex()
	}

	// -----------------------------------------------
}

func HandleInv(request []byte, chain *blockchain.Blockchain) {
	var buffer bytes.Buffer
	var payload Inv

	buffer.Write(request[commandLength:])
	decoder := gob.NewDecoder(&buffer)
	err := decoder.Decode(&payload)
	Handle(err)

	fmt.Printf("Received inventory with %d %s.\n", len(payload.Items), payload.Type)

	if payload.Type == "block" {
		blocksInTransit = payload.Items
		blockHash := payload.Items[0]
		SendGetData(payload.AddrFrom, "block", blockHash)

		newInTransit := [][]byte{}
		for _, b := range blocksInTransit {
			if !bytes.Equal(b, blockHash) {
				newInTransit = append(newInTransit, b)
			}
		}
		blocksInTransit = newInTransit
	}

	if payload.Type == "tx" {
		txID := payload.Items[0]
		if len(memoryPool[hex.EncodeToString(txID)].ID) == 0 {
			SendGetData(payload.AddrFrom, "tx", txID)
		}
	}
}

func HandleGetBlocks(request []byte, chain *blockchain.Blockchain) {
	var buffer bytes.Buffer
	var payload GetBlocks

	buffer.Write(request[commandLength:])
	decoder := gob.NewDecoder(&buffer)
	err := decoder.Decode(&payload)
	Handle(err)

	blocks := chain.GetBlockHashes()
	SendInv(payload.AddrFrom, "block", blocks)
}

func HandleGetData(request []byte, chain *blockchain.Blockchain) {
	var buffer bytes.Buffer
	var payload GetData

	buffer.Write(request[commandLength:])
	decoder := gob.NewDecoder(&buffer)
	err := decoder.Decode(&payload)
	Handle(err)

	if payload.Type == "block" {
		block, err := chain.GetBlock(payload.ID)
		Handle(err)
		SendBlock(payload.AddrFrom, &block)
	}

	if payload.Type == "tx" {
		txID := hex.EncodeToString(payload.ID)
		tx := memoryPool[txID]
		SendTx(payload.AddrFrom, &tx)
	}
}

func HandleTx(request []byte, chain *blockchain.Blockchain) {
	var buffer bytes.Buffer
	var payload Tx

	buffer.Write(request[commandLength:])
	decoder := gob.NewDecoder(&buffer)
	err := decoder.Decode(&payload)
	Handle(err)

	txData := payload.Transaction
	tx := blockchain.DeserializeTransaction(txData)
	memoryPool[hex.EncodeToString(tx.ID)] = tx

	fmt.Printf("Memory Pool of %s have %d transactions.\n", nodeAddress, len(memoryPool))

	if nodeAddress == KnownNodes[0] { // genesis node
		for _, node := range KnownNodes {
			if node != nodeAddress && node != payload.AddrFrom { // not genesis node and miner node
				fmt.Println("HandleTx", node)
				SendInv(node, "tx", [][]byte{tx.ID})
			}
		}
	} else {
		fmt.Println("HandleTx", "Miner: ", minerAddress)
		fmt.Println("HandleTx", "MemPool: ", len(memoryPool))
		if len(memoryPool) >= maxMemPool && len(minerAddress) > 0 {
			fmt.Println("HandleTx", "Mineeee")
			MineTx(chain)
		}
	}
}

func Handle(err error) {
	if err != nil {
		log.Panic(err)
	}
}

func MineTx(chain *blockchain.Blockchain) {
	var txs []*blockchain.Transaction

	for id := range memoryPool {
		fmt.Printf("Tx: %s.\n", memoryPool[id].ID)
		tx := memoryPool[id]
		if chain.VerifyTransaction(&tx) {
			txs = append(txs, &tx)
		}
	}

	if len(txs) == 0 {
		fmt.Println("All transactions are invalid!")
		return
	}

	cbTx := blockchain.CoinbaseTx(minerAddress, "")
	txs = append(txs, cbTx)

	newBlock := chain.MineBlock(txs)
	UTXOSet := blockchain.UTXOSet{Chain: chain}
	UTXOSet.Reindex()

	fmt.Println("New block was mined!")

	for _, tx := range txs {
		delete(memoryPool, hex.EncodeToString(tx.ID))
	}

	for _, node := range KnownNodes {
		if node != nodeAddress {
			SendInv(node, "block", [][]byte{newBlock.Hash})
		}
	}

	if len(memoryPool) > 0 {
		MineTx(chain)
	}
}

func HandleVersion(request []byte, chain *blockchain.Blockchain) {
	var buffer bytes.Buffer
	var payload Version

	buffer.Write(request[commandLength:])
	decoder := gob.NewDecoder(&buffer)
	err := decoder.Decode(&payload)
	Handle(err)

	bestHeight := chain.GetBestHeight()
	otherHeight := payload.BestHeight

	if bestHeight > otherHeight {
		SendVersion(payload.AddrFrom, chain)
	} else if bestHeight < otherHeight {
		SendGetBlocks(payload.AddrFrom)
	}

	if !NodeIsKnown(payload.AddrFrom) {
		KnownNodes = append(KnownNodes, payload.AddrFrom)
	}
}

func HandleConnection(conn net.Conn, chain *blockchain.Blockchain) {
	request, err := ioutil.ReadAll(conn)
	defer conn.Close()

	Handle(err)

	command := BytesToCmd(request[:commandLength])
	fmt.Printf("Received %s command!\n", command)

	switch command {
	case "addr":
		HandleAddr(request)
	case "block":
		HandleBlock(request, chain)
	case "inv":
		HandleInv(request, chain)
	case "getblocks":
		HandleGetBlocks(request, chain)
	case "getdata":
		HandleGetData(request, chain)
	case "tx":
		HandleTx(request, chain)
	case "version":
		HandleVersion(request, chain)
	default:
		fmt.Println("Unknown command!")
	}
}

func StartServer(nodeID, _minerAddress string) {
	nodeAddress = fmt.Sprintf("localhost:%s", nodeID)
	minerAddress = _minerAddress
	ln, err := net.Listen(protocol, nodeAddress)
	Handle(err)
	defer ln.Close()

	chain := blockchain.ContinueBlockchain(nodeID)
	defer chain.Database.Close()
	go CloseDB(chain)

	if nodeAddress != KnownNodes[0] {
		SendVersion(KnownNodes[0], chain)
	}

	for {
		conn, err := ln.Accept()
		Handle(err)
		go HandleConnection(conn, chain)
	}
}

func GobEncode(data interface{}) []byte {
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)
	err := encoder.Encode(data)
	Handle(err)
	return buffer.Bytes()
}

// func GobDecode(data []byte) interface{} {
// 	var res interface{}
// 	decoder := gob.NewDecoder(bytes.NewReader(data))
// 	err := decoder.Decode(res)
// 	Handle(err)
// 	fmt.Println("GobDecode ", res)
// 	return res
// }

func NodeIsKnown(addr string) bool {
	for _, node := range KnownNodes {
		if node == addr {
			return true
		}
	}
	return false
}

func CloseDB(chain *blockchain.Blockchain) {
	d := death.NewDeath(syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	d.WaitForDeathWithFunc(func() {
		defer os.Exit(1)
		defer runtime.Goexit()
		chain.Database.Close()
	})
}
