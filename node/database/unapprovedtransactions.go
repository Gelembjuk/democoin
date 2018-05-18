package database

import (
	"github.com/boltdb/bolt"
)

const unapprovedTransactionsBucket = "unapprovedtransactions"

type UnapprovedTransactions struct {
	DB *BoltDB
}

func (uts *UnapprovedTransactions) InitDB() error {
	err := uts.DB.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte(unapprovedTransactionsBucket))

		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// execute functon for each key/value in the bucket
func (uts *UnapprovedTransactions) ForEach(callback ForEachKeyIteratorInterface) error {
	return uts.DB.forEachInBucket(unapprovedTransactionsBucket, callback)
}

// get count of records in the table
func (uts *UnapprovedTransactions) GetCount() (int, error) {
	return uts.DB.getCountInBucket(unapprovedTransactionsBucket)
}

func (uts *UnapprovedTransactions) TruncateDB() error {
	err := uts.DB.db.Update(func(tx *bolt.Tx) error {
		err := tx.DeleteBucket([]byte(unapprovedTransactionsBucket))

		if err != nil && err != bolt.ErrBucketNotFound {
			return err
		}

		_, err = tx.CreateBucket([]byte(unapprovedTransactionsBucket))

		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

// returns transaction by ID if it exists
func (uts *UnapprovedTransactions) GetTransaction(txID []byte) ([]byte, error) {
	var txBytes []byte

	err := uts.DB.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(unapprovedTransactionsBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}

		txBytes = b.Get(txID)

		return nil
	})
	if err != nil {
		return nil, err
	}
	return txBytes, nil
}

// Add transaction record
func (uts *UnapprovedTransactions) PutTransaction(txID []byte, txdata []byte) error {
	return uts.DB.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(unapprovedTransactionsBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}

		return b.Put(txID, txdata)
	})
}

// delete transation from DB
func (uts *UnapprovedTransactions) DeleteTransaction(txID []byte) error {
	return uts.DB.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(unapprovedTransactionsBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}

		return b.Delete(txID)
	})
}
