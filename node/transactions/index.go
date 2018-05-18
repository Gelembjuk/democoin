package transactions

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/gelembjuk/democoin/lib/utils"
	"github.com/gelembjuk/democoin/node/blockchain"
	"github.com/gelembjuk/democoin/node/database"
	"github.com/gelembjuk/democoin/node/transaction"
)

type TransactionsIndex struct {
	DB     database.DBManager
	Logger *utils.LoggerMan
}

type TransactionsIndexSpentOutputs struct {
	OutInd      int
	TXWhereUsed []byte
	InInd       int
	BlockHash   []byte
}

func NewTransactionIndex(DB database.DBManager, Logger *utils.LoggerMan) *TransactionsIndex {
	return &TransactionsIndex{DB, Logger}
}

func (tiso TransactionsIndexSpentOutputs) String() string {
	return fmt.Sprintf("OI %d used in %x II %d block %x", tiso.OutInd, tiso.TXWhereUsed, tiso.InInd, tiso.BlockHash)
}
func (ti *TransactionsIndex) BlocksAdded(blocks []*blockchain.Block) error {
	for _, block := range blocks {

		err := ti.BlockAdded(block)

		if err != nil {

			return err
		}
	}
	return nil
}

// Block added. We need to update index of transactions
func (ti *TransactionsIndex) BlockAdded(block *blockchain.Block) error {
	txdb, err := ti.DB.GetTransactionsObject()

	if err != nil {
		return err
	}

	for _, tx := range block.Transactions {
		err = txdb.PutTXToBlockLink(tx.ID, block.Hash)

		if err != nil {
			return err
		}

		if tx.IsCoinbase() {
			continue
		}
		// for each input we save list of tranactions where iput was used
		for inInd, vin := range tx.Vin {
			// get existing ecordsfor this input
			to, err := txdb.GetTXSpentOutputs(vin.Txid)

			if err != nil {
				return err
			}

			outs := []TransactionsIndexSpentOutputs{}

			if to != nil {

				var err error
				outs, err = ti.DeserializeOutputs(to)

				if err != nil {
					return err
				}
			}

			outs = append(outs, TransactionsIndexSpentOutputs{vin.Vout, tx.ID[:], inInd, block.Hash[:]})

			to, err = ti.SerializeOutputs(outs)

			if err != nil {
				return err
			}
			// by this ID we can know which output were already used and in which transactions
			err = txdb.PutTXSpentOutputs(vin.Txid, to)

			if err != nil {
				return err
			}
		}
	}
	return nil
}
func (ti *TransactionsIndex) BlocksRemoved(blocks []*blockchain.Block) error {
	for _, block := range blocks {

		err := ti.BlockRemoved(block)

		if err != nil {

			return err
		}
	}
	return nil
}
func (ti *TransactionsIndex) BlockRemoved(block *blockchain.Block) error {
	txdb, err := ti.DB.GetTransactionsObject()

	if err != nil {
		return err
	}

	for _, tx := range block.Transactions {
		txdb.DeleteTXToBlockLink(tx.ID)

		if tx.IsCoinbase() {
			continue
		}

		// remove inputs from used outputs
		for _, vin := range tx.Vin {
			// get existing ecordsfor this input
			to, err := txdb.GetTXSpentOutputs(vin.Txid)

			if err != nil {
				return err
			}

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
				txdb.PutTXSpentOutputs(vin.Txid, to)
			} else {
				txdb.DeleteTXSpentData(vin.Txid)
			}

		}
	}
	return nil
}

// Reindex cach of trsnactions pointers to block
func (ti *TransactionsIndex) Reindex() error {
	ti.Logger.Trace.Println("TXCache.Reindex: Prepare to recreate bucket")

	txdb, err := ti.DB.GetTransactionsObject()

	if err != nil {
		return err
	}

	err = txdb.TruncateDB()

	if err != nil {
		return err
	}

	ti.Logger.Trace.Println("TXCache.Reindex: Bucket created")

	bci, err := blockchain.NewBlockchainIterator(ti.DB)

	if err != nil {
		return err
	}

	for {
		block, err := bci.Next()

		if err != nil {

			return err
		}

		ti.Logger.Trace.Printf("TXCache.Reindex: Process block: %d, %x", block.Height, block.Hash)

		err = ti.BlockAdded(block)

		if err != nil {
			return err
		}

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}
	ti.Logger.Trace.Println("TXCache.Reindex: Done")
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
	txdb, err := ti.DB.GetTransactionsObject()

	if err != nil {
		return nil, err
	}

	blockHash, err := txdb.GetBlockHashForTX(txID)

	if err != nil {
		return nil, err
	}
	return blockHash, nil
}

func (ti *TransactionsIndex) GetTranactionOutputsSpent(txID []byte) ([]TransactionsIndexSpentOutputs, error) {
	txdb, err := ti.DB.GetTransactionsObject()

	if err != nil {
		return nil, err
	}

	to, err := txdb.GetTXSpentOutputs(txID)

	if err != nil {
		return nil, err
	}

	res := []TransactionsIndexSpentOutputs{}

	if to != nil {
		var err error
		res, err = ti.DeserializeOutputs(to)

		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

// Get full TX, spending status and block hash for TX by ID
func (ti *TransactionsIndex) GetTransactionAllInfo(txID []byte) (*transaction.Transaction, []TransactionsIndexSpentOutputs, []byte, error) {
	localError := func(err error) (*transaction.Transaction, []TransactionsIndexSpentOutputs, []byte, error) {
		return nil, nil, nil, err
	}

	blockHash, err := ti.GetTranactionBlock(txID)

	if err != nil {
		return localError(err)
	}

	if blockHash == nil {
		return nil, nil, nil, nil
	}

	spentOuts, err := ti.GetTranactionOutputsSpent(txID)

	if err != nil {
		return localError(err)
	}

	bcMan, err := blockchain.NewBlockchainManager(ti.DB, ti.Logger)

	if err != nil {
		return localError(err)
	}

	tx, err := bcMan.FindTransactionByBlock(txID, blockHash)

	if err != nil {
		return localError(err)
	}

	return tx, spentOuts, blockHash, nil
}
