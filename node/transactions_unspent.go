package main

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"log"
	"sort"

	"github.com/boltdb/bolt"
	"github.com/gelembjuk/democoin/lib/transaction"
	"github.com/gelembjuk/democoin/lib/utils"
	"github.com/gelembjuk/democoin/lib/wallet"
)

// UnspentTransactions represents UTXO set
type UnspentTransactions struct {
	Blockchain *Blockchain //
	Logger     *utils.LoggerMan
}

/*
* Returns a DB bucket object where we store unspent outputs
 */
func (u *UnspentTransactions) getBucket(tx *bolt.Tx) *bolt.Bucket {
	// bucket is created when blockchain file inited. so, it must alway exist
	return tx.Bucket([]byte(UnspentTransactionsBucket))
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

// CGet input value. Input is unspent TX output
func (u UnspentTransactions) GetInputValue(input transaction.TXInput) (float64, error) {
	value := float64(0)

	err := u.Blockchain.db.View(func(tx *bolt.Tx) error {
		b := u.getBucket(tx)

		outsBytes := b.Get(input.Txid)

		if outsBytes == nil {
			return errors.New("Input TX is not found in unspent outputs")
		}
		outs := u.DeserializeOutputs(outsBytes)

		for _, o := range outs {
			if o.OIndex == input.Vout {
				value = o.Value
				break
			}
		}
		if value > 0 {
			return nil
		}
		return errors.New("Output index is not found in unspent outputs")
	})
	if err != nil {
		return 0, err
	}

	return value, nil
}

// Choose inputs for new transaction
func (u UnspentTransactions) ChooseSpendableOutputs(pubKeyHash []byte, amount float64,
	pendinguse []transaction.TXInput) (float64, []transaction.TXOutputIndependent, error) {

	unspentOutputs := []transaction.TXOutputIndependent{}
	accumulated := float64(0)

	db := u.Blockchain.db

	err := db.View(func(tx *bolt.Tx) error {
		b := u.getBucket(tx)
		c := b.Cursor()

		// get all possible outputs

		for k, v := c.First(); k != nil; k, v = c.Next() {
			outs := u.DeserializeOutputs(v)

			for _, out := range outs {
				if out.IsLockedWithKey(pubKeyHash) {
					// check if this output is not used in some pending transaction
					used := false
					for _, pin := range pendinguse {
						if bytes.Compare(pin.Txid, out.TXID) == 0 &&
							pin.Vout == out.OIndex {
							used = true
							break
						}
					}
					if used {
						continue
					}
					accumulated += out.Value
					unspentOutputs = append(unspentOutputs, out)
				}
			}
		}

		return nil
	})
	if err != nil {
		return 0, nil, err
	}

	if accumulated >= amount {
		// choose longest number of outputs to spent. it must be outs with smallest amounts
		sort.Sort(transaction.TXOutputIndependentList(unspentOutputs))

		accumulated = 0
		uo := []transaction.TXOutputIndependent{}

		for _, out := range unspentOutputs {

			accumulated += out.Value
			uo = append(uo, out)

			if accumulated >= amount {
				break
			}
		}

		unspentOutputs = uo
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
	pubKeyHash, err := utils.AddresToPubKeyHash(address)

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
	u.Logger.Trace.Println("Reindex UTXO: Prepare")
	db := u.Blockchain.db
	bucketName := []byte(UnspentTransactionsBucket)

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
	u.Logger.Trace.Println("Reindex UTXO: Prepare done")

	UTXO := u.FindUnspentTransactions()

	u.Logger.Trace.Println("Reindex UTXO: Store records")

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)

		for txID, outs := range UTXO {
			u.Logger.Trace.Printf("Reindex UTXO: Save %s %d", txID, len(outs))

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
	u.Logger.Trace.Println("Reindex UTXO: Done. Return counts")
	return u.CountUnspentOutputs()
}

/*
* Returns full list of unspent transactions outputs
* Iterates over full blockchain
* TODO this will not work for big blockchain. It keeps data in memory
 */
func (u UnspentTransactions) FindUnspentTransactions() map[string][]transaction.TXOutputIndependent {
	UTXO := make(map[string][]transaction.TXOutputIndependent)
	spentTXOs := make(map[string][]int)

	bci := u.Blockchain.Iterator()

	u.Logger.Trace.Println("Get All UTXO: Start")

	for {
		block, _ := bci.Next()

		for j := len(block.Transactions) - 1; j >= 0; j-- {
			tx := block.Transactions[j]
			txID := hex.EncodeToString(tx.ID)

			sender := []byte{}

			if tx.IsCoinbase() == false {
				sender, _ = utils.HashPubKey(tx.Vin[0].PubKey)
			}

			var spent bool

			for outIdx, out := range tx.Vout {
				// Was the output spent?
				spent = false

				if list, ok := spentTXOs[txID]; ok {

					for _, spentOutIdx := range list {

						if spentOutIdx == outIdx {
							// this output of the transaction was already spent
							// go to next output of this transaction
							spent = true
							break
						}
					}
				}
				if spent {
					continue
				}
				// add to unspent

				if _, ok := UTXO[txID]; !ok {
					UTXO[txID] = []transaction.TXOutputIndependent{}
				}
				outs := UTXO[txID]

				oute := transaction.TXOutputIndependent{}
				oute.LoadFromSimple(out, tx.ID, outIdx, sender, tx.IsCoinbase(), block.Hash)

				outs = append(outs, oute)
				UTXO[txID] = outs
			}

			if tx.IsCoinbase() {
				continue
			}
			for _, in := range tx.Vin {
				inTxID := hex.EncodeToString(in.Txid)

				if _, ok := spentTXOs[inTxID]; !ok {
					spentTXOs[inTxID] = []int{}
				}
				spentTXOs[inTxID] = append(spentTXOs[inTxID], in.Vout)

			}
		}

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}
	u.Logger.Trace.Printf("Get All UTXO: Return %d records", len(UTXO))
	return UTXO
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
	u.Logger.Trace.Printf("UPdate UTXO on block add %x", block.Hash)
	err := db.Update(func(txdb *bolt.Tx) error {
		b := u.getBucket(txdb)

		for _, tx := range block.Transactions {
			u.Logger.Trace.Printf("UpdateOnBlockAdd check tx %x", tx.ID)
			sender := []byte{}

			if !tx.IsCoinbase() {
				for _, vin := range tx.Vin {
					sender, _ = utils.HashPubKey(vin.PubKey)

					outsBytes := b.Get(vin.Txid)

					if outsBytes == nil {
						u.Logger.Trace.Printf("UpdateOnBlockAdd in tx is not found %x", vin.Txid)
						continue
					}

					outs := u.DeserializeOutputs(outsBytes)

					updatedOuts := []transaction.TXOutputIndependent{}

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
				no.LoadFromSimple(out, tx.ID, outInd, sender, tx.IsCoinbase(), block.Hash)
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

	err := db.Update(func(txdb *bolt.Tx) error {
		b := u.getBucket(txdb)

		for _, tx := range block.Transactions {
			u.Logger.Trace.Printf("tx %x", tx.ID) //REM
			if tx.IsCoinbase() == false {

				// all input outputs must be added back to unspent
				// but only if inputs are in current BC
				for _, vin := range tx.Vin {
					txi, spending, blockHash, err := u.Blockchain.FindTransaction(vin.Txid, []byte{})
					u.Logger.Trace.Printf("tx find input %x", vin.Txid) //REM
					if err != nil {
						u.Logger.Trace.Printf("error finding tx %x %s", tx.ID, err.Error()) //REM
						return err
					}

					if txi == nil {
						// TX is not found in current BC . no sense to add it to unspent
						u.Logger.Trace.Printf("tx not found in current BC") //REM
						break
					}

					u.Logger.Trace.Printf("found tx in block %x", blockHash) //REM

					sender, _ := utils.HashPubKey(txi.Vin[0].PubKey)

					UnspentOuts := []transaction.TXOutputIndependent{}

					for outInd, out := range txi.Vout {
						if _, ok := spending[outInd]; !ok {
							no := transaction.TXOutputIndependent{}
							no.LoadFromSimple(out, txi.ID, outInd, sender, tx.IsCoinbase(), blockHash)

							UnspentOuts = append(UnspentOuts, no)
						}
					}
					u.Logger.Trace.Printf("tx save as unspent %x %d outputs", vin.Txid, len(UnspentOuts))
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

// Find inputs for new transaction. Receives list of pending inputs used in other
// not yet confirmed transactions
// Returns list of inputs prepared. Even if less then requested
// Returns previous transactions. It later will be used to prepare data to sign
func (u UnspentTransactions) GetNewTransactionInputs(PubKey []byte, to string, amount float64,
	pendinguse []transaction.TXInput) ([]transaction.TXInput, map[string]transaction.Transaction, float64, error) {

	inputs := []transaction.TXInput{}

	pubKeyHash, _ := utils.HashPubKey(PubKey)
	totalamount, validOutputs, err := u.ChooseSpendableOutputs(pubKeyHash, amount, pendinguse)

	if err != nil {
		return inputs, nil, 0, err
	}

	// here we don't calculate is total amount is good or no.
	// later we will add unconfirmed transactions if no enough funds

	// build list of previous transactions
	prevTXs := make(map[string]transaction.Transaction)

	// Build a list of inputs
	for _, out := range validOutputs {
		input := transaction.TXInput{out.TXID, out.OIndex, nil, PubKey}
		inputs = append(inputs, input)

		prevTX, err := u.Blockchain.FindTransactionByBlock(out.TXID, out.BlockHash)

		if err != nil {
			return inputs, nil, 0, err
		}
		prevTXs[hex.EncodeToString(prevTX.ID)] = *prevTX
	}
	return inputs, prevTXs, totalamount, nil
}

// Returns previous transactions. It later will be used to prepare data to sign
func (u UnspentTransactions) ExtendNewTransactionInputs(PubKey []byte, amount, totalamount float64,
	inputs []transaction.TXInput, prevTXs map[string]transaction.Transaction,
	pendingoutputs []*transaction.TXOutputIndependent) ([]transaction.TXInput, map[string]transaction.Transaction, float64, error) {

	// Build a list of inputs
	for _, out := range pendingoutputs {
		input := transaction.TXInput{out.TXID, out.OIndex, nil, PubKey}
		inputs = append(inputs, input)

		prevTX := transaction.Transaction{}
		err := prevTX.DeserializeTransaction(out.BlockHash) // here we have transaction serialised, not block hash

		if err != nil {
			return inputs, prevTXs, totalamount, err
		}

		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX

		totalamount += out.Value

		if totalamount >= amount {
			break
		}
	}
	return inputs, prevTXs, totalamount, nil
}

// Verifies which transactions outputs are not yet spent.
// Returns list of inputs that are not found in list of unspent outputs
func (u UnspentTransactions) VerifyTransactionsOutputsAreNotSpent(txilist []transaction.TXInput) (map[int]transaction.TXInput, map[int]*transaction.Transaction, error) {
	// list of full input transactions. it can be used to verify signature later
	var inputTX map[int]*transaction.Transaction
	inputTX = make(map[int]*transaction.Transaction)

	var notFoundInputs map[int]transaction.TXInput
	notFoundInputs = make(map[int]transaction.TXInput)

	db := u.Blockchain.db

	err := db.View(func(tx *bolt.Tx) error {
		b := u.getBucket(tx)

		for txiInd, txi := range txilist {
			txdata := b.Get(txi.Txid)

			if txdata == nil {
				// not found
				inputTX[txiInd] = nil
				notFoundInputs[txiInd] = txi
				continue
			}
			exists := false
			blockHash := []byte{}

			outs := u.DeserializeOutputs(txdata)

			for _, out := range outs {
				if out.OIndex == txi.Vout {
					exists = true
					blockHash = out.BlockHash
					break
				}
			}

			if !exists {
				notFoundInputs[txiInd] = txi
				inputTX[txiInd] = nil
			} else {
				// find this TX and get full info about it
				prevTX, err := u.Blockchain.FindTransactionByBlock(txi.Txid, blockHash)

				if err != nil {
					return err
				}
				inputTX[txiInd] = prevTX
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return notFoundInputs, inputTX, nil
}
