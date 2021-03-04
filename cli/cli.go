package cli

import (
	"blockchain/blockchain"
	"blockchain/network"
	"blockchain/wallet"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
)

type CommandLine struct {
}

func (cli *CommandLine) printUsage() {
	fmt.Println("Usage: ")
	fmt.Println(" getBalance -add ress ADDRESS - get the balance for ")
	fmt.Println(" createBlockchain -address ADDRESS creates a blockchain")
	fmt.Println(" printchian - Prints the blocks in the chain")
	fmt.Println(" send -from FROM -to TO -amount AMOUNT -mine - Send amount")
	fmt.Println(" createWallet  - Creates a new Wallet")
	fmt.Println(" listAddresses - List the address in our wallet file")
	fmt.Println(" ReIndexUTXO - Rebuild the UTXO set")
	fmt.Println(" startNode - miner ADDRESS - Start a node with ID specified in NODE_ID env.")
}

func (cli *CommandLine) validateArgs() {
	if len(os.Args) < 2 {
		cli.printUsage()
		runtime.Goexit()
	}
}
func (cli *CommandLine) ReIndexUTXO() {
	chain := blockchain.ContinueBlockChain("")
	defer chain.Database.Close()

	UTXOSet := blockchain.UTXOSet{BlockChain: chain}
	UTXOSet.ReIndex()

	count := UTXOSet.CountTransactions()
	fmt.Printf("Done! There are %d Transaction in the UTXO set.\n", count)
}

func (cli *CommandLine) printChain(nodeID string) {
	chain := blockchain.ContinueBlockChain(nodeID)
	defer chain.Database.Close()
	iter := chain.Iterator()

	for {
		b := iter.Next()

		fmt.Printf("previous Hash: %x\n", b.PrevHash)
		fmt.Printf("Hash: %x\n", b.Hash)

		pow := blockchain.NewProof(b)
		fmt.Printf("Pow: %s\n", strconv.FormatBool(pow.Validate()))
		fmt.Println()

		for _, tx := range b.Transaction {
			fmt.Println(tx)
		}

		if len(b.PrevHash) == 0 {
			break
		}
	}
}

func (cli CommandLine) StartNode(nodeID, minerAddress string) {
	fmt.Println("Start Node %s\n", nodeID)

	if len(minerAddress) > 0 {
		if wallet.ValidateAddress(minerAddress) {
			fmt.Println("mining is on. Address to receive rewards: ", minerAddress)
		} else {
			log.Panic("Wrong miner address !")
		}
	}
	network.StartServer(nodeID, minerAddress)
}

func (cli *CommandLine) createBlockChain(nodeID string, address string) {
	chain := blockchain.InitBlockChain(address, nodeID)
	defer chain.Database.Close()

	UTXOSet := blockchain.UTXOSet{BlockChain: chain}
	UTXOSet.ReIndex()

	fmt.Println("Finished!")
}

func (cli *CommandLine) createWallet(nodeID string) {
	ws, err := wallet.CreateWallets(nodeID)
	if err != nil {
		log.Print(err)
	}

	address := ws.AddWallet()
	ws.SaveFile(nodeID)

	fmt.Println("address is " + address)

	fmt.Println("Finished!")
}

func (cli *CommandLine) listAddresses(nodeID string) {
	wallets, err := wallet.CreateWallets(nodeID)
	if err != nil {
		log.Panic(err)
	}

	fmt.Println(wallets.Wallets)
	addresses := wallets.GetAllAddresses()

	for _, address := range addresses {
		fmt.Println(address)
	}
	fmt.Println("Finished!")
}
func (cli *CommandLine) getBalance(address string, nodeID string) {
	if !wallet.ValidateAddress(address) {
		log.Panic("Address is not Valid")
	}

	chain := blockchain.ContinueBlockChain(nodeID)
	defer chain.Database.Close()

	UTXOSet := blockchain.UTXOSet{chain}

	balance := 0
	pubKeyHash := wallet.Base58Decode([]byte(address))
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-4]
	utxos := UTXOSet.FindUnspentTransactions(pubKeyHash)

	for _, out := range utxos {
		balance += out.Value
	}

	fmt.Printf("Balance of %s: %d\n", address, balance)
}

func (cli *CommandLine) send(from, to string, amount int, nodeID string, mineNow bool) {
	if !wallet.ValidateAddress(from) {
		log.Panic("from addres is not valid ")
	}

	if !wallet.ValidateAddress(to) {
		log.Panic("to addres is not valid ")
	}

	chain := blockchain.ContinueBlockChain(nodeID)
	defer chain.Database.Close()
	UTXOSet := blockchain.UTXOSet{BlockChain: chain}

	fmt.Println("Continue block chain")

	wallets, err := wallet.CreateWallets(nodeID)
	if err != nil {
		log.Panic(err)
	}
	fmt.Println("get wallets")

	fmt.Println(wallets.Wallets)
	fmt.Printf("from wallet is %s\n", from)

	wallet := wallets.GetWallet(from)

	fmt.Println("get wallet")

	tx := blockchain.NewTransaction(&wallet, to, amount, &UTXOSet)
	if mineNow {
		cbTx := blockchain.CoinbaseTx(from, "")
		txs := []*blockchain.Transaction{cbTx, tx}
		block := chain.MineBlock(txs)
		UTXOSet.Update(block)
	} else {
		network.SendTx(network.KnownNodes[0], tx)
		fmt.Println("send tx")
	}

	fmt.Println("Success!")
}

// Run ...
func (cli *CommandLine) Run() {
	cli.validateArgs()

	nodeID := os.Getenv("NODE_ID")
	if nodeID == "" {
		fmt.Printf("NODE_ID env is not set!")
		runtime.Goexit()
	}

	gbCmd := flag.NewFlagSet("getBalance", flag.ExitOnError)
	createBlockCmd := flag.NewFlagSet("createBlockchain", flag.ExitOnError)
	sendCmd := flag.NewFlagSet("send", flag.ExitOnError)
	printChainCmd := flag.NewFlagSet("printChain", flag.ExitOnError)
	createWalletCmd := flag.NewFlagSet("createWallet", flag.ExitOnError)
	listAddressesCmd := flag.NewFlagSet("listAddresses", flag.ExitOnError)
	ReIndexUTXOCmd := flag.NewFlagSet("ReIndexUTXO", flag.ExitOnError)
	startNodeCmd := flag.NewFlagSet("startNode", flag.ExitOnError)

	getBalanceAddress := gbCmd.String("address", "", "get address balance")
	createBlockchainAddress := createBlockCmd.String("address", "", "create block with address")
	sendFrom := sendCmd.String("from", "", "source wallet address")
	sendTo := sendCmd.String("to", "", "destination wallet address")
	sendAmount := sendCmd.Int("amount", 0, "amount to send")
	sendMine := sendCmd.Bool("mine", false, "Mine immediately on the same node")
	startNodeMiner := startNodeCmd.String("miner", "", "Enable mining mode and send reward to ")

	switch os.Args[1] {
	case "getBalance":
		err := gbCmd.Parse(os.Args[2:])
		blockchain.ErrHandler(err)
	case "createBlockchain":
		err := createBlockCmd.Parse(os.Args[2:])
		blockchain.ErrHandler(err)
	case "printChain":
		err := printChainCmd.Parse(os.Args[2:])
		blockchain.ErrHandler(err)
	case "send":
		err := sendCmd.Parse(os.Args[2:])
		blockchain.ErrHandler(err)
	case "createWallet":
		err := createWalletCmd.Parse(os.Args[2:])
		blockchain.ErrHandler(err)
	case "listAddresses":
		err := listAddressesCmd.Parse(os.Args[2:])
		blockchain.ErrHandler(err)
	case "ReIndexUTXO":
		err := ReIndexUTXOCmd.Parse(os.Args[2:])
		blockchain.ErrHandler(err)
	case "startNode":
		err := startNodeCmd.Parse(os.Args[2:])
		blockchain.ErrHandler(err)
	default:
		cli.printUsage()
		runtime.Goexit()
	}

	if gbCmd.Parsed() {
		if *getBalanceAddress == "" {
			gbCmd.Usage()
			runtime.Goexit()
		}
		cli.getBalance(*getBalanceAddress, nodeID)
	}

	if createBlockCmd.Parsed() {
		if *createBlockchainAddress == "" {
			createBlockCmd.Usage()
			runtime.Goexit()
		}
		cli.createBlockChain(nodeID, *createBlockchainAddress)
	}

	if sendCmd.Parsed() {
		if *sendFrom == "" || *sendTo == "" || *sendAmount <= 0 {
			sendCmd.Usage()
			runtime.Goexit()
		}

		cli.send(*sendFrom, *sendTo, *sendAmount, nodeID, *sendMine)
	}

	if printChainCmd.Parsed() {
		cli.printChain(nodeID)
	}

	if listAddressesCmd.Parsed() {
		cli.listAddresses(nodeID)
	}

	if createWalletCmd.Parsed() {
		cli.createWallet(nodeID)
	}

	if ReIndexUTXOCmd.Parsed() {
		cli.ReIndexUTXO()
	}

	if startNodeCmd.Parsed() {
		nodeID := os.Getenv("NODE_ID")
		if nodeID == "" {
			startNodeCmd.Usage()
			runtime.Goexit()
		}
		cli.StartNode(nodeID, *startNodeMiner)
	}
}
