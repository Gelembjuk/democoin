package transactions

import (
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	"github.com/gelembjuk/democoin/lib"
	"github.com/gelembjuk/democoin/lib/utils"
	"github.com/gelembjuk/democoin/lib/wallet"
	"github.com/gelembjuk/democoin/node/blockchain"
	"github.com/gelembjuk/democoin/node/database"
	"github.com/gelembjuk/democoin/node/structures"
)

type Manager struct {
	DB     database.DBManager
	Logger *utils.LoggerMan
}

func NewManager(DB database.DBManager, Logger *utils.LoggerMan) *Manager {
	return &Manager{DB, Logger}
}

func (n Manager) GetIndexManager() *TransactionsIndex {
	return NewTransactionIndex(n.DB, n.Logger)
}

func (n Manager) GetUnapprovedTransactionsManager() *UnApprovedTransactions {
	return &UnApprovedTransactions{n.DB, n.Logger}
}

func (n Manager) GetUnspentOutputsManager() *UnspentTransactions {
	return &UnspentTransactions{n.DB, n.Logger}
}

// Calculates balance of address. Uses DB of unspent trasaction outputs
// TODO must check lso cache of unapproved to skip return of transactions added as input
// to unapproved
func (n *Manager) GetAddressesBalance(addresses []string) (map[string]wallet.WalletBalance, error) {
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
func (n *Manager) GetAddressBalance(address string) (wallet.WalletBalance, error) {
	balance := wallet.WalletBalance{}

	n.Logger.Trace.Printf("Get balance %s", address)
	result, err := n.GetUnspentOutputsManager().GetAddressBalance(address)

	if err != nil {
		n.Logger.Trace.Printf("Error 1 %s", err.Error())
		return balance, err
	}

	balance.Approved = result

	// get pending
	n.Logger.Trace.Printf("Get pending %s", address)
	p, err := n.GetAddressPendingBalance(address)

	if err != nil {
		n.Logger.Trace.Printf("Error 2 %s", err.Error())
		return balance, err
	}
	balance.Pending = p

	balance.Total = balance.Approved + balance.Pending

	return balance, nil
}

// Calculates pending balance of address.
func (n *Manager) GetAddressPendingBalance(address string) (float64, error) {
	PubKeyHash, _ := utils.AddresToPubKeyHash(address)

	// inputs this is what a wallet spent from his real approved balance
	// outputs this is what a wallet receives (and didn't resulse in other pending TXs)
	// slice inputs contains only inputs from approved transactions outputs
	_, outputs, inputs, err := n.GetUnapprovedTransactionsManager().GetPreparedBy(PubKeyHash)

	if err != nil {
		return 0, err
	}

	pendingbalance := float64(0)

	for _, o := range outputs {
		// this is amount sent to this wallet and this
		// list contains only what was not spent in other prepared TX
		pendingbalance += o.Value
	}

	// we need to know values for inputs. this are inputs based on TXs that are in approved
	// input TX can be confirmed (in unspent outputs) or unconfirmed . we need to look for it in
	// both places
	for _, i := range inputs {
		n.Logger.Trace.Printf("find input %s for tx %x", i, i.Txid)
		v, err := n.GetUnspentOutputsManager().GetInputValue(i)

		if err != nil {
			/*
				if err, ok := err.(*TXNotFoundError); ok && err.GetKind() == TXNotFoundErrorUnspent {

					// check this TX in prepared
					v2, err := n.GetUnapprovedTransactionsManager().GetInputValue(i)

					if err != nil {
						return 0, errors.New(fmt.Sprintf("Pending Balance Error: input check fails on unapproved: %s", err.Error()))
					}
					v = v2
				} else {
					return 0, errors.New(fmt.Sprintf("Pending Balance Error: input check fails on unspent: %s", err.Error()))
				}
			*/
			return 0, errors.New(fmt.Sprintf("Pending Balance Error: input check fails on unspent: %s", err.Error()))
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
func (n *Manager) CancelTransaction(txidstr string) error {
	if txidstr == "" {
		return errors.New("Transaction ID not provided")
	}

	txid, err := hex.DecodeString(txidstr)

	if err != nil {
		return err
	}

	found, err := n.GetUnapprovedTransactionsManager().Delete(txid)

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
func (n *Manager) VerifyTransactionQuick(tx *structures.Transaction) (bool, error) {
	notFoundInputs, inputTXs, err := n.GetUnspentOutputsManager().VerifyTransactionsOutputsAreNotSpent(tx.Vin)

	if err != nil {
		return false, err
	}

	if len(notFoundInputs) > 0 {
		// some inputs are not existent
		// we need to try to find them in list of unapproved transactions
		// if not found then it is bad transaction
		err := n.GetUnapprovedTransactionsManager().CheckInputsArePrepared(notFoundInputs, inputTXs)

		if err != nil {
			return false, err
		}
	}
	// verify signatures

	err = tx.Verify(inputTXs)

	if err != nil {
		return false, err
	}
	return true, nil
}

// Verify if transaction is correct.
// If it is build on correct outputs.This does checks agains blockchain. Needs more time
// NOTE Transaction can have outputs of other transactions that are not yet approved.
// This must be considered as correct case
func (n *Manager) VerifyTransactionDeep(tx *structures.Transaction, prevtxs []*structures.Transaction, tip []byte) (bool, error) {
	inputTXs, notFoundInputs, err := n.GetInputTransactionsState(tx, tip)
	if err != nil {
		return false, err
	}

	if len(notFoundInputs) > 0 {
		// some of inputs can be from other transactions in this pool
		inputTXs, err = n.GetUnapprovedTransactionsManager().CheckInputsWereBefore(notFoundInputs, prevtxs, inputTXs)

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

// Verifies transaction inputs. Check if that are real existent transactions. And that outputs are not yet used
// Is some transaction is not in blockchain, returns nil pointer in map and this input in separate map
// Missed inputs can be some unconfirmed transactions
// Returns: map of previous transactions (full info about input TX). map by input index
// next map is wrong input, where a TX is not found.
func (n *Manager) GetInputTransactionsState(tx *structures.Transaction,
	tip []byte) (map[int]*structures.Transaction, map[int]structures.TXInput, error) {

	//n.Logger.Trace.Printf("get state %x , tip %x", tx.ID, tip)

	prevTXs := make(map[int]*structures.Transaction)
	badinputs := make(map[int]structures.TXInput)

	if tx.IsCoinbase() {

		return prevTXs, badinputs, nil
	}

	bcMan, err := blockchain.NewBlockchainManager(n.DB, n.Logger)

	if err != nil {
		return nil, nil, err
	}

	for vind, vin := range tx.Vin {
		//n.Logger.Trace.Printf("Load in tx %x", vin.Txid)
		txBockHashes, err := n.GetIndexManager().GetTranactionBlocks(vin.Txid)

		if err != nil {
			n.Logger.Trace.Printf("Error %s", err.Error())
			return nil, nil, err
		}

		txBockHash, err := bcMan.ChooseHashUnderTip(txBockHashes, tip)

		if err != nil {
			n.Logger.Trace.Printf("Error getting correct hash %s", err.Error())
			return nil, nil, err
		}

		var prevTX *structures.Transaction

		if txBockHash == nil {
			//n.Logger.Trace.Printf("Not found TX")
			prevTX = nil
		} else {

			//n.Logger.Trace.Printf("tx block hash %x %x", vin.Txid, txBockHash)
			// check this block is part of chain
			heigh, err := bcMan.GetBlockInTheChain(txBockHash, tip)

			if err != nil {
				return nil, nil, err
			}

			if heigh >= 0 {
				// if block is in this chain
				//n.Logger.Trace.Printf("block height %d", heigh)
				prevTX, err = bcMan.FindTransactionByBlock(vin.Txid, txBockHash)

				if err != nil {
					return nil, nil, err
				}
			} else {
				// TX is in some other block that is in other chain. we want to include it in new block
				// so, we consider this TX as missed from blocks (unapproved)
				//n.Logger.Trace.Printf("Not found TX . type 2")
				prevTX = nil
			}
		}

		if prevTX == nil {
			// transaction not found
			badinputs[vind] = vin
			prevTXs[vind] = nil
			//n.Logger.Trace.Printf("tx is not in blocks")
		} else {
			//n.Logger.Trace.Printf("tx found")
			// check if this input was not yet spent somewhere
			spentouts, err := n.GetIndexManager().GetTranactionOutputsSpent(vin.Txid, txBockHash, tip)

			if err != nil {
				return nil, nil, err
			}
			//n.Logger.Trace.Printf("spending of tx count %d", len(spentouts))
			if len(spentouts) > 0 {

				for _, o := range spentouts {
					if o.OutInd == vin.Vout {
						heigh, err := bcMan.GetBlockInTheChain(o.BlockHash, tip)

						if err != nil {
							return nil, nil, err
						}

						if heigh < 0 {
							// this block is not found in current tip chain
							// so, this spending can be ignored
							continue
						}
						return nil, nil, errors.New("Transaction input was already spent before")
					}
				}
			}
			// the transaction out was not yet spent
			prevTXs[vind] = prevTX
		}
	}

	return prevTXs, badinputs, nil
}

/*
* Allows to iterate over unapproved transactions, for eample to display them . Accepts callback as argument
 */
func (n *Manager) IterateUnapprovedTransactions(callback UnApprovedTransactionsIteratorInterface) (int, error) {
	return n.GetUnapprovedTransactionsManager().IterateTransactions(callback)
}

func (n *Manager) ReceivedNewTransactionData(txBytes []byte, Signatures [][]byte) (*structures.Transaction, error) {
	tx := structures.Transaction{}
	err := tx.DeserializeTransaction(txBytes)

	if err != nil {
		return nil, err
	}

	err = tx.SetSignatures(Signatures)

	if err != nil {
		return nil, err
	}

	err = n.ReceivedNewTransaction(&tx)

	if err != nil {
		return nil, err
	}

	return &tx, nil
}

// New transaction reveived from other node. We need to verify and add to cache of unapproved
func (n *Manager) ReceivedNewTransaction(tx *structures.Transaction) error {
	// verify this transaction
	good, err := n.VerifyTransactionQuick(tx)

	if err != nil {
		return err
	}
	if !good {
		return errors.New("Transaction verification failed")
	}
	// if all is ok, add it to the list of unapproved
	return n.GetUnapprovedTransactionsManager().Add(tx)
}

// Request to make new transaction and prepare data to sign
// This function should find good input transactions for this amount
// Including inputs from unapproved transactions if no good approved transactions yet
func (n *Manager) PrepareNewTransaction(PubKey []byte, to string, amount float64) ([]byte, [][]byte, error) {
	amount, err := strconv.ParseFloat(fmt.Sprintf("%.8f", amount), 64)

	if err != nil {
		return nil, nil, err
	}
	PubKeyHash, _ := utils.HashPubKey(PubKey)
	// get from pending transactions. find outputs used by this pubkey
	pendinginputs, pendingoutputs, _, err := n.GetUnapprovedTransactionsManager().GetPreparedBy(PubKeyHash)
	n.Logger.Trace.Printf("Pending transactions state: %d- inputs, %d - unspent outputs", len(pendinginputs), len(pendingoutputs))

	inputs, prevTXs, totalamount, err := n.GetUnspentOutputsManager().GetNewTransactionInputs(PubKey, to, amount, pendinginputs)

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
			n.GetUnspentOutputsManager().ExtendNewTransactionInputs(PubKey, amount, totalamount,
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
func (n *Manager) PrepareNewTransactionComplete(PubKey []byte, to string, amount float64,
	inputs []structures.TXInput, totalamount float64, prevTXs map[string]structures.Transaction) ([]byte, [][]byte, error) {

	var outputs []structures.TXOutput

	// Build a list of outputs
	from, _ := utils.PubKeyToAddres(PubKey)
	outputs = append(outputs, *structures.NewTXOutput(amount, to))

	if totalamount > amount && totalamount-amount > lib.SmallestUnit {
		outputs = append(outputs, *structures.NewTXOutput(totalamount-amount, from)) // a change
	}

	inputTXs := make(map[int]*structures.Transaction)

	for vinInd, vin := range inputs {
		tx := prevTXs[hex.EncodeToString(vin.Txid)]
		inputTXs[vinInd] = &tx
	}

	tx := structures.Transaction{nil, inputs, outputs, 0}
	tx.TimeNow()

	signdata, err := tx.PrepareSignData(inputTXs)

	if err != nil {
		return nil, nil, err
	}

	txBytes, err := tx.Serialize()

	if err != nil {
		return nil, nil, err
	}

	return txBytes, signdata, nil
}

// Send amount of money if a node is not running.
// This function only adds a transaction to queue
// Attempt to send the transaction to other nodes will be done in other place
//
// Returns new transaction hash. This return can be used to try to send transaction
// to other nodes or to try mining
func (n *Manager) Send(PubKey []byte, privKey ecdsa.PrivateKey, to string, amount float64) (*structures.Transaction, error) {

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

	txBytes, DataToSign, err := n.PrepareNewTransaction(PubKey, to, amount)

	if err != nil {
		return nil, errors.New(fmt.Sprintf("Prepare error: %s", err.Error()))
	}

	signatures, err := utils.SignDataSet(PubKey, privKey, DataToSign)

	if err != nil {
		return nil, errors.New(fmt.Sprintf("Sign error: %s", err.Error()))
	}
	NewTX, err := n.ReceivedNewTransactionData(txBytes, signatures)

	if err != nil {
		return nil, errors.New(fmt.Sprintf("Final ading TX error: %s", err.Error()))
	}

	return NewTX, nil
}

func (n *Manager) CleanUnapprovedCache() error {
	return n.GetUnapprovedTransactionsManager().CleanUnapprovedCache()
}

// to execute when new block added to the top of blockchain
func (n *Manager) BlockAdddedToTop(block *structures.Block) error {
	// update caches
	n.GetIndexManager().BlockAdded(block)
	n.GetUnapprovedTransactionsManager().DeleteFromBlock(block)
	n.GetUnspentOutputsManager().UpdateOnBlockAdd(block)
	return nil
}

// check if transaction exists. it checks in all places. in approved and pending
func (n *Manager) GetIfExists(txid []byte) (*structures.Transaction, error) {
	// check in pending first
	tx, err := n.GetUnapprovedTransactionsManager().GetIfExists(txid)

	if !(tx == nil && err == nil) {
		return tx, err
	}

	// not exist on pending and no error
	// try to check in approved . it will look only in primary branch
	tx, _, _, err = n.GetIndexManager().GetTransactionAllInfo(txid, []byte{})
	return tx, err
}
