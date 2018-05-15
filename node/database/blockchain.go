package database

import (
	"github.com/boltdb/bolt"
	"github.com/gelembjuk/democoin/lib/utils"
)

const blocksBucket = "blocks"

type Blockchain struct {
	DB *BoltDB
}

// create bucket etc. DB is already inited
func (bc *Blockchain) InitDB() error {
	err := bc.DB.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte(blocksBucket))

		if err != nil {
			return err
		}
		return nil
	})
	return err
}

// Get block on the top of blockchain
func (bc *Blockchain) GetTopBlock() ([]byte, error) {
	topHash, err := bc.GetTopHash()

	if err != nil {
		return nil, err
	}

	return bc.GetBlock(topHash)
}

// Get block data by hash. It returns just []byte and and must be deserialised on ther place
func (bc *Blockchain) GetBlock(hash []byte) ([]byte, error) {
	var blockData []byte

	err := bc.DB.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}

		blockData = b.Get(hash)

		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(blockData) > 0 {
		blockData = utils.CopyBytes(blockData)
	}

	return blockData, nil
}

//  Check if block exists by hash
func (bc *Blockchain) CheckBlockExists(hash []byte) (bool, error) {
	// we just load this bloc data from db .
	blockData, err := bc.GetBlock(hash)

	if err != nil {
		return false, err
	}

	if len(blockData) > 0 {
		return true, nil
	}
	return false, nil
}

// Add block to the top of block chain
func (bc *Blockchain) PutBlockOnTop(hash []byte, blockdata []byte) error {
	err := bc.PutBlock(hash, blockdata)

	if err != nil {
		return err
	}

	return bc.SaveTopHash(hash)
}

// Add block record
func (bc *Blockchain) PutBlock(hash []byte, blockdata []byte) error {
	err := bc.DB.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}

		return b.Put(hash, blockdata)
	})
	return err
}

// Delete block record
func (bc *Blockchain) DeleteBlock(hash []byte) error {
	err := bc.DB.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}

		return b.Delete(hash)
	})
	return err
}

// Save top level block hash
func (bc *Blockchain) SaveTopHash(hash []byte) error {
	err := bc.DB.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}
		return b.Put([]byte("l"), hash)

		return nil
	})
	return err
}

// Get top level block hash
func (bc *Blockchain) GetTopHash() ([]byte, error) {
	var topHash []byte

	err := bc.DB.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}
		topHash = b.Get([]byte("l"))

		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(topHash) > 0 {
		topHash = utils.CopyBytes(topHash)

		return topHash, nil
	}

	return nil, NewNotFoundDBError("tophash")
}

// Save first (or genesis) block hash. It should be called when blockchain is created
func (bc *Blockchain) SaveFirstHash(hash []byte) error {
	err := bc.DB.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}
		return b.Put([]byte("f"), hash)
	})
	return err
}

func (bc *Blockchain) GetFirstHash() ([]byte, error) {
	var firstHash []byte

	err := bc.DB.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}
		firstHash = b.Get([]byte("f"))

		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(firstHash) > 0 {
		firstHash = utils.CopyBytes(firstHash)

		return firstHash, nil
	}

	return nil, NewNotFoundDBError("firsthash")
}
