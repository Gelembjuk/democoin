package main

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"log"

	"github.com/boltdb/bolt"
	"github.com/gelembjuk/democoin/lib"
	"github.com/gelembjuk/democoin/lib/transaction"
	"github.com/gelembjuk/democoin/lib/wallet"
)

// UnspentTransactions represents UTXO set
type UnspentTransactions struct {
	Blockchain *Blockchain //
	Logger     *lib.LoggerMan
}

/*
* Returns a DB bucket object where we store unspent outputs
 */
func (u *UnspentTransactions) getBucket(tx *bolt.Tx) *bolt.Bucket {
	// bucket is created when blockchain file inited. so, it must alway exist
	return tx.Bucket([]byte(utxoBucket))
}

func (u *UnspentTransactions) SetBlockchain(bc *Blockchain) {
	u.Blockchain = bc
}

/*
*Serialize. We need this to store data in DB in bytes
 */
func (u UnspentTransactions) SerializeOutputs(outs []transaction.TXOutputIndependent) []byte {
	var buff bytes.Buffer
	enc := gob.NewEncoder(&buff)
	err := enc.Encode(outs)
	if err != nil {
		log.Panic(err)
	}

	return buff.Bytes()
}

/*
* Deserialize data from bytes loaded fom DB
 */
func (u UnspentTransactions) DeserializeOutputs(data []byte) []transaction.TXOutputIndependent {
	var outputs []transaction.TXOutputIndependent

	dec := gob.NewDecoder(bytes.NewReader(data))
	err := dec.Decode(&outputs)
	if err != nil {
		log.Panic(err)
	}

	return outputs
}

/*
* Calculates address balance using the cache of unspent transactions outputs
 */
func (u UnspentTransactions) GetAddressBalance(address string) (float64, error) {
	if address == "" {
		return 0, errors.New("Address is missed")
	}
	w := wallet.Wallet{}

	if !w.ValidateAddress(address) {
		return 0, errors.New("Address is not valid")
	}

	balance := float64(0)

	UnspentTXs, err2 := u.GetUnspentTransactionsOutputs(address)

	if err2 != nil {
		return 0, err2
	}

	for _, out := range UnspentTXs {
		balance += out.Value
	}
	return balance, nil
}

/*
* Choose inputs for new transaction
 */
func (u UnspentTransactions) ChooseSpendableOutputs(pubKeyHash []byte, amount float64) (float64, map[string][]int, error) {
	unspentOutputs := make(map[string][]int)
	accumulated := float64(0)

	db := u.Blockchain.db

	err := db.View(func(tx *bolt.Tx) error {
		b := u.getBucket(tx)
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			txID := hex.EncodeToString(k)
			outs := u.DeserializeOutputs(v)

			for outIdx, out := range outs {
				if out.IsLockedWithKey(pubKeyHash) {
					accumulated += out.Value
					unspentOutputs[txID] = append(unspentOutputs[txID], outIdx)

					if accumulated >= amount {
						// found enough transactions. stop the process and return
						return nil
					}
				}
			}
		}

		return nil
	})
	if err != nil {
		return 0, nil, err
	}

	return accumulated, unspentOutputs, nil
}

/*
* Returns list of unspent transactions outputs for address
 */
func (u UnspentTransactions) GetUnspentTransactionsOutputs(address string) ([]transaction.TXOutputIndependent, error) {
	if address == "" {
		return nil, errors.New("Address is missed")
	}
	w := wallet.Wallet{}

	if !w.ValidateAddress(address) {
		return nil, errors.New("Address is not valid")
	}
	pubKeyHash, err := lib.AddresToPubKeyHash(address)

	if err != nil {
		return nil, err
	}

	UTXOs := []transaction.TXOutputIndependent{}

	db := u.Blockchain.db

	err = db.View(func(tx *bolt.Tx) error {
		b := u.getBucket(tx)

		if b == nil {
			u.Logger.Trace.Printf("Bucket object bot found")
			return nil
		}

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			outs := u.DeserializeOutputs(v)

			for _, out := range outs {
				if out.IsLockedWithKey(pubKeyHash) {
					UTXOs = append(UTXOs, out)
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return UTXOs, nil
}

/*
* Returns total number of unspent transactions in a cache.
 */
func (u UnspentTransactions) CountTransactions() (int, error) {
	db := u.Blockchain.db
	counter := 0

	err := db.View(func(tx *bolt.Tx) error {
		b := u.getBucket(tx)
		c := b.Cursor()

		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			counter++
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	return counter, nil
}

/*
* Returns toal number of transactions outputs in a cache
 */
func (u UnspentTransactions) CountUnspentOutputs() (int, error) {
	db := u.Blockchain.db
	counter := 0

	err := db.View(func(tx *bolt.Tx) error {
		b := u.getBucket(tx)
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			outs := u.DeserializeOutputs(v)
			counter += len(outs)
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	return counter, nil
}

/*
* Rebuilds the DB of unspent transactions
 */
func (u UnspentTransactions) Reindex() (int, error) {
	db := u.Blockchain.db
	bucketName := []byte(utxoBucket)

	err := db.Update(func(tx *bolt.Tx) error {
		err := tx.DeleteBucket(bucketName)
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
		return 0, err
	}

	UTXO := u.Blockchain.FindUnspentTransactions()

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)

		for txID, outs := range UTXO {

			key, err := hex.DecodeString(txID)
			if err != nil {
				return err
			}

			err = b.Put(key, u.SerializeOutputs(outs))
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	return u.CountUnspentOutputs()
}

/*
* set of blocks added to block chain. we need to mark transactions unspent and spent
 */
func (u UnspentTransactions) UpdateOnBlocksAdd(blocks []*Block) error {
	for _, block := range blocks {
		err := u.UpdateOnBlockAdd(block)

		if err != nil {
			return err
		}
	}

	return nil
}

/*
* New Block added
* Input of all tranactions are removed from unspent
* OUtput of all transactions are added to unspent
* Update the UTXO set with transactions from the Block
* The Block is considered to be the tip of a blockchain
 */
func (u UnspentTransactions) UpdateOnBlockAdd(block *Block) error {
	db := u.Blockchain.db

	err := db.Update(func(tx *bolt.Tx) error {
		b := u.getBucket(tx)

		for _, tx := range block.Transactions {

			sender := []byte{}

			if !tx.IsCoinbase() {
				for _, vin := range tx.Vin {
					sender, _ = lib.HashPubKey(vin.PubKey)

					updatedOuts := []transaction.TXOutputIndependent{}

					outsBytes := b.Get(vin.Txid)
					outs := u.DeserializeOutputs(outsBytes)

					for _, out := range outs {
						if out.OIndex != vin.Vout {
							updatedOuts = append(updatedOuts, out)
						}
					}

					if len(updatedOuts) == 0 {
						err := b.Delete(vin.Txid)
						if err != nil {
							return err
						}
					} else {
						err := b.Put(vin.Txid, u.SerializeOutputs(updatedOuts))
						if err != nil {
							return err
						}
					}

				}
			}
			newOutputs := []transaction.TXOutputIndependent{}

			for outInd, out := range tx.Vout {
				no := transaction.TXOutputIndependent{}
				no.LoadFromSimple(out, tx.ID, outInd, sender, tx.IsCoinbase())
				newOutputs = append(newOutputs, no)
			}

			err := b.Put(tx.ID, u.SerializeOutputs(newOutputs))

			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

/*
* Is called for a case when a list of block is canceled. Usually, when new chanin branch appears
* and replaces part of blocks on a top
 */
func (u UnspentTransactions) UpdateOnBlocksCancel(blocks []*Block) error {
	for _, block := range blocks {
		err := u.UpdateOnBlockCancel(block)

		if err != nil {
			return err
		}
	}

	return nil
}

/*
* This is executed when a block is canceled.
* All input transactions must be return to "unspent"
* And all outpt must be deleted from "unspent"
 */
func (u UnspentTransactions) UpdateOnBlockCancel(block *Block) error {
	db := u.Blockchain.db

	err := db.Update(func(tx *bolt.Tx) error {
		b := u.getBucket(tx)

		for _, tx := range block.Transactions {
			if tx.IsCoinbase() == false {

				// all input outputs must be added back to unspent
				for _, vin := range tx.Vin {
					txi, spending, err := u.Blockchain.FindTransaction(vin.Txid)

					sender, _ := lib.HashPubKey(txi.Vin[0].PubKey)

					UnspentOuts := []transaction.TXOutputIndependent{}

					for outInd, out := range txi.Vout {
						if _, ok := spending[outInd]; !ok {
							no := transaction.TXOutputIndependent{}
							no.LoadFromSimple(out, txi.ID, outInd, sender, tx.IsCoinbase())
							UnspentOuts = append(UnspentOuts, no)
						}
					}

					err = b.Put(vin.Txid, u.SerializeOutputs(UnspentOuts))

					if err != nil {
						return err
					}

				}
			}
			// delete this transaction from list of unspent
			b.Delete(tx.ID)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

/*
* Create New Transaction Object with empty signature and returns also data to sign
 */
func (u UnspentTransactions) NewTransaction(PubKey []byte, to string, amount float64) (*transaction.Transaction, [][]byte, error) {
	var inputs []transaction.TXInput
	var outputs []transaction.TXOutput

	pubKeyHash, _ := lib.HashPubKey(PubKey)
	acc, validOutputs, err := u.ChooseSpendableOutputs(pubKeyHash, amount)

	if err != nil {
		return nil, nil, err
	}

	if acc < amount {
		return nil, nil, errors.New("Not enough funds")
	}

	// Build a list of inputs
	for txid, outs := range validOutputs {
		txID, err := hex.DecodeString(txid)
		if err != nil {
			return nil, nil, err
		}

		for _, out := range outs {
			input := transaction.TXInput{txID, out, nil, PubKey}
			inputs = append(inputs, input)
		}
	}

	// Build a list of outputs
	from, _ := lib.PubKeyToAddres(PubKey)
	outputs = append(outputs, *transaction.NewTXOutput(amount, to))

	if acc > amount {
		outputs = append(outputs, *transaction.NewTXOutput(acc-amount, from)) // a change
	}

	tx := transaction.Transaction{nil, inputs, outputs}
	tx.ID = tx.Hash()

	// get list of previous transactions
	prevTXs := make(map[string]transaction.Transaction)

	for _, vin := range tx.Vin {
		prevTX, _, err := u.Blockchain.FindTransaction(vin.Txid)

		if err != nil {
			return nil, nil, err
		}
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	signdata, err := tx.PrepareSignData(prevTXs)

	if err != nil {
		return nil, nil, err
	}

	return &tx, signdata, nil
}

/*
* Verifies which transactions outputs are not yet spent.
* Returns list of inputs that are not found in list of unspent outputs
 */
func (u UnspentTransactions) VerifyTransactionsOutputsAreNotSpent(txilist []transaction.TXInput) ([]*transaction.TXInput, error) {
	notFoundInputs := []*transaction.TXInput{}

	db := u.Blockchain.db

	err := db.View(func(tx *bolt.Tx) error {
		b := u.getBucket(tx)

		for _, txi := range txilist {
			txdata := b.Get(txi.Txid)

			if txdata == nil {
				// not found
				notFoundInputs = append(notFoundInputs, &txi)
				continue
			}
			exists := false

			outs := u.DeserializeOutputs(txdata)

			for _, out := range outs {
				if out.OIndex == txi.Vout {
					exists = true
					break
				}
			}

			if !exists {
				notFoundInputs = append(notFoundInputs, &txi)
			}

		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return notFoundInputs, nil
}
