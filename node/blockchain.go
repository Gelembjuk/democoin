package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gelembjuk/democoin/lib"
	"github.com/gelembjuk/democoin/lib/transaction"
)

const (
	BCBAddState_error              = 0
	BCBAddState_addedToTop         = 1
	BCBAddState_addedToParallelTop = 2
	BCBAddState_addedToParallel    = 3
	BCBAddState_notAddedNoPrev     = 4
	BCBAddState_notAddedExists     = 5
)

/*
* Structure to work with blockchain DB
 */
type Blockchain struct {
	tip     []byte
	db      *bolt.DB
	datadir string
	Logger  *lib.LoggerMan
}

// CreateBlockchain creates a new blockchain DB. Genesis block is received as argument

func (bc *Blockchain) CreateBlockchain(datadir string, genesis *Block) error {

	dbFile := datadir + dbFile

	if bc.dbExists(dbFile) {
		return errors.New("Blockchain already exists in the folder.")
	}

	var tip []byte

	db, err := bolt.Open(dbFile, 0600, nil)

	if err != nil {
		return err

	}

	err = db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket([]byte(blocksBucket))
		if err != nil {
			return err
		}

		bs, err := genesis.Serialize()

		if err != nil {
			return err

		}

		err = b.Put(genesis.Hash, bs)

		if err != nil {
			return err

		}

		err = b.Put([]byte("l"), genesis.Hash)
		if err != nil {
			return err
		}
		tip = genesis.Hash

		return nil
	})
	if err != nil {
		return err
	}

	bc.tip = tip
	bc.db = db
	bc.datadir = datadir

	db.Close()

	return nil
}

// Inits blockchain existent DB
// It just opens a DB. DB access is locked to this process since open
func (bc *Blockchain) Init(datadir string) error {
	dbFile := datadir + dbFile

	if bc.dbExists(dbFile) == false {
		return errors.New("No existing blockchain found. Create one first.")
	}

	var tip []byte

	err := bc.lockDB(datadir)

	if err != nil {
		return err
	}

	db, err := bolt.Open(dbFile, 0600, &bolt.Options{Timeout: 1 * time.Second})

	if err != nil {
		bc.Logger.Trace.Println("Error opening BC " + err.Error())
		return err
	}

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		tip = b.Get([]byte("l"))

		return nil
	})
	if err != nil {
		bc.Logger.Trace.Println("BC read error: " + err.Error())
		return err
	}
	bc.tip = tip
	bc.db = db
	bc.datadir = datadir

	return nil
}

// Creates a lock file for DB access. We need this to controll parallel access to the DB
func (bc *Blockchain) lockDB(datadir string) error {
	lockfile := datadir + dbFileLock

	i := 0

	for bc.dbExists(lockfile) != false {
		time.Sleep(1 * time.Second)
		i++

		if i > 100 {
			return errors.New("Can not open DB. Lock failed after many attempts")
		}
	}

	file, err := os.Create(lockfile)

	if err != nil {
		return err
	}

	defer file.Close()

	_, err = file.WriteString("1")

	if err != nil {
		return err
	}

	file.Sync() // flush to disk

	return nil
}

// Removes DB lock file
func (bc *Blockchain) unLockDB() {
	lockfile := bc.datadir + dbFileLock

	if bc.dbExists(lockfile) != false {
		os.Remove(lockfile)
	}
}

// Closes blockchain DB. After this call db is not accesible. It is needed to call Init to open it again
// This frees access to the DB by other processes
func (bc *Blockchain) Close() {
	bc.db.Close()
	bc.db = nil
	bc.unLockDB()
}

/*
* Add new block to the blockchain
	BCBAddState_error              = 0 not added to the chain. Because of error
	BCBAddState_addedToTop         = 1 added to the top of current chain
	BCBAddState_addedToParallelTop = 2 added to the top, but on other branch. Other branch becomes primary now
	BCBAddState_addedToParallel    = 3 added but not in main branch and heigh i lower then main branch
	BCBAddState_notAddedNoPrev     = 4 previous not found
	BCBAddState_notAddedExists     = 5 already in blockchain
*
*/
func (bc *Blockchain) AddBlock(block *Block) (uint, error) {
	bc.Logger.Trace.Printf("Adding new block to block chain %x", block.Hash)

	addstatus := uint(BCBAddState_error)

	err := bc.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		blockInDb := b.Get(block.Hash)

		if blockInDb != nil {
			bc.Logger.Trace.Printf("Block is already in blockchain")
			addstatus = BCBAddState_notAddedExists // already in blockchain
			return nil
		}

		// check previous block exists
		prevBlockData := b.Get(block.PrevBlockHash)

		if prevBlockData == nil {
			// previous block is not yet in our DB
			addstatus = BCBAddState_notAddedNoPrev // means block is not added because previous is not in the DB
			return nil
		}

		// add this block
		blockData, err := block.Serialize()
		if err != nil {
			return err
		}

		err = b.Put(block.Hash, blockData)

		if err != nil {
			return err
		}

		lastHash := b.Get([]byte("l"))
		lastBlockData := b.Get(lastHash)

		lastBlock := Block{}
		err = lastBlock.DeserializeBlock(lastBlockData)

		if err != nil {
			return err
		}

		bc.Logger.Trace.Printf("Current BC state %d , %x\n", lastBlock.Height, lastHash)
		bc.Logger.Trace.Printf("New block height %d\n", block.Height)

		if block.Height > lastBlock.Height {
			// the block becomes highest and is top of he blockchain
			err = b.Put([]byte("l"), block.Hash)

			if err != nil {
				return err

			}
			bc.Logger.Trace.Printf("Set new current hash %x\n", block.Hash)
			bc.tip = block.Hash
			addstatus = BCBAddState_addedToTop // added to the top

			if bytes.Compare(lastHash, block.PrevBlockHash) != 0 {
				// other branch becomes main branch now.
				// it is needed to reindex unspent transactions and non confirmed
				addstatus = BCBAddState_addedToParallelTop // added to the top, but on other branch
			}
		} else {
			// block added, but is not on the top
			addstatus = BCBAddState_addedToParallel
		}

		return nil
	})
	if err != nil {
		return BCBAddState_error, err
	}
	return addstatus, nil
}

/*
* DeleteBlock deletes the top block (last added)
* The function extracts the last block, deletes it and sets the tip to point to
* previous block.
* TODO
* It is needed to make some more correct logic. f a block is removed then tip could go to some other blocks branch that
* is longer now. It is needed to care blockchain branches
* Returns deleted block object
 */
func (bc *Blockchain) DeleteBlock() (*Block, error) {
	var block *Block
	err := bc.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		blockInDb := b.Get(bc.tip)

		if blockInDb == nil {
			return errors.New("Top block is not found!")
		}

		block = &Block{}

		err := block.DeserializeBlock(blockInDb)

		if err != nil {
			return err

		}

		err = b.Put([]byte("l"), block.PrevBlockHash)

		if err != nil {
			return err

		}
		bc.tip = block.PrevBlockHash
		b.Delete(block.Hash)

		return nil
	})
	if err != nil {
		return nil, err
	}
	return block, nil
}

// FindTransaction finds a transaction by its ID
// returns also spending status, if it was already spent or not
// and block hash where transaction is stored
// If block is unknown
func (bc *Blockchain) FindTransaction(ID []byte) (*transaction.Transaction, map[int][]byte, []byte, error) {

	bci := bc.Iterator()

	txo := map[int][]byte{}

	for {
		block := bci.Next()

		for _, tx := range block.Transactions {
			if bytes.Compare(tx.ID, ID) == 0 {
				return tx, txo, block.Hash, nil
			}

			for _, txi := range tx.Vin {
				if bytes.Compare(txi.Txid, ID) == 0 {
					// the transaction was spent!!!
					// we remember pubkeys who used the transaction
					// and vout of transaction
					txo[txi.Vout] = txi.PubKey
				}
			}
		}

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}

	return nil, txo, nil, nil
}

// FindTransactionByBlock finds a transaction by its ID in given block
// If block is known . It worsk much faster then FindTransaction
func (bc *Blockchain) FindTransactionByBlock(ID []byte, blockHash []byte) (*transaction.Transaction, error) {
	block, err := bc.GetBlock(blockHash)

	if err != nil {
		return nil, err
	}

	// get transaction from a block
	for _, tx := range block.Transactions {
		if bytes.Compare(tx.ID, ID) == 0 {
			return tx, nil
		}
	}

	return nil, errors.New("Transaction is not found")
}

/*
* Returns full list of unspent transactions outputs
* Iterates over full blockchain
* TODO this will not work for big blockchain. It keeps data in memory
 */
func (bc *Blockchain) FindUnspentTransactions() map[string][]transaction.TXOutputIndependent {
	UTXO := make(map[string][]transaction.TXOutputIndependent)
	spentTXOs := make(map[string][]int)
	bci := bc.Iterator()

	for {
		block := bci.Next()

		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)

			sender := []byte{}

			if tx.IsCoinbase() == false {
				sender, _ = lib.HashPubKey(tx.Vin[0].PubKey)
			}

			for outIdx, out := range tx.Vout {
				// Was the output spent?

				if spentTXOs[txID] != nil {
					spent := false
					for _, spentOutIdx := range spentTXOs[txID] {

						if spentOutIdx == outIdx {
							// this output of the transaction was already spent
							// go to next output of this transaction
							spent = true
							break
						}
					}

					if spent {
						break
					}
				}

				// add to unspent
				outs := UTXO[txID]

				oute := transaction.TXOutputIndependent{}
				oute.LoadFromSimple(out, tx.ID, outIdx, sender, tx.IsCoinbase(), block.Hash)

				outs = append(outs, oute)
				UTXO[txID] = outs
			}

			if tx.IsCoinbase() == false {
				for _, in := range tx.Vin {
					inTxID := hex.EncodeToString(in.Txid)
					spentTXOs[inTxID] = append(spentTXOs[inTxID], in.Vout)

				}
			}
		}

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}

	return UTXO
}

/*
* Iterator returns a BlockchainIterator . Can be used to do something with blockchain from outside
 */
func (bc *Blockchain) Iterator() *BlockchainIterator {
	bci := &BlockchainIterator{bc.tip, bc.db}

	return bci
}

/*
* Iterator returns a BlockchainIterator starting from a given block hash.
 */
func (bc *Blockchain) IteratorFrom(tip []byte) *BlockchainIterator {
	bci := &BlockchainIterator{tip, bc.db}

	return bci
}

/*
* Returns a block with specified height in current blockchain
 */
func (bc *Blockchain) GetBlockAtHeight(height int) (*Block, error) {
	// finds a block with this height

	bci := bc.Iterator()

	for {
		block := bci.Next()

		if block.Height == height {
			return block, nil
		}

		if block.Height < height {
			return nil, errors.New("Block with the heigh doesn't exist")
		}

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}
	return nil, errors.New("Block with the heigh doesn't exist")
}

/*
* GetBestHeight returns the height of the latest block
 */
func (bc *Blockchain) GetBestHeight() (int, error) {
	var lastBlock Block

	err := bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		lastHash := b.Get([]byte("l"))
		blockData := b.Get(lastHash)

		lastBlock = Block{}
		return lastBlock.DeserializeBlock(blockData)
	})
	if err != nil {
		return 0, err
	}

	return lastBlock.Height, nil
}

/*
* Check block exists
 */
func (bc *Blockchain) CheckBlockExists(blockHash []byte) (bool, error) {
	exists := false

	err := bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))

		blockData := b.Get(blockHash)

		if blockData != nil {
			exists = true
		}
		return nil
	})
	if err != nil {
		return false, err
	}

	return exists, nil
}

/*
* GetBlock finds a block by its hash and returns it
 */
func (bc *Blockchain) GetBlock(blockHash []byte) (Block, error) {
	var block Block

	err := bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))

		blockData := b.Get(blockHash)

		if blockData == nil {
			return errors.New("Block is not found.")
		}
		block = Block{}
		return block.DeserializeBlock(blockData)
	})
	if err != nil {
		return block, err
	}

	return block, nil
}

// GetBlockHashes returns a list of hashes of all the blocks in the chain
// TODO
// This can use too much memory. Improve in the future. Add some paging

func (bc *Blockchain) GetBlockHashes() [][]byte {
	var blocks [][]byte
	bci := bc.Iterator()

	for {
		block := bci.Next()

		blocks = append(blocks, block.Hash)

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}

	return blocks
}

/*
* Returns a list of blocks short info stating from given block or from a top
 */
func (bc *Blockchain) GetBlocksShortInfo(startfrom []byte, maxcount int) []*BlockShort {
	var blocks []*BlockShort
	var bci *BlockchainIterator

	if len(startfrom) > 0 {
		bci = bc.IteratorFrom(startfrom)
	} else {
		bci = bc.Iterator()
	}

	for {
		block := bci.Next()
		bs := block.GetShortCopy()

		blocks = append(blocks, bs)

		if len(block.PrevBlockHash) == 0 {
			break
		}

		if len(blocks) > maxcount {
			break
		}
	}

	return blocks
}

/*
* returns a list of hashes of all the blocks in the chain
 */
func (bc *Blockchain) GetNextBlocks(startfrom []byte) []*BlockShort {
	maxcount := 1000

	blocks := []*BlockShort{}

	bci := bc.Iterator()

	found := false

	for {
		block := bci.Next()

		if bytes.Compare(block.Hash, startfrom) == 0 {
			found = true
			break
		}

		bs := block.GetShortCopy()

		blocks = append(blocks, bs)

		if len(blocks) > maxcount+100 {
			// we don't want to truncate after every append
			blocks = blocks[:maxcount]
		}

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}

	if !found {
		return nil
	}

	if len(blocks) > maxcount {
		// final truncate
		blocks = blocks[:maxcount]
	}

	return blocks
}

/*
* Returns first blocks in block chain
 */
func (bc *Blockchain) GetFirstBlocks(maxcount int) ([]*Block, int, error) {
	_, height, err := bc.GetState()

	if err != nil {
		return nil, 0, err
	}
	var bci *BlockchainIterator

	if height > maxcount-1 {
		// find a block with height maxcount-1
		b, err := bc.GetBlockAtHeight(maxcount - 1)

		if err != nil {
			return nil, 0, err
		}
		bci = bc.IteratorFrom(b.Hash)
	} else {
		// start from top block
		bci = bc.Iterator()
	}

	blocks := []*Block{}

	for {
		block := bci.Next()

		blocks = append([]*Block{block}, blocks...)

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}

	return blocks, height, nil
}

/*
* Returns info about the top block. Hash and Height
 */
func (bc *Blockchain) GetState() ([]byte, int, error) {
	var lastHash []byte
	var lastHeight int

	err := bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		lastHash = b.Get([]byte("l"))

		if lastHash == nil {
			return errors.New("No top block address found")
		}

		blockData := b.Get(lastHash)

		if blockData == nil {
			return errors.New("No top block data found")
		}
		block := &Block{}
		err := block.DeserializeBlock(blockData)

		if err != nil {
			return err
		}

		lastHeight = block.Height

		return nil
	})
	if err != nil {
		return nil, -1, err
	}

	return lastHash, lastHeight, nil
}

// Verifies transaction inputs and their signatures
// Is some transaction is not in blockchain, returns nil pointer in map and this input in separate map
// Missed inputs can be some unconfirmed transactions
// Returns: map of previous transactions (full info about input TX). map by input index
// next map is wrong input, where a TX is not found.
// TODO this function is not really good. it iterates over blockchain
func (bc *Blockchain) GetInputTransactionsState(tx *transaction.Transaction) (map[int]*transaction.Transaction, map[int]transaction.TXInput, error) {
	prevTXs := make(map[int]*transaction.Transaction)
	badinputs := make(map[int]transaction.TXInput)

	if tx.IsCoinbase() {
		return prevTXs, badinputs, nil
	}

	for vind, vin := range tx.Vin {
		prevTX, spentouts, _, err := bc.FindTransaction(vin.Txid)

		if err != nil {
			return prevTXs, badinputs, err
		}

		if prevTX == nil {
			// transaction not found
			badinputs[vind] = vin
			prevTXs[vind] = nil
		} else {
			if len(spentouts) > 0 {
				// someone already spent this transaction
				// check if it was this pubkey or some other
				for vout, _ := range spentouts {
					if vout == vin.Vout {
						// this out was already spent before!!!
						// TODO should we check also pub key here? or vout is enough to check?
						return prevTXs, badinputs, errors.New("Transaction input was already spent before")
					}
				}
			}
			// the transaction out was not yet spent
			prevTXs[vind] = prevTX
		}
	}

	return prevTXs, badinputs, nil
}

// Returns a chain of blocks starting fron a hash and till
// end of blockchain or block from main chain found
// if already in main chain then returns empty list
// Returns also a block from main chain which is the base of the side branch
//
// The function load all hashes to the memory from "main" chain

func (bc *Blockchain) GetSideBranch(hash []byte) ([]*Block, *Block, error) {
	return nil, nil, nil
}

/*
* Returns a chain of blocks starting fron a hash and till
* end of blockchain or block from main chain found
* if already in main chain then returns empty list
*
* The function load all hashes to the memory from "main" chain
 */
func (bc *Blockchain) GetBranchesReplacement(sideBranchHash []byte) ([]*Block, []*Block, error) {
	sideBlocks, BCBlock, err := bc.GetSideBranch(sideBranchHash)

	if err != nil {
		return nil, nil, err
	}

	if BCBlock == nil {
		// the branch is not found or is already in main chain
		// or not connected to main chain at all
		return nil, nil, nil
	}

	// iterate main chain till this block and correct blocks
	bci := bc.Iterator()

	mainBlocks := []*Block{}

	for {
		block := bci.Next()

		if bytes.Compare(block.Hash, BCBlock.Hash) == 0 {
			break
		}

		mainBlocks = append(mainBlocks, block)

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}

	return mainBlocks, sideBlocks, nil
}

func (bc *Blockchain) dbExists(dbFile string) bool {
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		return false
	}

	return true
}
