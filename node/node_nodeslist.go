package main

import (
	"os"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gelembjuk/democoin/lib/net"
)

const nodesFileName = "nodeslist.db"
const nodesBucket = "nodes"

type NodesListStorage struct {
	DataDir string
}

func (s NodesListStorage) GetNodes() ([]net.NodeAddr, error) {
	db, err := s.openDB()

	if err != nil {
		return nil, err
	}

	defer db.Close()

	nodes := []net.NodeAddr{}

	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(nodesBucket))

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			addr := string(v)
			node := net.NodeAddr{}
			node.LoadFromString(addr)

			nodes = append(nodes, node)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return nodes, nil
}
func (s NodesListStorage) AddNodeToKnown(addr net.NodeAddr) {
	db, err := s.openDB()

	if err != nil {
		return
	}

	defer db.Close()

	err = db.Update(func(txdb *bolt.Tx) error {
		b := txdb.Bucket([]byte(nodesBucket))

		addr := addr.NodeAddrToString()
		key := []byte(addr)

		return b.Put(key, key)
	})

}
func (s NodesListStorage) RemoveNodeFromKnown(addr net.NodeAddr) {
	db, err := s.openDB()

	if err != nil {
		return
	}

	defer db.Close()

	err = db.Update(func(txdb *bolt.Tx) error {
		b := txdb.Bucket([]byte(nodesBucket))

		addr := addr.NodeAddrToString()
		key := []byte(addr)

		return b.Delete(key)
	})
}
func (s NodesListStorage) GetCountOfKnownNodes() (int, error) {
	db, err := s.openDB()

	if err != nil {
		return 0, err
	}

	defer db.Close()

	count := 0

	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(nodesBucket))

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
func (s NodesListStorage) openDB() (*bolt.DB, error) {
	f, err := s.getDbFile()

	if err != nil {
		return nil, err
	}

	db, err := bolt.Open(f, 0600, &bolt.Options{Timeout: 10 * time.Second})

	if err != nil {
		return nil, err
	}

	return db, nil

}
func (s NodesListStorage) getDbFile() (string, error) {

	filePath := s.DataDir + nodesFileName

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// create empty DB

		db, err := bolt.Open(filePath, 0600, nil)

		if err != nil {
			return "", err
		}

		err = db.Update(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucket([]byte(nodesBucket))

			if err != nil {
				return err
			}

			return nil
		})

		db.Close()

		if err != nil {
			return "", err
		}

	}
	return filePath, nil
}
