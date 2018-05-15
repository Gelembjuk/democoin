package database

import (
	"github.com/boltdb/bolt"
)

const transactionsBucket = "transactions"
const transactionsOutputsBucket = "transactionsoutputs"

type Tranactions struct {
	DB *BoltDB
}

// Init database
func (txs *Tranactions) InitDB() error {
	err := txs.DB.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte(transactionsBucket))

		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	err = txs.DB.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte(transactionsOutputsBucket))

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
func (txs *Tranactions) TruncateDB() error {
	err := txs.DB.db.Update(func(tx *bolt.Tx) error {
		err := tx.DeleteBucket([]byte(transactionsBucket))

		if err != nil && err != bolt.ErrBucketNotFound {
			return err
		}

		_, err = tx.CreateBucket([]byte(transactionsBucket))

		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	err = txs.DB.db.Update(func(tx *bolt.Tx) error {
		err := tx.DeleteBucket([]byte(transactionsOutputsBucket))

		if err != nil && err != bolt.ErrBucketNotFound {
			return err
		}

		_, err = tx.CreateBucket([]byte(transactionsOutputsBucket))

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

// Save link between TX and block hash
func (txs *Tranactions) PutTXToBlockLink(txID []byte, blockHash []byte) error {
	return txs.DB.db.Update(func(txDB *bolt.Tx) error {
		b := txDB.Bucket([]byte(transactionsBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}
		return b.Put(txID, blockHash)
	})
}

// Get block hash for TX
func (txs *Tranactions) GetBlockHashForTX(txID []byte) ([]byte, error) {
	var blockHash []byte

	err := txs.DB.db.View(func(txDB *bolt.Tx) error {
		b := txDB.Bucket([]byte(transactionsBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}

		blockHash = b.Get(txID)

		return nil
	})
	if err != nil {
		return nil, err
	}
	return blockHash, nil
}

// Delete link between TX and a block hash
func (txs *Tranactions) DeleteTXToBlockLink(txID []byte) error {
	return txs.DB.db.Update(func(txDB *bolt.Tx) error {
		b := txDB.Bucket([]byte(transactionsBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}
		return b.Delete(txID)
	})
}

// Save spent outputs for TX
func (txs *Tranactions) PutTXSpentOutputs(txID []byte, outputs []byte) error {
	return txs.DB.db.Update(func(txDB *bolt.Tx) error {
		b := txDB.Bucket([]byte(transactionsOutputsBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}
		return b.Put(txID, outputs)
	})
}

// Get spent outputs for TX , seialised to bytes
func (txs *Tranactions) GetTXSpentOutputs(txID []byte) ([]byte, error) {
	var outputsData []byte

	err := txs.DB.db.View(func(txDB *bolt.Tx) error {
		b := txDB.Bucket([]byte(transactionsOutputsBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}

		outputsData = b.Get(txID)

		return nil
	})
	if err != nil {
		return nil, err
	}
	return outputsData, nil
}

// Delete info about spent outputs for TX
func (txs *Tranactions) DeleteTXSpentData(txID []byte) error {
	return txs.DB.db.Update(func(txDB *bolt.Tx) error {
		b := txDB.Bucket([]byte(transactionsOutputsBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}
		return b.Delete(txID)
	})
}
