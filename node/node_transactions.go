package main

import (
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"

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

/*
* Calculates balance of address. Uses DB of unspent trasaction outputs
* TODO must check lso cache of unapproved to skip return of transactions added as input
* to unapproved
 */
func (n *NodeTransactions) GetAddressesBalance(addresses []string) (map[string]float64, error) {
	result := map[string]float64{}

	for _, address := range addresses {
		balance, err := n.UnspentTXs.GetAddressBalance(address)

		if err != nil {
			return result, err
		}
		result[string(address)] = balance
	}

	return result, nil
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

/*
* Verify if transaction is correct.
* If it is build on correct outputs.
* NOTE Traaction can have outputs of other transactions that are not yet approved.
* This must be considered as correct case
 */
func (n *NodeTransactions) VerifyTransaction(tx *transaction.Transaction) error {
	notFoundInputs, err := n.UnspentTXs.VerifyTransactionsOutputsAreNotSpent(tx.Vin)

	if err != nil {
		return err
	}

	if len(notFoundInputs) > 0 {
		return errors.New(fmt.Sprintf("%d input trsnactions are already spent!", len(notFoundInputs)))
	}
	return nil
}

/*
* Allows to iterate over unapproved transactions, for eample to display them . Accepts callback as argument
 */
func (n *NodeTransactions) IterateUnapprovedTransactions(callback UnApprovedTransactionsIteratorInterface) (int, error) {
	return n.UnapprovedTXs.IterateTransactions(callback)
}

/*
* New transaction reveived from other node. We need to verify and add to cache of unapproved
 */
func (n *NodeTransactions) NewTransaction(tx *transaction.Transaction) error {
	// verify this transaction
	err := n.VerifyTransaction(tx)

	if err != nil {
		return nil
	}
	// if all is ok, add it to the list of unapproved
	return n.UnapprovedTXs.Add(tx)
}

/*
* Send amount of money if a node is not running.
* This function only adds a transaction to queue
* Attempt to send the transaction to other nodes will be done in other place
*
* Returns new transaction hash. This return can be used to try to send transaction
* to other nodes or to try mining
 */
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

	NewTX, DataToSign, err := n.UnspentTXs.NewTransaction(PubKey, to, amount)

	if err != nil {
		return nil, err
	}

	err = NewTX.SignData(privKey, DataToSign)

	if err != nil {
		return nil, err
	}

	// final verification of the transaction
	// but it is not required here.
	err = n.VerifyTransaction(NewTX)

	// add transactions to queue of unapproved
	err = n.UnapprovedTXs.Add(NewTX)

	return NewTX, nil
}
