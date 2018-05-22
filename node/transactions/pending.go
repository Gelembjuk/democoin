package transactions

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"

	"github.com/gelembjuk/democoin/lib/utils"
	"github.com/gelembjuk/democoin/node/blockchain"
	"github.com/gelembjuk/democoin/node/database"
	"github.com/gelembjuk/democoin/node/transaction"
)

type UnApprovedTransactions struct {
	DB     database.DBManager
	Logger *utils.LoggerMan
}
type UnApprovedTransactionsIteratorInterface func(txhash, txstr string)

// Check if transaction inputs are pointed to some prepared transactions.
// Check conflicts too. Same output can not be repeated twice
func (u *UnApprovedTransactions) CheckInputsArePrepared(inputs map[int]transaction.TXInput, inputTXs map[int]*transaction.Transaction) error {
	checked := map[string][]int{}

	for vinInd, vin := range inputs {
		// look if not yet checked

		txstr := hex.EncodeToString(vin.Txid)

		if outs, ok := checked[txstr]; ok {
			// tx was checked
			for _, out := range outs {
				if out == vin.Vout {
					// this output was already used in outher input
					return errors.New(fmt.Sprintf("Duplicate usage of transaction output: %s - %d", txstr, out))
				}
			}
		}

		// check if this transaction exists
		tx, err := u.GetIfExists(vin.Txid)

		if err != nil {
			return err
		}

		if tx == nil {
			return NewTXVerifyError("Input transaction is not found in prepared to approve", TXVerifyErrorNoInput, vin.Txid)
		}
		inputTXs[vinInd] = tx
		checked[txstr] = append(checked[txstr], vin.Vout)
	}
	return nil
}

// Check if transaction inputs are pointed to some non approved transactions.
// That are listed in a block before this transactions
// Receives list of inputs and previous transactions
// and input transactions for this tx
// Check conflicts too. Same output can not be repeated twice

func (u *UnApprovedTransactions) CheckInputsWereBefore(
	inputs map[int]transaction.TXInput, prevTXs []*transaction.Transaction,
	inputTXs map[int]*transaction.Transaction) (map[int]*transaction.Transaction, error) {

	checked := map[string][]int{}

	for vind, vin := range inputs {
		// look if not yet checked

		txstr := hex.EncodeToString(vin.Txid)

		if outs, ok := checked[txstr]; ok {
			// tx was checked
			for _, out := range outs {
				if out == vin.Vout {
					// this output was already used in outher input
					return inputTXs, errors.New("Duplicate usage of transaction output")
				}
			}
		}

		// check if this transaction exists in the list
		exists := false

		for _, tx := range prevTXs {
			if bytes.Compare(vin.Txid, tx.ID) == 0 {
				inputTXs[vind] = tx
				exists = true
				break
			}
		}

		if !exists {
			return inputTXs, NewTXVerifyError("Input transaction is not found in prepared to approve", TXVerifyErrorNoInput, vin.Txid)
		}

		checked[txstr] = append(checked[txstr], vin.Vout)
	}
	return inputTXs, nil
}

// Returns pending transations info prepared by address
// Return contains:
// List of all inputs used by this PubKeyHash
// List of Outputs that were not yet used in any input returns in the first list
// List of inputs based on non-approved outputs (sub list of the first list)
func (u *UnApprovedTransactions) GetPreparedBy(PubKeyHash []byte) ([]transaction.TXInput,
	[]*transaction.TXOutputIndependent, []transaction.TXInput, error) {

	inputs := []transaction.TXInput{}
	outputs := []*transaction.TXOutputIndependent{}

	utdb, err := u.DB.GetUnapprovedTransactionsObject()

	if err != nil {
		return nil, nil, nil, err
	}

	err = utdb.ForEach(func(k, txBytes []byte) error {
		tx := transaction.Transaction{}
		err = tx.DeserializeTransaction(txBytes)

		if err != nil {
			return err
		}

		sender := []byte{}

		if !tx.IsCoinbase() {
			sender = tx.Vin[0].PubKey

			for _, vin := range tx.Vin {
				if vin.UsesKey(PubKeyHash) {
					inputs = append(inputs, vin)
				}
			}
		}
		for indV, vout := range tx.Vout {
			if vout.IsLockedWithKey(PubKeyHash) {
				voutind := transaction.TXOutputIndependent{}
				// we are settings serialised transaction in place of block hash
				// we don't have a block for such ransaction , but we need full transaction later
				voutind.LoadFromSimple(vout, tx.ID, indV, sender, tx.IsCoinbase(), txBytes)
				outputs = append(outputs, &voutind)
			}
		}
		return nil
	})

	if err != nil {
		return nil, nil, nil, err
	}

	// outputs not yet used in other pending transactions
	realoutputs := []*transaction.TXOutputIndependent{}
	// inputs based on approved transactions
	pendinginputs := []transaction.TXInput{}

	for _, vout := range outputs {
		used := false
		for _, vin := range inputs {
			if bytes.Compare(vin.Txid, vout.TXID) == 0 && vin.Vout == vout.OIndex {
				// this output is already used in other pending transaction
				used = true
				break
			}
		}
		if !used {
			realoutputs = append(realoutputs, vout)
		}
	}
	for _, vin := range inputs {
		pendingout := false

		for _, vout := range outputs {
			if bytes.Compare(vin.Txid, vout.TXID) == 0 && vin.Vout == vout.OIndex {
				// this input uses pending output
				pendingout = true
				break
			}
		}

		if !pendingout {
			pendinginputs = append(pendinginputs, vin)
		}
	}
	return inputs, realoutputs, pendinginputs, nil
}

// Get input value for TX in the cache
func (u *UnApprovedTransactions) GetInputValue(input transaction.TXInput) (float64, error) {
	u.Logger.Trace.Printf("Find TX %x in unapproved", input.Txid)
	tx, err := u.GetIfExists(input.Txid)

	if err != nil {
		return 0, err
	}

	if tx == nil {
		return 0, errors.New("TX not found in cache of unapproved")
	}

	return tx.Vout[input.Vout].Value, nil
}

// Check if transaction exists in a cache of unapproved
func (u *UnApprovedTransactions) GetIfExists(txid []byte) (*transaction.Transaction, error) {
	utdb, err := u.DB.GetUnapprovedTransactionsObject()

	if err != nil {
		return nil, err
	}

	txBytes, err := utdb.GetTransaction(txid)

	if err != nil {
		return nil, err
	}

	if len(txBytes) == 0 {
		return nil, nil
	}

	tx := transaction.Transaction{}
	err = tx.DeserializeTransaction(txBytes)

	if err != nil {
		return nil, err
	}

	return &tx, nil

}

/*
* Get all unapproved transactions
 */
func (u *UnApprovedTransactions) GetTransactions(number int) ([]*transaction.Transaction, error) {
	utdb, err := u.DB.GetUnapprovedTransactionsObject()

	if err != nil {
		return nil, err
	}
	txset := []*transaction.Transaction{}

	totalnumber := 0

	err = utdb.ForEach(func(k, txBytes []byte) error {
		tx := transaction.Transaction{}
		err = tx.DeserializeTransaction(txBytes)

		if err != nil {
			return err
		}

		txset = append(txset, &tx)
		totalnumber++

		if totalnumber >= number {
			// time to exit the loop. we don't need more
			return database.NewDBCursorStopError()
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// we need to sort transactions. oldest should be first
	sort.Sort(transaction.Transactions(txset))
	return txset, nil
}

// Get number of unapproved transactions in a cache

func (u *UnApprovedTransactions) GetCount() (int, error) {
	utdb, err := u.DB.GetUnapprovedTransactionsObject()

	if err != nil {
		return 0, err
	}

	return utdb.GetCount()
}

// Add new transaction for the list of unapproved
// Before to call this function we checked that transaction is valid
// Now we need to check if there are no conflicts with other transactions in the cache
func (u *UnApprovedTransactions) Add(txadd *transaction.Transaction) error {
	conflicts, err := u.DetectConflictsForNew(txadd)

	if err != nil {
		return err
	}

	if conflicts != nil {
		return errors.New(fmt.Sprintf("The transaction conflicts with other prepared transaction: %x", conflicts.ID))
	}

	utdb, err := u.DB.GetUnapprovedTransactionsObject()

	if err != nil {
		return err
	}

	u.Logger.Trace.Printf("adding TX to unappr %x", txadd.ID)

	txser, err := txadd.Serialize()

	if err != nil {
		return err
	}

	err = utdb.PutTransaction(txadd.ID, txser)

	if err != nil {
		return errors.New("Adding new transaction to unapproved cache: " + err.Error())
	}

	return nil
}

/*
* Delete transaction from a cache. When transaction becomes part ofa block
 */
func (u *UnApprovedTransactions) Delete(txid []byte) (bool, error) {
	utdb, err := u.DB.GetUnapprovedTransactionsObject()

	if err != nil {
		return false, err
	}

	txBytes, err := utdb.GetTransaction(txid)

	if err != nil {

		return false, err
	}

	if len(txBytes) > 0 {
		err = utdb.DeleteTransaction(txid)

		if err != nil {
			return false, err
		}
		return true, nil
	}

	return false, nil
}

/*
* Remove given blocks transavtions from unapproved . For case when list of blocks are added to primary blockchain branch
 */
func (u *UnApprovedTransactions) DeleteFromBlocks(blocks []*blockchain.Block) error {
	for _, block := range blocks {

		err := u.DeleteFromBlock(block)

		if err != nil {

			return err
		}
	}
	return nil
}

/*
* Remove all transactions from this cache listed in a block.
* Is used when new block added and transactions are approved now
 */
func (u *UnApprovedTransactions) DeleteFromBlock(block *blockchain.Block) error {
	// try to delete each transaction from this block
	u.Logger.Trace.Printf("UnApprTXs: remove on block add %x", block.Hash)

	for _, tx := range block.Transactions {
		if !tx.IsCoinbase() {
			u.Delete(tx.ID)
		}
	}

	return nil
}

// Is used for cases when it is needed to do something with all cached transactions.
// For example, to print them.

func (u *UnApprovedTransactions) IterateTransactions(callback UnApprovedTransactionsIteratorInterface) (int, error) {
	utdb, err := u.DB.GetUnapprovedTransactionsObject()

	if err != nil {
		return 0, err
	}

	total := 0

	err = utdb.ForEach(func(txID, txBytes []byte) error {
		tx := transaction.Transaction{}
		err = tx.DeserializeTransaction(txBytes)

		if err != nil {
			return err
		}
		callback(hex.EncodeToString(txID), tx.String())
		total++

		return nil
	})
	if err != nil {
		return 0, err
	}

	return total, nil
}

// Check if this new transaction conflicts with any other transaction in the cache
// It is not allowed 2 prepared transactions have same inputs
// we return first found transaction taht conflicts
func (u *UnApprovedTransactions) DetectConflictsForNew(txcheck *transaction.Transaction) (*transaction.Transaction, error) {
	// it i needed to go over all tranactions in cache and check each of them if input is same as in this tx
	utdb, err := u.DB.GetUnapprovedTransactionsObject()

	if err != nil {
		return nil, err
	}

	var txconflicts *transaction.Transaction

	err = utdb.ForEach(func(txID, txBytes []byte) error {
		txexi := transaction.Transaction{}
		err = txexi.DeserializeTransaction(txBytes)

		if err != nil {
			return err
		}

		conflicts := false

		for _, vin := range txcheck.Vin {
			for _, vine := range txexi.Vin {
				if bytes.Compare(vin.Txid, vine.Txid) == 0 && vin.Vout == vine.Vout {
					// this is same input transaction. it is conflict
					txconflicts = &txexi
					conflicts = true
					break
				}
			}
			if conflicts {
				break
			}
		}
		if conflicts {
			// return out of loop
			return database.NewDBCursorStopError()
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return txconflicts, nil
}

/*
* The function detects conflicts in unconfirmed transactions list
* This is for case when some transaction output was used for 2 or more transactions input
* For building of a block we should use only one of them.
* Transaction can be used more 1 time in a block. But each time must be differeent output index
 */
func (u *UnApprovedTransactions) DetectConflicts(txs []*transaction.Transaction) ([]*transaction.Transaction, []*transaction.Transaction, error) {
	goodtransactions := []*transaction.Transaction{}
	conflicts := []*transaction.Transaction{}

	usedoutputs := map[string][]int{}

	for _, tx := range txs {
		used := false

		for _, txi := range tx.Vin {
			txinhax := hex.EncodeToString(txi.Txid)

			// check if this input was already used
			if vouts, ok := usedoutputs[txinhax]; ok {
				for _, vout := range vouts {
					if vout == txi.Vout {
						// used by other transaction!
						used = true
						break
					}
				}

				if !used {
					// it was not yet used. add to the list
					usedoutputs[txinhax] = append(usedoutputs[txinhax], txi.Vout)
				}
			} else {
				// this transaction is not yet in the map. add it
				usedoutputs[txinhax] = []int{txi.Vout}
			}

			if used {
				// add to conflicting transactions. we will have to delete them
				conflicts = append(conflicts, tx)
				break
			}
		}

		if !used {
			goodtransactions = append(goodtransactions, tx)
		}
	}

	return goodtransactions, conflicts, nil
}

/*
* Many blocks canceled. Make their transactions to be unapproved.
* Blocks can be canceled when other branch of blockchain becomes primary
 */
func (u *UnApprovedTransactions) AddFromBlocksCancel(blocks []*blockchain.Block) error {
	for _, block := range blocks {

		err := u.AddFromCanceled(block.Transactions)

		if err != nil {
			return err
		}
	}
	return nil
}

/*
* Is used for case when a block canceled. all transactions from a block are back to unapproved cache
 */
func (u *UnApprovedTransactions) AddFromCanceled(txs []*transaction.Transaction) error {
	for _, tx := range txs {
		if !tx.IsCoinbase() {
			err := u.Add(tx)

			if err != nil {
				u.Logger.Trace.Printf("add tx %x error %s", tx.ID, err.Error())
			} else {
				u.Logger.Trace.Printf("add tx fine %x", tx.ID)
			}
		}
	}

	return nil

}
func (u *UnApprovedTransactions) CleanUnapprovedCache() error {

	u.Logger.Trace.Println("Clean Unapproved Transactions cache: Prepare")

	utdb, err := u.DB.GetUnapprovedTransactionsObject()

	if err != nil {
		return err
	}
	return utdb.TruncateDB()

}
