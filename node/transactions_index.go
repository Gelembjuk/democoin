package main

import (
	"bytes"
	"encoding/gob"
	"log"

	"github.com/boltdb/bolt"
	"github.com/gelembjuk/democoin/lib"
)

type TransactionsIndex struct {
	Blockchain *Blockchain
	Logger     *lib.LoggerMan
}

type TransactionsIndexSpentOutputs struct {
	OutInd      int
	TXWhereUsed []byte
	InInd       int
	BlockHash   []byte
}

func (ti *TransactionsIndex) SetBlockchain(bc *Blockchain) {
	ti.Blockchain = bc
}

// Returns a bucket where we keep association of blocks and transactions

func (ti *TransactionsIndex) getIndexBucket(tx *bolt.Tx) *bolt.Bucket {
	tx.CreateBucketIfNotExists([]byte(TransactionsBucket))

	return tx.Bucket([]byte(TransactionsBucket))
}

// Returns a bucket where we keep association of blocks and transactions

func (ti *TransactionsIndex) getIndexOutputsBucket(tx *bolt.Tx) *bolt.Bucket {
	tx.CreateBucketIfNotExists([]byte(TransactionsOutputsBucket))

	return tx.Bucket([]byte(TransactionsOutputsBucket))
}

// Block added. We need to update index of transactions
func (ti *TransactionsIndex) BlockAdded(block *Block) error {
	db := ti.Blockchain.db

	err := db.Update(func(txDB *bolt.Tx) error {
		b := ti.getIndexBucket(txDB)
		bo := ti.getIndexOutputsBucket(txDB)

		for _, tx := range block.Transactions {
			b.Put(tx.ID, block.Hash)

			if tx.IsCoinbase() {
				continue
			}
			// for each input we save list of tranactions where iput was used
			for inInd, vin := range tx.Vin {

				// get existing ecordsfor this input
				to := bo.Get(vin.Txid)

				outs := []TransactionsIndexSpentOutputs{}

				if to != nil {

					var err error
					outs, err = ti.DeserializeOutputs(to)

					if err != nil {
						log.Panic(err)
						return err
					}
				}

				outs = append(outs, TransactionsIndexSpentOutputs{vin.Vout, tx.ID[:], inInd, block.Hash[:]})

				to, err := ti.SerializeOutputs(outs)

				if err != nil {
					return err
				}
				// by this ID we can know which output were already used and in which transactions

				bo.Put(vin.Txid, to)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (ti *TransactionsIndex) BlockRemoved(block *Block) error {
	db := ti.Blockchain.db

	err := db.Update(func(txDB *bolt.Tx) error {
		b := ti.getIndexBucket(txDB)
		bo := ti.getIndexOutputsBucket(txDB)

		for _, tx := range block.Transactions {
			b.Delete(tx.ID)

			if tx.IsCoinbase() {
				continue
			}

			// remove inputs from used outputs
			for _, vin := range tx.Vin {
				// get existing ecordsfor this input
				to := bo.Get(vin.Txid)

				if to == nil {
					continue
				}
				outs, err := ti.DeserializeOutputs(to)

				if err != nil {
					return err
				}
				newOoutputs := []TransactionsIndexSpentOutputs{}

				for _, o := range outs {
					if o.OutInd != vin.Vout {
						newOoutputs = append(newOoutputs, o)
					}
				}
				outs = newOoutputs[:]

				if len(outs) > 0 {
					to, err = ti.SerializeOutputs(outs)

					if err != nil {
						return err
					}
					// by this ID we can know which output were already used and in which transactions
					bo.Put(vin.Txid, to)
				} else {
					bo.Delete(vin.Txid)
				}

			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

// Reindex cach of trsnactions pointers to block
func (ti *TransactionsIndex) Reindex() error {

	db := ti.Blockchain.db

	err := db.Update(func(tx *bolt.Tx) error {
		bucketName := []byte(TransactionsBucket)

		err := tx.DeleteBucket(bucketName)
		if err != nil && err != bolt.ErrBucketNotFound {
			return err
		}

		_, err = tx.CreateBucket(bucketName)
		if err != nil {
			return err
		}

		bucketName = []byte(TransactionsOutputsBucket)

		err = tx.DeleteBucket(bucketName)
		if err != nil && err != bolt.ErrBucketNotFound {
			return err
		}

		_, err = tx.CreateBucket(bucketName)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}
	bci := ti.Blockchain.Iterator()

	for {
		block, err := bci.Next()

		if err != nil {

			return err
		}

		err = ti.BlockAdded(block)

		if err != nil {
			return err
		}

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}
	return nil
}

// Serialize. We need this to store data in DB in bytes

func (ti *TransactionsIndex) SerializeOutputs(outs []TransactionsIndexSpentOutputs) ([]byte, error) {
	var buff bytes.Buffer
	enc := gob.NewEncoder(&buff)
	err := enc.Encode(outs)
	if err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

/*
* Deserialize data from bytes loaded fom DB
 */
func (ti *TransactionsIndex) DeserializeOutputs(data []byte) ([]TransactionsIndexSpentOutputs, error) {
	var outputs []TransactionsIndexSpentOutputs

	dec := gob.NewDecoder(bytes.NewReader(data))
	err := dec.Decode(&outputs)
	if err != nil {
		return nil, err
	}

	return outputs, nil
}

func (ti *TransactionsIndex) GetTranactionBlock(txID []byte) ([]byte, error) {
	var blockHash []byte

	err := ti.Blockchain.db.View(func(txDB *bolt.Tx) error {
		b := ti.getIndexBucket(txDB)

		blockHash = b.Get(txID)

		return nil
	})
	if err != nil {
		return nil, err
	}
	return blockHash, nil
}

func (ti *TransactionsIndex) GetTranactionOutputsSpent(txID []byte) ([]TransactionsIndexSpentOutputs, error) {
	var res []TransactionsIndexSpentOutputs

	err := ti.Blockchain.db.View(func(txDB *bolt.Tx) error {
		bo := ti.getIndexOutputsBucket(txDB)

		to := bo.Get(txID)

		res = []TransactionsIndexSpentOutputs{}

		if to != nil {
			var err error
			res, err = ti.DeserializeOutputs(to)

			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}
