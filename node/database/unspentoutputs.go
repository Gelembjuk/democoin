package database

import (
	"github.com/boltdb/bolt"
)

const unspentTransactionsBucket = "unspentoutputstransactions"

type UnspentOutputs struct {
	DB *BoltDB
}

func (uos *UnspentOutputs) InitDB() error {
	return uos.DB.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte(unspentTransactionsBucket))
		return err
	})
}

// retrns BC iterator
func (uos *UnspentOutputs) GetCursor() (CursorInterface, error) {
	i := &Cursor{unspentTransactionsBucket, uos.DB, nil, nil}

	return i, nil
}

func (uos *UnspentOutputs) TruncateDB() error {
	return uos.DB.db.Update(func(tx *bolt.Tx) error {
		err := tx.DeleteBucket([]byte(unspentTransactionsBucket))

		if err != nil {
			return err
		}
		_, err = tx.CreateBucket([]byte(unspentTransactionsBucket))

		return err
	})
}

func (uos *UnspentOutputs) GetDataForTransaction(txID []byte) ([]byte, error) {
	var txData []byte

	err := uos.DB.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(unspentTransactionsBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}

		txData = b.Get(txID)

		return nil
	})
	if err != nil {
		return nil, err
	}
	return txData, nil
}

func (uos *UnspentOutputs) DeleteDataForTransaction(txID []byte) error {
	return uos.DB.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(unspentTransactionsBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}

		return b.Delete(txID)
	})
}
func (uos *UnspentOutputs) PutDataForTransaction(txID []byte, txData []byte) error {
	return uos.DB.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(unspentTransactionsBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}

		return b.Put(txID, txData)
	})
}
