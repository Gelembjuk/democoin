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
		// we don't use ForEach because we need a way to break
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			err := callback(k, v)

			if err, ok := err.(*DBError); ok {
				if err.IsKind(DBCursorBreak) {
					// the function wants to break the loop
					return nil
				}
			}

			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (bdb *BoltDB) getCountInBucket(bucket string) (int, error) {
	count := 0

	err := bdb.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}
		// we don't use ForEach because we need a way to break
		c := b.Cursor()

		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			count++
		}
		return nil
	})

	if err != nil {
		return 0, err
	}
	return count, nil
}
