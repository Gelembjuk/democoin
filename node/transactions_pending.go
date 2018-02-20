package main

import (
	"encoding/hex"
	"errors"

	"github.com/boltdb/bolt"
	"github.com/gelembjuk/democoin/lib"
	"github.com/gelembjuk/democoin/lib/transaction"
)

type UnApprovedTransactions struct {
	Blockchain *Blockchain
	Logger     *lib.LoggerMan
}
type UnApprovedTransactionsIteratorInterface func(txhash, txstr string)

func (u *UnApprovedTransactions) SetBlockchain(bc *Blockchain) {
	u.Blockchain = bc
}

/*
* Returns a bucket where we keep unapproved transactions
 */
func (u *UnApprovedTransactions) getBucket(tx *bolt.Tx) *bolt.Bucket {

	tx.CreateBucketIfNotExists([]byte(transactionsBucket))

	return tx.Bucket([]byte(transactionsBucket))
}

/*
* Is called after blockchain DB creation. It must to create a bucket to keep unapproved tranactions
 */
func (u *UnApprovedTransactions) InitDB() {
	db := u.Blockchain.db

	db.Update(func(tx *bolt.Tx) error {
		u.getBucket(tx)
		return nil
	})
}

/*
* Check if transaction exists in a cache of unapproved
 */
func (u *UnApprovedTransactions) GetIfExists(txid []byte) (*transaction.Transaction, error) {
	db := u.Blockchain.db

	var txres *transaction.Transaction

	txres = nil

	err := db.View(func(tx *bolt.Tx) error {
		b := u.getBucket(tx)

		v := b.Get(txid)

		if v != nil {
			tx := transaction.DeserializeTransaction(v)
			txres = &tx
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return txres, nil
}

/*
* Get all unapproved transactions
 */
func (u *UnApprovedTransactions) GetTransactions(number int) ([]*transaction.Transaction, error) {
	db := u.Blockchain.db
	txset := []*transaction.Transaction{}

	err := db.View(func(tx *bolt.Tx) error {
		b := u.getBucket(tx)
		c := b.Cursor()

		totalnumber := 0

		for k, v := c.First(); k != nil && number > totalnumber; k, v = c.Next() {
			tx := transaction.DeserializeTransaction(v)
			txset = append(txset, &tx)
			totalnumber++
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return txset, nil
}

/*
* Get number of unapproved transactions in a cache
 */
func (u *UnApprovedTransactions) GetCount() (int, error) {
	db := u.Blockchain.db
	counter := 0

	err := db.View(func(tx *bolt.Tx) error {
		b := u.getBucket(tx)

		c := b.Cursor()

		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			counter++
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	return counter, nil
}

/*
* Add new transaction for the list of unapproved
 */
func (u *UnApprovedTransactions) Add(txadd *transaction.Transaction) error {
	db := u.Blockchain.db

	err := db.Update(func(tx *bolt.Tx) error {
		b := u.getBucket(tx)

		err := b.Put(txadd.ID, txadd.Serialize())

		if err != nil {
			return errors.New("Adding new transaction to unapproved cache: " + err.Error())
		}

		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

/*
* Delete transaction from a cache. When transaction becomes part ofa block
 */
func (u *UnApprovedTransactions) Delete(txid []byte) (bool, error) {
	db := u.Blockchain.db

	found := false

	err := db.Update(func(tx *bolt.Tx) error {
		b := u.getBucket(tx)

		v := b.Get(txid)

		if v != nil {
			found = true

			err := b.Delete(txid)

			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return false, err
	}
	return found, nil
}

/*
* Remove given blocks transavtions from unapproved . For case when list of blocks are added to primary blockchain branch
 */
func (u *UnApprovedTransactions) DeleteFromBlocks(blocks []*Block) error {
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
func (u *UnApprovedTransactions) DeleteFromBlock(block *Block) error {
	// try to delete each transaction from this block

	for _, tx := range block.Transactions {
		if !tx.IsCoinbase() {
			u.Delete(tx.ID)
		}
	}

	return nil
}

/*
* Is used for cases when it is needed to do something with all cached transactions.
* For example, to print them.
 */
func (u *UnApprovedTransactions) IterateTransactions(callback UnApprovedTransactionsIteratorInterface) (int, error) {
	db := u.Blockchain.db

	total := 0

	err := db.View(func(tx *bolt.Tx) error {
		b := u.getBucket(tx)

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			tx := transaction.DeserializeTransaction(v)
			callback(hex.EncodeToString(k), tx.String())
			total++
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	return total, nil
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
func (u *UnApprovedTransactions) AddFromBlocksCancel(blocks []*Block) error {
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
			u.Add(tx)
		}
	}

	return nil

}
