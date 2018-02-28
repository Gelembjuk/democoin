package wallet

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"

	"github.com/gelembjuk/democoin/lib"
	"github.com/gelembjuk/democoin/lib/nodeclient"
)

const walletFile = "wallet.dat"

type AppInput struct {
	Command   string
	Address   string
	ToAddress string
	Amount    float64
	NodePort  int
	NodeHost  string
	DataDir   string
	Nodes     []lib.NodeAddr
	LogDest   string
}

type WalletCLI struct {
	Input      AppInput
	Node       lib.NodeAddr
	DataDir    string
	Nodes      []lib.NodeAddr
	NodeCLI    *nodeclient.NodeClient
	WalletsObj *Wallets
	NodeMode   bool
	Logger     *lib.LoggerMan
}

// Init wallet client object. This will manage execution
// of tasks related to a wallet
func (wc *WalletCLI) Init(logger *lib.LoggerMan, input AppInput) {
	wc.Logger = logger
	wc.Input = input
	wc.DataDir = input.DataDir

	wc.initNodeClient()
	wc.initWallets()

	wc.Node.Port = wc.Input.NodePort
	wc.Node.Host = wc.Input.NodeHost
}

// Creates Wallets object and fills it from a file if it exists
func (wc *WalletCLI) initWallets() error {
	wallets := Wallets{}
	wallets.Wallets = make(map[string]*Wallet)

	wallets.DataDir = wc.DataDir

	err := wallets.LoadFromFile()

	if err != nil && !os.IsNotExist(err) {
		return err
	}

	wc.WalletsObj = &wallets

	return nil
}

// Inits nodeclient object. It is used to communicate with a node
func (wc *WalletCLI) initNodeClient() {
	if wc.NodeCLI != nil {
		return
	}
	client := nodeclient.NodeClient{}

	client.Logger = wc.Logger
	nt := lib.NodeNetwork{}
	client.NodeNet = &nt

	wc.NodeCLI = &client
}

// Executes command based on input arguments
func (wc *WalletCLI) ExecuteCommand() error {
	wc.initNodeClient()

	if wc.Input.Command == "createwallet" {
		return wc.commandCreatewallet()

	} else if wc.Input.Command == "listaddresses" {
		return wc.commandListAddresses()

	} else if wc.Input.Command == "getbalances" ||
		wc.Input.Command == "listbalances" {
		return wc.commandListAddressesExt()

	} else if wc.Input.Command == "getbalance" {
		return wc.commandGetBalance()

	} else if wc.Input.Command == "send" {
		return wc.commandSend()

	} else if wc.Input.Command == "showunspent" {
		return wc.commandUnspentTransactions()

	} else if wc.Input.Command == "showhistory" {
		return wc.commandShowHistory()

	}

	return errors.New("Unknown wallets command")
}

// Creates new wallet and saves it in a wallets file
// Wallet is a pare of keys
func (wc *WalletCLI) commandCreatewallet() error {
	address, err := wc.WalletsObj.CreateWallet()

	if err != nil {
		return err
	}

	fmt.Printf("Your new address: %s\n", address)

	return nil
}

// List addresses (wallets) stored in the wallets file
func (wc *WalletCLI) commandListAddresses() error {
	addresses := wc.WalletsObj.GetAddresses()

	for _, address := range addresses {
		fmt.Println(address)
	}

	return nil
}

// Lists wallets and balance for each wallet
func (wc *WalletCLI) commandListAddressesExt() error {
	addresses := wc.WalletsObj.GetAddresses()

	for _, address := range addresses {
		// the wallet has to connect to node to execute this operation
		list, err := wc.NodeCLI.SendGetUnspent(wc.Node, address, []byte{})

		if err != nil {
			return err
		}

		balance := float64(0)

		for _, tx := range list.Transactions {
			balance += tx.Amount
		}

		fmt.Printf("%s - %f\n", address, balance)
	}

	return nil
}

// Displays history for a wallet (address) . All transactions
func (wc *WalletCLI) commandShowHistory() error {
	w := Wallet{}
	// check input
	if !w.ValidateAddress(wc.Input.Address) {
		return errors.New("Address is not valid")
	}

	// the wallet has to connect to node to execute this operation
	list, err := wc.NodeCLI.SendGetHistory(wc.Node, wc.Input.Address)

	if err != nil {
		return err
	}

	for _, rec := range list {
		if rec.IOType {
			fmt.Printf("%f\t In from\t%s\n", rec.Amount, rec.From)
		} else {
			fmt.Printf("%f\t Out To  \t%s\n", rec.Amount, rec.To)
		}

	}

	return nil
}

// Shows list of unspent transactions for an address
func (wc *WalletCLI) commandUnspentTransactions() error {
	w := Wallet{}
	// check input
	if !w.ValidateAddress(wc.Input.Address) {
		return errors.New("Address is not valid")
	}

	// the wallet has to connect to node to execute this operation
	list, err := wc.NodeCLI.SendGetUnspent(wc.Node, wc.Input.Address, []byte{})

	if err != nil {
		return err
	}

	balance := float64(0)

	for _, tx := range list.Transactions {

		fmt.Printf("%f\t from\t%s in transaction %s output #%d\n", tx.Amount, tx.From, hex.EncodeToString(tx.TXID), tx.Vout)
		balance += tx.Amount
	}

	fmt.Printf("\nBalance - %f\n", balance)

	return nil
}

// Requests a node for balance and displays it. for given address
func (wc *WalletCLI) commandGetBalance() error {
	w := Wallet{}
	// check input
	if !w.ValidateAddress(wc.Input.Address) {
		return errors.New("Address is not valid")
	}

	list, err := wc.NodeCLI.SendGetUnspent(wc.Node, wc.Input.Address, []byte{})

	if err != nil {
		return err
	}

	balance := float64(0)

	for _, tx := range list.Transactions {
		balance += tx.Amount
	}

	fmt.Printf("\nBalance - %f\n", balance)

	return nil
}

// Send money command. Connects to a node to do this operation
func (wc *WalletCLI) commandSend() error {
	w := Wallet{}
	// check input
	if !w.ValidateAddress(wc.Input.Address) {
		return errors.New("From Address is not valid")
	}
	if !w.ValidateAddress(wc.Input.ToAddress) {
		return errors.New("To Address is not valid")
	}

	if wc.Input.Amount <= 0 {
		return errors.New("The amount of transaction must be more 0")
	}

	wc.Logger.Trace.Printf("Prepare wallet to send data to node " + wc.Node.NodeAddrToString())

	// load wallet object for this address
	walletobj, err := wc.WalletsObj.GetWallet(wc.Input.Address)

	if err != nil {
		return err
	}
	// Prepares new transaction without signatures
	// This is just request to a node and it returns prepared transaction
	TX, DataToSign, err := wc.NodeCLI.SendRequestNewTransaction(wc.Node,
		walletobj.GetPublicKey(), wc.Input.ToAddress, wc.Input.Amount)

	if err != nil {
		return err
	}
	// Sign transaction.
	TX.SignData(walletobj.GetPrivateKey(), DataToSign)

	// Sends final complete transaction
	txID, err := wc.NodeCLI.SendNewTransaction(wc.Node, wc.Input.Address, TX)

	if err != nil {
		return err
	}

	fmt.Printf("Success. New transaction: %x", txID)

	return nil
}
