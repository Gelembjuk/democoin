package database

import (
	"bytes"

	"github.com/boltdb/bolt"
	"github.com/gelembjuk/democoin/lib/utils"
)

const blocksBucket = "blocks"
const blockChainBucket = "blockchain"

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
		_, err = tx.CreateBucket([]byte(blockChainBucket))

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

// add block to chain
func (bc *Blockchain) AddToChain(hash, prevHash []byte) error {
	length := len(hash)

	if length == 0 {
		return NewHashEmptyDBError()
	}

	emptyHash := make([]byte, length)

	return bc.DB.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blockChainBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}

		// maybe it already exists in chain. check it
		// TODO . not sure if we need to do this check. ignore for now

		hashBytes := make([]byte, length*2)

		if len(prevHash) > 0 {
			// get prev hash and put this hash as next
			prRecTmp := b.Get(prevHash)

			if len(prRecTmp) < length*2 {
				return NewHashNotFoundDBError("Previous hash is not found in the chain")
			}

			prRec := utils.CopyBytes(prRecTmp)

			exNext := make([]byte, length)

			copy(exNext, prRec[length:])

			if bytes.Compare(exNext, emptyHash) > 0 {
				return NewHashDBError("Previous hash already has a next hash")
			}

			copy(prRec[length:], hash)

			err := b.Put(prevHash, prRec)

			if err != nil {
				return err
			}

			copy(hashBytes[0:], prevHash)
		}

		return b.Put(hash, hashBytes)
	})
}

// remove block from chain
func (bc *Blockchain) RemoveFromChain(hash []byte) error {
	length := len(hash)

	if length == 0 {
		return NewHashEmptyDBError()
	}

	emptyHash := make([]byte, length)

	return bc.DB.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blockChainBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}

		// get prev hash and put this hash as next
		hashBytesTmp := b.Get(hash)

		if len(hashBytesTmp) < length*2 {
			return NewHashNotFoundDBError(" ")
		}

		hashBytes := utils.CopyBytes(hashBytesTmp)

		nextHash := make([]byte, length)

		copy(nextHash, hashBytes[length:])

		if bytes.Compare(nextHash, emptyHash) > 0 {
			return NewHashDBError("Only last hash can be removed")
		}

		prevHash := make([]byte, length)
		copy(prevHash, hashBytes[0:length])

		if bytes.Compare(prevHash, emptyHash) > 0 {

			prevHashBytesTmp := b.Get(prevHash)

			if len(prevHashBytesTmp) < length*2 {
				return NewHashNotFoundDBError("Previous hash is not found")
			}
			prevHashBytes := utils.CopyBytes(prevHashBytesTmp)

			copy(prevHashBytes[length:], emptyHash)

			err := b.Put(prevHash, prevHashBytes)

			if err != nil {
				return err
			}

		}

		return b.Delete(hash)
	})
}

func (bc *Blockchain) BlockInChain(hash []byte) (bool, error) {
	length := len(hash)

	if length == 0 {
		return false, NewHashEmptyDBError()
	}

	found := false

	err := bc.DB.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blockChainBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}
		h := b.Get(hash)

		if len(h) == length*2 {
			found = true
		}

		return nil
	})
	if err != nil {
		return false, err
	}

	return found, nil
}

func (bc *Blockchain) GetLocationInChain(hash []byte) (bool, []byte, []byte, error) {
	length := len(hash)

	if length == 0 {
		return false, nil, nil, NewHashEmptyDBError()
	}
	var prevHash []byte
	var nextHash []byte

	emptyHash := make([]byte, length)

	found := false

	err := bc.DB.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blockChainBucket))

		if b == nil {
			return NewDBIsNotReadyError()
		}

		hTmp := b.Get(hash)

		if len(hTmp) == len(hash)*2 {
			h := utils.CopyBytes(hTmp)

			prevHash = make([]byte, length)
			nextHash = make([]byte, length)
			copy(prevHash, h[:length])
			copy(nextHash, h[length:])

			if bytes.Compare(prevHash, emptyHash) == 0 {
				prevHash = []byte{}
			}

			if bytes.Compare(nextHash, emptyHash) == 0 {
				nextHash = []byte{}
			}

			found = true
		}

		return nil
	})
	if err != nil {
		return false, nil, nil, err
	}

	return found, prevHash, nextHash, nil
}
