package database

import (
	"github.com/boltdb/bolt"
)

type BoltDB struct {
	db       *bolt.DB
	lockFile string
}

func (bdb *BoltDB) Close() error {
	if bdb.db == nil {
		return nil
	}
	bdb.db.Close()
	bdb.db = nil

	return nil
}

func (bdb *BoltDB) forEachInBucket(bucket string, callback ForEachKeyIteratorInterface) error {
	return bdb.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			callback(k, v)
		}
		return nil
	})
}
