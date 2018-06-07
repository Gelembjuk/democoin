package consensus

import (
	"bytes"
	"errors"
	"time"

	"github.com/gelembjuk/democoin/lib/utils"
	"github.com/gelembjuk/democoin/node/blockchain"
	"github.com/gelembjuk/democoin/node/config"
	"github.com/gelembjuk/democoin/node/database"
	"github.com/gelembjuk/democoin/node/structures"
	"github.com/gelembjuk/democoin/node/transactions"
)

type NodeBlockMaker struct {
	DB            database.DBManager
	Logger        *utils.LoggerMan
	MinterAddress string // this is the wallet that will receive for mining
}

func (n *NodeBlockMaker) SetDBManager(DB database.DBManager) {
	n.DB = DB
}
func (n *NodeBlockMaker) getTransactionsManager() *transactions.Manager {
	return transactions.NewManager(n.DB, n.Logger)
}

func (n *NodeBlockMaker) getBlockchainManager() *blockchain.Blockchain {
	bcm, _ := blockchain.NewBlockchainManager(n.DB, n.Logger)

	return bcm
}

func (n *NodeBlockMaker) GetUnapprovedTransactionsManager() *transactions.UnApprovedTransactions {
	return n.getTransactionsManager().GetUnapprovedTransactionsManager()
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
	count, err := n.GetUnapprovedTransactionsManager().GetCount()

	if err != nil {
		n.Logger.Trace.Printf("Error when check unapproved cache: %s", err.Error())
		return false
	}

	n.Logger.Trace.Printf("Transaction in cache - %d", count)

	min, max, err := n.getBlockchainManager().GetTransactionNumbersLimits(nil)

	if count >= min {
		if count > max {
			count = max
		}
		return true
	}
	return false
}

/*
* Makes new block, without a hash. Only finds transactions to add to a block
 */
func (n *NodeBlockMaker) PrepareNewBlock() (*structures.Block, error) {
	// firstly, check count of transactions to know if there are enough
	count, err := n.GetUnapprovedTransactionsManager().GetCount()

	if err != nil {
		return nil, err
	}
	min, max, err := n.getBlockchainManager().GetTransactionNumbersLimits(nil)

	if err != nil {
		return nil, err
	}

	n.Logger.Trace.Printf("Minting: Found %d transaction from minimum %d\n", count, min)

	if count >= min {
		// number of transactions is fine
		if count > max {
			count = max
		}
		// get unapproved transactions
		txlist, err := n.GetUnapprovedTransactionsManager().GetTransactions(count)

		if err != nil {
			return nil, err
		}

		n.Logger.Trace.Printf("Minting: Found %d transaction to mine\n", len(txlist))

		txs := []*structures.Transaction{}

		for _, tx := range txlist {
			n.Logger.Trace.Printf("Minting: Go to verify: %x\n", tx.ID)

			// we need to verify each transaction
			// we will do full deep check of transaction
			// also, a transaction can have input from other transaction from thi block
			vtx, err := n.getTransactionsManager().VerifyTransactionDeep(tx, txs, []byte{})

			if err != nil {
				// this can be case when a transaction is based on other unapproved transaction
				// and that transaction was created in same second
				n.Logger.Trace.Printf("Minting: Ignore transaction %x. Verify failed with error: %s\n", tx.ID, err.Error())
				// we delete this transaction. no sense to keep it
				n.GetUnapprovedTransactionsManager().Delete(tx.ID)
				continue
			}

			if vtx {
				// transaction is valid
				txs = append(txs, tx)
			} else {
				// the transaction is invalid. some input was already used in other confirmed transaction
				// or somethign wrong with signatures.
				// remove this transaction from the DB of unconfirmed transactions
				n.Logger.Trace.Printf("Minting: Delete transaction used in other block before: %x\n", tx.ID)
				n.GetUnapprovedTransactionsManager().Delete(tx.ID)
			}
		}
		txlist = nil

		n.Logger.Trace.Printf("Minting: After verification %d transaction are left\n", len(txs))

		if len(txs) == 0 {
			return nil, errors.New("All transactions are invalid! Waiting for new ones...")
		}
		// check if total count is still good
		if len(txs) < min {
			return nil, errors.New("No enought valid transactions! Waiting for new ones...")
		}

		// there is anough of "good" transactions. where inputs were not yet used in other confirmed transactions
		// now it is needed to check if transactions don't conflict one to other
		var badtransactions []*structures.Transaction
		txs, badtransactions, err = n.GetUnapprovedTransactionsManager().DetectConflicts(txs)

		n.Logger.Trace.Printf("Minting: After conflict detection %d - fine, %d - conflicts\n", len(txs), len(badtransactions))

		if err != nil {
			return nil, err
		}

		if len(badtransactions) > 0 {
			// there are conflicts! remove conflicting transactions
			for _, tx := range badtransactions {
				n.Logger.Trace.Printf("Delete conflicting transaction: %x\n", tx.ID)
				n.GetUnapprovedTransactionsManager().Delete(tx.ID)
			}
		}

		if len(txs) < min {
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
func (n *NodeBlockMaker) CompleteBlock(b *structures.Block) error {
	// NOTE
	// We don't check if transactions are valid in this place .
	// we did checks before in the calling function
	// we checked each tranaction if it has correct signature,
	// it inputs  are not yet stent before
	// if there is no 2 transaction with same input in one block

	n.Logger.Trace.Printf("Minting: Start proof of work for the block\n")

	starttime := time.Now()

	pow := NewProofOfWork(b)

	nonce, hash, err := pow.Run()

	if err != nil {
		return err
	}

	b.Hash = hash[:]
	b.Nonce = nonce

	if config.MinimumBlockBuildingTime > 0 {
		for t := time.Since(starttime).Seconds(); t < float64(config.MinimumBlockBuildingTime); t = time.Since(starttime).Seconds() {
			time.Sleep(1 * time.Second)
			n.Logger.Trace.Printf("Sleep")
		}
	}

	n.Logger.Trace.Printf("Minting: New hash is %x\n", b.Hash)

	return nil
}

// this builds a block object from given transactions list
// adds coinbase transacion (prize for miner)
func (n *NodeBlockMaker) makeNewBlockFromTransactions(transactions []*structures.Transaction) (*structures.Block, error) {
	// get last block info
	lastHash, lastHeight, err := n.getBlockchainManager().GetState()

	if err != nil {
		return nil, err
	}

	// add transaction - prize for miner
	cbTx := &structures.Transaction{}

	errc := cbTx.MakeCoinbaseTX(n.MinterAddress, "")

	if errc != nil {
		return nil, errc
	}

	transactions = append(transactions, cbTx)
	/*
		txlist := []*transaction.Transaction{}

		for _, t := range transactions {
			tx, _ := t.Copy()
			txlist = append(txlist, &tx)
		}
	*/
	newblock := structures.Block{}
	err = newblock.PrepareNewBlock(transactions, lastHash[:], lastHeight+1)

	if err != nil {
		return nil, err
	}

	return &newblock, nil
}

// correct a block before adding to blockchain
// it can be that input transactions were used
// or current block transactions were used in other block added paralelly
// wecan continue, we can correct and we can return error here
// TODO . Not sure we have do this work. We can build parallel chain if somethign happens on background
func (n *NodeBlockMaker) FinalBlockCheck(b *structures.Block) error {
	// Blockchain DB should be opened here
	lastHash, _, err := n.getBlockchainManager().GetState()

	if err != nil {
		return err
	}

	if bytes.Compare(lastHash, b.PrevBlockHash) == 0 {
		// all is fine. nothing changed since we started minting
		return nil
	}
	// TODO
	return nil
}
