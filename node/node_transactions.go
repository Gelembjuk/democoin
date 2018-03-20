package main

import (
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	"github.com/gelembjuk/democoin/lib"
	"github.com/gelembjuk/democoin/lib/transaction"
	"github.com/gelembjuk/democoin/lib/wallet"
)

type NodeTransactions struct {
	Logger        *lib.LoggerMan
	BC            *Blockchain
	UnspentTXs    UnspentTransactions
	UnapprovedTXs UnApprovedTransactions
	DataDir       string
}

// Calculates balance of address. Uses DB of unspent trasaction outputs
// TODO must check lso cache of unapproved to skip return of transactions added as input
// to unapproved
func (n *NodeTransactions) GetAddressesBalance(addresses []string) (map[string]wallet.WalletBalance, error) {
	result := map[string]wallet.WalletBalance{}

	for _, address := range addresses {
		balance, err := n.GetAddressBalance(address)

		if err != nil {
			return result, err
		}
		result[string(address)] = balance
	}

	return result, nil
}

// Calculates balance of address. Uses DB of unspent trasaction outputs
// and cache of pending transactions
func (n *NodeTransactions) GetAddressBalance(address string) (wallet.WalletBalance, error) {
	balance := wallet.WalletBalance{}

	result, err := n.UnspentTXs.GetAddressBalance(address)

	if err != nil {
		return balance, err
	}

	balance.Approved = result

	// get pending
	p, err := n.GetAddressPendingBalance(address)

	if err != nil {
		return balance, err
	}
	balance.Pending = p

	balance.Total = balance.Approved + balance.Pending

	return balance, nil
}

// Calculates pending balance of address.
func (n *NodeTransactions) GetAddressPendingBalance(address string) (float64, error) {
	PubKeyHash, _ := lib.AddresToPubKeyHash(address)

	// inputs this is what a wallet spent
	// outputs this is what a wallet receives
	_, outputs, inputs, err := n.UnapprovedTXs.GetPreparedBy(PubKeyHash)

	if err != nil {
		return 0, err
	}

	pendingbalance := float64(0)

	for _, o := range outputs {
		// this is amount sent to this wallet and this
		// list contains only what was not spent in other prepared TX
		pendingbalance += o.Value
	}

	// we need to know values for inputs. this are inputs based on TXs that are in unapproved
	for _, i := range inputs {
		v, err := n.UnspentTXs.GetInputValue(i)

		if err != nil {
			return 0, err
		}
		pendingbalance -= v
	}

	return pendingbalance, nil
}

/*
* Cancels unapproved transaction.
* NOTE this can work only for local node. it a transaction was already sent to other nodes, it will not be canceled
* and can be added to next block
 */
func (n *NodeTransactions) CancelTransaction(txidstr string) error {
	if txidstr == "" {
		return errors.New("Transaction ID not provided")
	}

	txid, err := hex.DecodeString(txidstr)

	if err != nil {
		return err
	}

	found, err := n.UnapprovedTXs.Delete(txid)

	if err == nil && !found {
		return errors.New("Transaction ID not found in the list of unapproved transactions")
	}

	return nil
}

// Verify if transaction is correct.
// If it is build on correct outputs.It checks only cache of unspent transactions
// This function doesn't do full alidation with blockchain
// NOTE Transaction can have outputs of other transactions that are not yet approved.
// This must be considered as correct case
func (n *NodeTransactions) VerifyTransactionQuick(tx *transaction.Transaction) (bool, error) {
	notFoundInputs, err := n.UnspentTXs.VerifyTransactionsOutputsAreNotSpent(tx.Vin)

	if err != nil {
		return false, err
	}

	if len(notFoundInputs) > 0 {
		// some inputs are not existent
		// we need to try to find them in list of unapproved transactions
		// if not found then it is bad transaction
		err := n.UnapprovedTXs.CheckInputsArePrepared(notFoundInputs)

		if err != nil {
			return false, err
		}
	}
	return true, nil
}

// Verify if transaction is correct.
// If it is build on correct outputs.This does checks agains blockchain. Needs more time
// NOTE Transaction can have outputs of other transactions that are not yet approved.
// This must be considered as correct case
func (n *NodeTransactions) VerifyTransactionDeep(tx *transaction.Transaction, prevtxs []*transaction.Transaction, tip []byte) (bool, error) {
	inputTXs, notFoundInputs, err := n.BC.GetInputTransactionsState(tx, tip)

	if err != nil {
		return false, err
	}

	if len(notFoundInputs) > 0 {
		// some of inputs can be from other transactions in this pool
		inputTXs, err = n.UnapprovedTXs.CheckInputsWereBefore(notFoundInputs, prevtxs, inputTXs)

		if err != nil {
			return false, err
		}
	}
	// do final check against inputs

	err = tx.Verify(inputTXs)

	if err != nil {
		return false, err
	}

	return true, nil
}

/*
* Allows to iterate over unapproved transactions, for eample to display them . Accepts callback as argument
 */
func (n *NodeTransactions) IterateUnapprovedTransactions(callback UnApprovedTransactionsIteratorInterface) (int, error) {
	return n.UnapprovedTXs.IterateTransactions(callback)
}

// New transaction reveived from other node. We need to verify and add to cache of unapproved
func (n *NodeTransactions) ReceivedNewTransaction(tx *transaction.Transaction) error {
	// verify this transaction
	good, err := n.VerifyTransactionQuick(tx)

	if err != nil {
		return err
	}
	if !good {
		return errors.New("Transaction verification failed")
	}
	// if all is ok, add it to the list of unapproved
	return n.UnapprovedTXs.Add(tx)
}

// Request to make new transaction and prepare data to sign
// This function should find good input transactions for this amount
// Including inputs from unapproved transactions if no good approved transactions yet
func (n *NodeTransactions) PrepareNewTransaction(PubKey []byte, to string, amount float64) (*transaction.Transaction, [][]byte, error) {
	amount, err := strconv.ParseFloat(fmt.Sprintf("%.8f", amount), 64)

	if err != nil {
		return nil, nil, err
	}
	PubKeyHash, _ := lib.HashPubKey(PubKey)
	// get from pending transactions. find outputs used by this pubkey
	pendinginputs, pendingoutputs, _, err := n.UnapprovedTXs.GetPreparedBy(PubKeyHash)
	n.Logger.Trace.Printf("Pending transactions state: %d- inputs, %d - unspent outputs", len(pendinginputs), len(pendingoutputs))

	inputs, prevTXs, totalamount, err := n.UnspentTXs.GetNewTransactionInputs(PubKey, to, amount, pendinginputs)

	if err != nil {
		return nil, nil, err
	}

	n.Logger.Trace.Printf("First step prepared amount %f of %f", totalamount, amount)

	if totalamount < amount {
		// no anough funds in confirmed transactions
		// pending must be used

		if len(pendingoutputs) == 0 {
			// nothing to add
			return nil, nil, errors.New("No enough funds for requested transaction")
		}
		inputs, prevTXs, totalamount, err =
			n.UnspentTXs.ExtendNewTransactionInputs(PubKey, amount, totalamount,
				inputs, prevTXs, pendingoutputs)

		if err != nil {
			return nil, nil, err
		}
	}

	n.Logger.Trace.Printf("Second step prepared amount %f of %f", totalamount, amount)

	if totalamount < amount {
		return nil, nil, errors.New("No anough funds to make new transaction")
	}

	return n.PrepareNewTransactionComplete(PubKey, to, amount, inputs, totalamount, prevTXs)
}

//
func (n *NodeTransactions) PrepareNewTransactionComplete(PubKey []byte, to string, amount float64,
	inputs []transaction.TXInput, totalamount float64, prevTXs map[string]transaction.Transaction) (*transaction.Transaction, [][]byte, error) {

	var outputs []transaction.TXOutput

	// Build a list of outputs
	from, _ := lib.PubKeyToAddres(PubKey)
	outputs = append(outputs, *transaction.NewTXOutput(amount, to))

	if totalamount > amount && totalamount-amount > lib.SmallestUnit {
		outputs = append(outputs, *transaction.NewTXOutput(totalamount-amount, from)) // a change
	}

	tx := transaction.Transaction{nil, inputs, outputs, 0}
	tx.TimeNow()
	n.Logger.Trace.Println("Prepare sign data")
	n.Logger.Trace.Println(tx)
	n.Logger.Trace.Println(prevTXs)
	signdata, err := tx.PrepareSignData(prevTXs)

	if err != nil {
		return nil, nil, err
	}

	return &tx, signdata, nil
}

// Send amount of money if a node is not running.
// This function only adds a transaction to queue
// Attempt to send the transaction to other nodes will be done in other place
//
// Returns new transaction hash. This return can be used to try to send transaction
// to other nodes or to try mining
func (n *NodeTransactions) Send(PubKey []byte, privKey ecdsa.PrivateKey, to string, amount float64) (*transaction.Transaction, error) {

	if amount <= 0 {
		return nil, errors.New("Amount must be positive value")
	}
	if to == "" {
		return nil, errors.New("Recipient address is not provided")
	}
	w := wallet.Wallet{}

	if !w.ValidateAddress(to) {
		return nil, errors.New("Recipient address is not valid")
	}

	NewTX, DataToSign, err := n.PrepareNewTransaction(PubKey, to, amount)

	if err != nil {
		return nil, err
	}

	n.Logger.Trace.Println(NewTX)
	n.Logger.Trace.Println("Data to sign")
	for _, d := range DataToSign {
		n.Logger.Trace.Printf("%x", d)
	}

	err = NewTX.SignData(privKey, DataToSign)

	if err != nil {
		return nil, err
	}
	err = n.ReceivedNewTransaction(NewTX)

	if err != nil {
		n.Logger.Trace.Printf("Sending Error for %x: %s", NewTX.ID, err.Error())
		return nil, err
	}

	return NewTX, nil
}
