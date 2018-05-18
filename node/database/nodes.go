package database

import (
	"github.com/boltdb/bolt"
)

const nodesBucket = "nodes"

type Nodes struct {
	DB *BoltDB
}

func (ns *Nodes) InitDB() error {
	err := ns.DB.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte(nodesBucket))

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

// retrns nodes list iterator
func (ns *Nodes) ForEach(callback ForEachKeyIteratorInterface) error {
	return ns.DB.forEachInBucket(nodesBucket, callback)
}

// get count of records in the table
func (ns *Nodes) GetCount() (int, error) {
	return ns.DB.getCountInBucket(nodesBucket)
}

// Save node info
func (ns *Nodes) PutNode(nodeID []byte, nodeData []byte) error {
	return ns.DB.db.Update(func(txDB *bolt.Tx) error {
		b := txDB.Bucket([]byte(nodesBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}
		return b.Put(nodeID, nodeData)
	})
}

func (ns *Nodes) DeleteNode(nodeID []byte) error {
	return ns.DB.db.Update(func(txDB *bolt.Tx) error {
		b := txDB.Bucket([]byte(nodesBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}
		return b.Delete(nodeID)
	})
}
