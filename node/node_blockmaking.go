package main

import (
	"errors"

	"github.com/gelembjuk/democoin/lib"
	"github.com/gelembjuk/democoin/lib/transaction"
)

type NodeBlockMaker struct {
	Logger        *lib.LoggerMan
	BC            *Blockchain
	UnspentTXs    *UnspentTransactions
	UnapprovedTXs *UnApprovedTransactions
	MinterAddress string // this is the wallet that will receive for mining
}

/*
* Checks if this is good time for this node to make a block
* In this version it is always true
 */
func (n *NodeBlockMaker) CheckGoodTimeToMakeBlock() bool {
	return true
}

/*
* Check if there are abough unapproved transactions to make a block
 */
func (n *NodeBlockMaker) CheckUnapprovedCache() bool {
	count, err := n.UnapprovedTXs.GetCount()

	if err != nil {
		n.Logger.Trace.Printf("Error when check unapproved cache: %s", err.Error())
		return false
	}

	n.Logger.Trace.Printf("Transaction in cache - %d", count)

	if count >= minNumberTransactionInBlock {
		if count > maxNumberTransactionInBlock {
			count = maxNumberTransactionInBlock
		}
		return true
	}
	return false
}

/*
* Makes new block, without a hash. Only finds transactions to add to a block
 */
func (n *NodeBlockMaker) PrepareNewBlock() (*Block, error) {
	// firstly, check count of transactions to know if there are enough
	count, err := n.UnapprovedTXs.GetCount()

	if err != nil {
		return nil, err
	}

	n.Logger.Trace.Printf("Minting: Found %d transaction from minimum %d\n", count, minNumberTransactionInBlock)

	if count >= minNumberTransactionInBlock {
		// number of transactions is fine
		if count > maxNumberTransactionInBlock {
			count = maxNumberTransactionInBlock
		}
		// get unapproved transactions
		txlist, err := n.UnapprovedTXs.GetTransactions(count)

		if err != nil {
			return nil, err
		}

		n.Logger.Trace.Printf("Minting: Found %d transaction to mine\n", len(txlist))

		var txs []*transaction.Transaction

		for id := range txlist {
			tx := txlist[id]

			// we need to verify each transaction
			// we can not allow
			vtx, err := n.BC.VerifyTransaction(tx)

			if err != nil {
				return nil, err
			}

			if vtx {
				// transaction is valid
				txs = append(txs, tx)
			} else {
				// the transaction is invalid. some input was already used in other confirmed transaction
				// or somethign wrong with signatures.
				// remove this transaction from the DB of unconfirmed transactions
				n.Logger.Trace.Printf("Minting: Delete transaction used in other block before: %x\n", tx.ID)
				n.UnapprovedTXs.Delete(tx.ID)
			}
		}
		txlist = nil

		n.Logger.Trace.Printf("Minting: After verification %d transaction are left\n", len(txs))

		if len(txs) == 0 {
			return nil, errors.New("All transactions are invalid! Waiting for new ones...")
		}
		// check if total count is still good
		if len(txs) < minNumberTransactionInBlock {
			return nil, errors.New("No enought valid transactions! Waiting for new ones...")
		}

		// there is anough of "good" transactions. where inputs were not yet used in other confirmed transactions
		// now it is needed to check if transactions don't conflict one to other
		var badtransactions []*transaction.Transaction
		txs, badtransactions, err = n.UnapprovedTXs.DetectConflicts(txs)

		n.Logger.Trace.Printf("Minting: After conflict detection %d - fine, %d - conflicts\n", len(txs), len(badtransactions))

		if err != nil {
			return nil, err
		}

		if len(badtransactions) > 0 {
			// there are conflicts! remove conflicting transactions
			for _, tx := range badtransactions {
				n.Logger.Trace.Printf("Delete conflicting transaction: %x\n", tx.ID)
				n.UnapprovedTXs.Delete(tx.ID)
			}
		}

		if len(txs) < minNumberTransactionInBlock {
			return nil, errors.New("No enought valid transactions! Waiting for new ones...")
		}

		n.Logger.Trace.Printf("Minting: All good. New block assigned to address %s\n", n.MinterAddress)

		newBlock, err := n.makeNewBlockFromTransactions(txs)

		if err != nil {
			return nil, err
		}

		n.Logger.Trace.Printf("Minting: New block Prepared. Not yet complete\n")

		return newBlock, nil
	}

	return nil, nil
}

// finalise a block. in this place we do MIMING
func (n *NodeBlockMaker) CompleteBlock(b *Block) error {
	// NOTE
	// We don't check if transactions are valid in this place .
	// we did checks before in the calling function
	// we checked each tranaction if it has correct signature,
	// it inputs  are not yet stent before
	// if there is no 2 transaction with same input in one block

	n.Logger.Trace.Printf("Minting: Start proof of work for the block\n")

	pow := NewProofOfWork(b)
	nonce, hash := pow.Run()

	b.Hash = hash[:]
	b.Nonce = nonce

	n.Logger.Trace.Printf("Minting: New hash is %x\n", b.Hash)

	return nil
}

// this builds a block object from given transactions list
// adds coinbase transacion (prize for miner)
func (n *NodeBlockMaker) makeNewBlockFromTransactions(transactions []*transaction.Transaction) (*Block, error) {
	// get last block info
	lastHash, lastHeight, err := n.BC.GetState()

	if err != nil {
		return nil, err
	}

	// add transaction - prize for miner
	cbTx := &transaction.Transaction{}

	errc := cbTx.MakeCoinbaseTX(n.MinterAddress, "")

	if errc != nil {
		return nil, errc
	}

	transactions = append(transactions, cbTx)

	newblock := Block{}
	err = newblock.PrepareNewBlock(transactions, lastHash[:], lastHeight+1)

	if err != nil {
		return nil, err
	}

	return &newblock, nil
}
