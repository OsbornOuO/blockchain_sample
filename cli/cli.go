package cli

import (
	"blockchain/blockchain"
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
	fmt.Println(" send -from FROM -to TO -amount AMOUNT - Send amount")
	fmt.Println(" createWallet  - Creates a new Wallet")
	fmt.Println(" listAddresses - List the address in our wallet file")
}

func (cli *CommandLine) validateArgs() {
	if len(os.Args) < 2 {
		cli.printUsage()
		runtime.Goexit()
	}
}

func (cli *CommandLine) printChain() {
	chain := blockchain.ContinueBlockChain("")
	defer chain.Database.Close()
	iter := chain.Iterator()

	for {
		b := iter.Next()

		fmt.Printf("previous Hash: %x\n", b.PrevHash)
		fmt.Printf("Hash: %x\n", b.Hash)

		pow := blockchain.NewProof(b)
		fmt.Printf("Pow: %s\n", strconv.FormatBool(pow.Validate()))
		fmt.Println()

		if len(b.PrevHash) == 0 {
			break
		}
	}
}
func (cli *CommandLine) createBlockChain(address string) {
	chain := blockchain.InitBlockChain(address)
	chain.Database.Close()
	fmt.Println("Finished!")
}

func (cli *CommandLine) createWallet() {
	ws, err := wallet.CreateWallets()
	if err != nil {
		log.Print(err)
	}

	address := ws.AddWallet()
	ws.SaveFile()

	fmt.Println("address is " + address)

	fmt.Println("Finished!")
}

func (cli *CommandLine) listAddresses() {
	wallets, err := wallet.CreateWallets()
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
func (cli *CommandLine) getBalance(address string) {
	chain := blockchain.ContinueBlockChain(address)
	defer chain.Database.Close()

	balance := 0
	utx0s := chain.FindUTX0(address)

	for _, out := range utx0s {
		balance += out.Value
	}

	fmt.Printf("Balance of %s: %d\n", address, balance)
}

func (cli *CommandLine) send(from, to string, amount int) {
	chain := blockchain.ContinueBlockChain(from)
	defer chain.Database.Close()

	tx := blockchain.NewTransaction(from, to, amount, chain)
	chain.AddBlock([]*blockchain.Transaction{tx})

	fmt.Println("Success!")
}

// Run ...
func (cli *CommandLine) Run() {
	cli.validateArgs()

	gbCmd := flag.NewFlagSet("getBalance", flag.ExitOnError)
	createBlockCmd := flag.NewFlagSet("createBlockchain", flag.ExitOnError)
	sendCmd := flag.NewFlagSet("send", flag.ExitOnError)
	printChainCmd := flag.NewFlagSet("printChain", flag.ExitOnError)
	createWalletCmd := flag.NewFlagSet("createWallet", flag.ExitOnError)
	listAddressesCmd := flag.NewFlagSet("listAddresses", flag.ExitOnError)

	getBalanceAddress := gbCmd.String("address", "", "get address balance")
	createBlockchainAddress := createBlockCmd.String("address", "", "create block with address")
	sendFrom := sendCmd.String("from", "", "source wallet address")
	sendTo := sendCmd.String("to", "", "destination wallet address")
	sendAmount := sendCmd.Int("amount", 0, "amount to send")

	switch os.Args[1] {
	case "getBalance":
		err := gbCmd.Parse(os.Args[2:])
		blockchain.ErrHandler(err)
	case "createBlockChain":
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
	default:
		cli.printUsage()
		runtime.Goexit()
	}

	if gbCmd.Parsed() {
		if *getBalanceAddress == "" {
			gbCmd.Usage()
			runtime.Goexit()
		}
		cli.getBalance(*getBalanceAddress)
	}

	if createBlockCmd.Parsed() {
		if *createBlockchainAddress == "" {
			createBlockCmd.Usage()
			runtime.Goexit()
		}
		cli.createBlockChain(*createBlockchainAddress)
	}

	if sendCmd.Parsed() {
		if *sendFrom == "" || *sendTo == "" || *sendAmount <= 0 {
			sendCmd.Usage()
			runtime.Goexit()
		}

		cli.send(*sendFrom, *sendTo, *sendAmount)
	}

	if printChainCmd.Parsed() {
		cli.printChain()
	}

	if listAddressesCmd.Parsed() {
		cli.listAddresses()
	}

	if createWalletCmd.Parsed() {
		cli.createWallet()
	}
}