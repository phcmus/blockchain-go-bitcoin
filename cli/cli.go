package cli

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"

	"github.com/phnaharris/harris-blockchain-token/blockchain"
	"github.com/phnaharris/harris-blockchain-token/network"
	"github.com/phnaharris/harris-blockchain-token/wallet"
)

type CommandLine struct{}

type Command struct {
	fullCommand string
	detail      string
}

func (cli *CommandLine) printUsage() {
	commands := []Command{}
	commands = append(commands, Command{"getbalance -address ADDRESS", "get the balance for an address"})
	commands = append(commands, Command{"createblockchain -address ADDRESS", "creates a blockchain and sends genesis reward to address"})
	commands = append(commands, Command{"printchain", "Prints the blocks in the chain"})
	commands = append(commands, Command{"send -from FROM -to TO -amount AMOUNT -mine", "Send amount of coins. Then -mine flag is set, mine off of this node"})
	commands = append(commands, Command{"createwallet", "Creates a new Wallet"})
	commands = append(commands, Command{"listaddresses", "Lists the addresses in our wallet file"})
	commands = append(commands, Command{"reindexutxo", "Rebuilds the UTXO set"})
	commands = append(commands, Command{"startnode -miner ADDRESS", "Start a node with ID specified in NODE_ID env. var. -miner enables mining"})

	fmt.Println("Usage:")
	for _, command := range commands {
		fmt.Printf("%4s %-55s %s\n", "", command.fullCommand, command.detail)
	}
}

func (cli *CommandLine) validateArgs() {
	if len(os.Args) < 2 {
		cli.printUsage()
		runtime.Goexit()
	}
}

func (cli *CommandLine) StartNode(nodeID, minerAddress string) {
	fmt.Printf("Starting node %s.\n", nodeID)
	if len(minerAddress) > 0 {
		if wallet.ValidateAddress([]byte(minerAddress)) {
			fmt.Printf("Mining mode is on. Address to receive rewards: %s.\n", minerAddress)
		} else {
			Handle(errors.New("wrong miner address"))
		}
	}
	network.StartServer(nodeID, minerAddress)
}

func (cli *CommandLine) reindexUTXO(nodeID string) {
	chain := blockchain.ContinueBlockchain(nodeID)
	defer chain.Database.Close()
	UTXOSet := blockchain.UTXOSet{Chain: chain}
	UTXOSet.Reindex()

	count := UTXOSet.CountTransactions()
	fmt.Printf("Done! There are %d transactions in the UTXO set.\n", count)
}

func (cli *CommandLine) listAddresses(nodeID string) {
	wallets, _ := wallet.CreateWallets(nodeID)
	addresses := wallets.GetAllAddresses()

	for _, address := range addresses {
		fmt.Println(address)
	}
}

func (cli *CommandLine) createWallet(nodeID string) {
	wallets, _ := wallet.CreateWallets(nodeID)
	address := wallets.AddWallet()
	wallets.SaveFile(nodeID)
	fmt.Printf("New address is %s.\n", address)
}

func (cli *CommandLine) printChain(nodeID string) {
	chain := blockchain.ContinueBlockchain(nodeID)
	defer chain.Database.Close()

	iter := chain.Iterator()
	for {
		block := iter.Next()

		fmt.Printf("Hash: %x.\n", block.Hash)
		fmt.Printf("Prev hash: %x.\n", block.PrevHash)
		pow := blockchain.NewProof(block)
		fmt.Printf("PoW: %x.\n", strconv.FormatBool(pow.Validate()))

		for _, tx := range block.Transaction {
			fmt.Println(tx)
		}
		fmt.Println()

		if len(block.PrevHash) == 0 {
			break
		}
	}
}

func (cli *CommandLine) createBlockchain(address, nodeID string) {
	if !wallet.ValidateAddress([]byte(address)) {
		Handle(errors.New("address is not valid"))
	}
	chain := blockchain.InitBlockchain(address, nodeID)
	defer chain.Database.Close()

	UTXOSet := blockchain.UTXOSet{Chain: chain}
	UTXOSet.Reindex()

	fmt.Println("Create new blockchain finished!")
}

func (cli *CommandLine) getBalance(address, nodeID string) {
	if !wallet.ValidateAddress([]byte(address)) {
		Handle(errors.New("address is not valid"))
	}

	chain := blockchain.ContinueBlockchain(nodeID)
	UTXOSet := blockchain.UTXOSet{Chain: chain}
	defer chain.Database.Close()

	balance := 0
	pubKeyHash := wallet.Base58Decode([]byte(address))
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-wallet.ChecksumLength]
	UTXOs := UTXOSet.FindUnspentTransactions(pubKeyHash)

	for _, out := range UTXOs {
		balance += out.Value
	}

	fmt.Printf("Balance of %s: %d.\n", address, balance)
}

func (cli *CommandLine) send(from, to string, amount int, nodeID string, isMineNow bool) {
	fmt.Println("send 0")
	if !wallet.ValidateAddress([]byte(from)) || !wallet.ValidateAddress([]byte(to)) {
		Handle(errors.New("address is not valid"))
	}
	fmt.Println("send 1")

	chain := blockchain.ContinueBlockchain(nodeID)
	defer chain.Database.Close()
	fmt.Println("send 2")
	UTXOSet := blockchain.UTXOSet{Chain: chain}
	UTXOSet.Reindex()
	fmt.Println("send 3")
	wallets, err := wallet.CreateWallets(nodeID)
	if err != nil {
		fmt.Println("send", err)
		log.Panic(err)
	}
	wallet := wallets.GetWallet(from)

	fmt.Println("send 4")

	tx := blockchain.NewTransaction(&wallet, to, amount, &UTXOSet)
	if isMineNow {
		fmt.Println("send 5")
		cbTx := blockchain.CoinbaseTx(from, "")
		txs := []*blockchain.Transaction{cbTx, tx}
		block := chain.MineBlock(txs)
		UTXOSet.Update(block)
		fmt.Println("send 6")
	} else {
		network.SendTx(network.KnownNodes[0], tx)
		fmt.Println("Send tx.")
	}

	fmt.Println("Success!")
}

func (cli *CommandLine) Run() {
	cli.validateArgs()

	nodeID := os.Getenv("NODE_ID")
	if len(nodeID) == 0 {
		fmt.Printf("NODE_ID env is not available!")
		runtime.Goexit()
	}

	getBalanceCmd := flag.NewFlagSet("getbalance", flag.ExitOnError)
	createBlockchainCmd := flag.NewFlagSet("createblockchain", flag.ExitOnError)
	printChainCmd := flag.NewFlagSet("printchain", flag.ExitOnError)
	sendCmd := flag.NewFlagSet("send", flag.ExitOnError)
	createWalletCmd := flag.NewFlagSet("createwallet", flag.ExitOnError)
	listAddressesCmd := flag.NewFlagSet("listaddresses", flag.ExitOnError)
	reindexUTXOCmd := flag.NewFlagSet("reindexutxo", flag.ExitOnError)
	startNodeCmd := flag.NewFlagSet("startnode", flag.ExitOnError)

	getBalanceAddress := getBalanceCmd.String("address", "", "The address to get balance for.")
	createBlockchainAddress := createBlockchainCmd.String("address", "", "The address to send genesis block reward to.")
	sendFrom := sendCmd.String("from", "", "Source wallet address")
	sendTo := sendCmd.String("to", "", "Destination wallet address")
	sendAmount := sendCmd.Int("amount", 0, "Amount to send")
	sendMine := sendCmd.Bool("mine", false, "Mine immediately on the same node")
	startNodeMiner := startNodeCmd.String("miner", "", "Enable mining mode and send reward to ADDRESS")

	switch os.Args[1] {

	case "getbalance":
		err := getBalanceCmd.Parse(os.Args[2:])
		Handle(err)
	case "createblockchain":
		err := createBlockchainCmd.Parse(os.Args[2:])
		Handle(err)
	case "printchain":
		err := printChainCmd.Parse(os.Args[2:])
		Handle(err)
	case "send":
		err := sendCmd.Parse(os.Args[2:])
		Handle(err)
	case "createwallet":
		err := createWalletCmd.Parse(os.Args[2:])
		Handle(err)
	case "listaddresses":
		err := listAddressesCmd.Parse(os.Args[2:])
		Handle(err)
	case "reindexutxo":
		err := reindexUTXOCmd.Parse(os.Args[2:])
		Handle(err)
	case "startnode":
		err := startNodeCmd.Parse(os.Args[2:])
		Handle(err)
	default:
		cli.printUsage()
		runtime.Goexit()
	}

	if getBalanceCmd.Parsed() {
		if len(*getBalanceAddress) == 0 {
			getBalanceCmd.Usage()
			runtime.Goexit()
		}
		cli.getBalance(*getBalanceAddress, nodeID)
	}
	if createBlockchainCmd.Parsed() {
		if len(*createBlockchainAddress) == 0 {
			createBlockchainCmd.Usage()
			runtime.Goexit()
		}
		cli.createBlockchain(*createBlockchainAddress, nodeID)
	}
	if printChainCmd.Parsed() {
		cli.printChain(nodeID)
	}
	if sendCmd.Parsed() {
		if len(*sendFrom) == 0 || len(*sendTo) == 0 || *sendAmount < 0 {
			sendCmd.Usage()
			runtime.Goexit()
		}
		cli.send(*sendFrom, *sendTo, *sendAmount, nodeID, *sendMine)
	}
	if createWalletCmd.Parsed() {
		cli.createWallet(nodeID)
	}
	if listAddressesCmd.Parsed() {
		cli.listAddresses(nodeID)
	}
	if reindexUTXOCmd.Parsed() {
		cli.reindexUTXO(nodeID)
	}
	if startNodeCmd.Parsed() {
		nodeID := os.Getenv("NODE_ID")
		if len(nodeID) == 0 {
			startNodeCmd.Usage()
			runtime.Goexit()
		}
		cli.StartNode(nodeID, *startNodeMiner)
	}
}

func Handle(err error) {
	if err != nil {
		log.Panic(err)
	}
}
