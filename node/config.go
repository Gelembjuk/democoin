package main

// This code reads command line arguments and config file
import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/gelembjuk/democoin/lib"
)

// Thi is the struct with all possible command line arguments
type AllPossibleArgs struct {
	Address     string
	From        string
	To          string
	Port        int
	Host        string
	NodePort    int
	NodeHost    string
	Genesis     string
	Amount      float64
	LogDest     string
	Transaction string
	View        string
}

// Input summary
type AppInput struct {
	Command       string
	MinterAddress string
	Port          int
	Host          string
	DataDir       string
	Nodes         []lib.NodeAddr
	Args          AllPossibleArgs
}

// Parses inout and config file. Command line arguments ovverride config file options
func GetAppInput() (AppInput, error) {
	input := AppInput{}

	if len(os.Args) < 2 {
		input.Command = "help"
		return input, nil
	}

	input.Command = os.Args[1]

	cmd := flag.NewFlagSet(input.Command, flag.ExitOnError)

	cmd.StringVar(&input.Args.Address, "address", "", "Address of operation")
	cmd.StringVar(&input.MinterAddress, "minter", "", "Wallet address which signs blocks")
	cmd.StringVar(&input.Args.Genesis, "genesis", "", "Genesis block text")
	cmd.StringVar(&input.Args.Transaction, "transaction", "", "Transaction ID")
	cmd.StringVar(&input.Args.From, "from", "", "Address to send money from")
	cmd.StringVar(&input.Args.To, "to", "", "Address to send money to")
	cmd.StringVar(&input.Args.Host, "host", "", "Node Server Host")
	cmd.StringVar(&input.Args.NodeHost, "nodehost", "", "Remote Node Server Host")
	cmd.IntVar(&input.Args.Port, "port", 0, "Node Server port")
	cmd.IntVar(&input.Args.NodePort, "nodeport", 0, "Remote Node Server port")
	cmd.Float64Var(&input.Args.Amount, "amount", 0, "Amount money to send")
	cmd.StringVar(&input.Args.LogDest, "logdest", "file", "Destination of logs. file or stdout")
	cmd.StringVar(&input.Args.View, "view", "", "View format")

	datadirPtr := cmd.String("datadir", "", "Location of data files, config, DB etc")
	err := cmd.Parse(os.Args[2:])

	if err != nil {
		return input, err
	}

	if *datadirPtr != "" {
		input.DataDir = *datadirPtr
		if input.DataDir[len(input.DataDir)-1:] != "/" {
			input.DataDir += "/"
		}
	}
	if input.DataDir == "" {
		input.DataDir = "data/"
	}

	if _, err := os.Stat(input.DataDir); os.IsNotExist(err) {
		os.Mkdir(input.DataDir, 0755)
	}

	input.Port = input.Args.Port
	input.Host = strings.Trim(input.Args.Host, " ")

	// read config file . command line arguments are more important than a config

	file, errf := os.Open(input.DataDir + "config.json")

	if errf != nil && !os.IsNotExist(errf) {
		// error is bad only if file exists but we can not open to read
		return input, errf
	}
	if errf == nil {
		config := AppInput{}
		// we open a file only if it exists. in other case options can be set with command line
		decoder := json.NewDecoder(file)
		err := decoder.Decode(&config)

		if err != nil {
			return input, err
		}

		if input.MinterAddress == "" && config.MinterAddress != "" {
			input.MinterAddress = config.MinterAddress
		}

		if input.Port < 1 && config.Port > 0 {
			input.Port = config.Port
		}

		if input.Host == "" && config.Host != "" {
			input.Host = config.Host
		}

		if len(config.Nodes) > 0 {
			input.Nodes = config.Nodes
		}
	}

	if input.Host == "" {
		input.Host = "localhost"
	}

	return input, nil
}

func (c AppInput) checkNeedsHelp() bool {
	if c.Command == "help" || c.Command == "" {
		return true
	}
	return false
}

func (c AppInput) printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  help - Prints this help")
	fmt.Println("  == Any of next commands can have optional argument [-datadir /path/to/dir] [-logdest stdout]==")
	fmt.Println("  createwallet - Generates a new key-pair and saves it into the wallet file")
	fmt.Println("  createblockchain -address ADDRESS -genesis GENESISTEXT - Create a blockchain and send genesis block reward to ADDRESS")
	fmt.Println("  initblockchain [-nodehost HOST] [-nodeport PORT] - Loads a blockchain from other node to init the DB.")
	fmt.Println("  printchain [-view short|long] - Print all the blocks of the blockchain. Default view is long")
	fmt.Println("  makeblock [-minter ADDRESS] - Try to mine new block if there are enough transactions")
	fmt.Println("  dropblock - Delete last block fro the block chain. All transaction are returned back to unapproved state")
	fmt.Println("  reindexunspent - Rebuilds the database of unspent transactions outputs")
	fmt.Println("  showunspent -address ADDRESS - Print the list of all unspent transactions and balance")
	fmt.Println("  unapprovedtransactions - Print the list of transactions not included in any block yet")

	fmt.Println("  getbalance -address ADDRESS - Get balance of ADDRESS")
	fmt.Println("  listaddresses - Lists all addresses from the wallet file")
	fmt.Println("  getbalances - Lists all addresses from the wallet file and show balance for each")
	fmt.Println("  addrhistory -address ADDRESS - Shows all transactions for a wallet address")

	fmt.Println("  send -from FROM -to TO -amount AMOUNT - Send AMOUNT of coins from FROM address to TO. ")
	fmt.Println("  canceltransaction -transaction TRANSACTIONID - Cancel unapproved transaction. NOTE!. This cancels only from local cache!")

	fmt.Println("  startnode [-minter ADDRESS] [-port PORT] - Start a node server. -minter defines minting address and -port - listening port")
	fmt.Println("  startintnode [-minter ADDRESS] [-port PORT] - Start a node server in interactive mode (no deamon). -minter defines minting address and -port - listening port")
	fmt.Println("  stopnode - Stop runnning node")
	fmt.Println("  nodestate - Print state of the node process")

	fmt.Println("  shownodes - Display list of nodes addresses, including inactive")
	fmt.Println("  addnode -nodehost HOST -nodeport PORT - Adds new node to list of connections")
	fmt.Println("  removenode -nodehost HOST -nodeport PORT - Removes a node from list of connections")
}
