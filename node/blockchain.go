package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gelembjuk/democoin/lib/utils"
	"github.com/gelembjuk/democoin/node/transaction"
)

const (
	BCBAddState_error                = 0
	BCBAddState_addedToTop           = 1
	BCBAddState_addedToParallelTop   = 2
	BCBAddState_addedToParallel      = 3
	BCBAddState_notAddedNoPrev       = 4
	BCBAddState_notAddedExists       = 5
	BCBState_error                   = -1
	BCBState_canAdd                  = 0
	BCBState_exists                  = 1
	BCBState_notExistAndPrevNotExist = 2
)

/*
* Structure to work with blockchain DB
 */
type Blockchain struct {
	tip             []byte
	db              *bolt.DB
	datadir         string
	Logger          *utils.LoggerMan
	HashCache       map[string]int
	LastHashInCache []byte
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
		b, err := tx.CreateBucket([]byte(BlocksBucket))
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

	bc.tip = utils.CopyBytes(tip)
	bc.db = db
	bc.datadir = datadir

	db.Close()

	return nil
}

// Inits blockchain existent DB
// It just opens a DB. DB access is locked to this process since open
func (bc *Blockchain) Init(datadir string, reason string) error {
	dbFile := datadir + dbFile

	if bc.dbExists(dbFile) == false {
		return errors.New("No existing blockchain found. Create one first.")
	}

	var tip []byte

	err := bc.lockDB(datadir, reason)

	if err != nil {
		return err
	}
	// we will clean this all time when DB is opened
	// the cache will  work only for current session
	bc.HashCache = make(map[string]int)
	bc.LastHashInCache = []byte{}

	db, err := bolt.Open(dbFile, 0600, &bolt.Options{Timeout: 10 * time.Second})

	if err != nil {
		bc.Logger.Trace.Println("Error opening BC " + err.Error())
		return err
	}

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BlocksBucket))
		tip = b.Get([]byte("l"))

		return nil
	})
	if err != nil {
		bc.Logger.Trace.Println("BC read error: " + err.Error())
		return err
	}
	bc.tip = utils.CopyBytes(tip)
	bc.db = db
	bc.datadir = datadir

	return nil
}

// Creates a lock file for DB access. We need this to controll parallel access to the DB
func (bc *Blockchain) lockDB(datadir string, reason string) error {
	lockfile := datadir + dbFileLock

	i := 0

	info, err := os.Stat(lockfile)

	if err == nil {
		t := time.Since(info.ModTime())

		// this is for case when something goes very wrong , process fails and lock is not removed
		if t.Minutes() > 60 {
			os.Remove(lockfile)
		}
	}

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

	starttime := time.Now().UTC().UnixNano()

	bc.Logger.Trace.Printf("Locked for %s", reason)

	_, err = file.WriteString(strconv.Itoa(int(starttime)) + " " + reason)

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
		lockinfobytes, err := ioutil.ReadFile(lockfile)

		if err == nil {
			lockinfo := string(lockinfobytes)

			parts := strings.SplitN(lockinfo, " ", 2)

			reason := parts[1]

			starttime, err := strconv.Atoi(parts[0])

			if err == nil {
				duration := time.Since(time.Unix(0, int64(starttime)))
				ms := duration.Nanoseconds() / int64(time.Millisecond)
				bc.Logger.Trace.Printf("UnLocked for %s after %d ms", reason, ms)
			}
		}
		os.Remove(lockfile)
	}
}

// Closes blockchain DB. After this call db is not accesible. It is needed to call Init to open it again
// This frees access to the DB by other processes
func (bc *Blockchain) Close() {
	bc.db.Close()
	bc.db = nil
	bc.HashCache = nil
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

	err := bc.db.Update(func(txDB *bolt.Tx) error {
		b := txDB.Bucket([]byte(BlocksBucket))
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
	err := bc.db.Update(func(txDB *bolt.Tx) error {
		b := txDB.Bucket([]byte(BlocksBucket))
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
func (bc *Blockchain) FindTransaction(ID []byte, tip []byte) (*transaction.Transaction, map[int][]byte, []byte, error) {
	var bci *BlockchainIterator

	if len(tip) > 0 {
		bci = bc.IteratorFrom(tip)
	} else {
		bci = bc.Iterator()
	}

	txo := map[int][]byte{}

	for {
		block, _ := bci.Next()

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
* Iterator returns a BlockchainIterator . Can be used to do something with blockchain from outside
 */
func (bc *Blockchain) Iterator() *BlockchainIterator {
	starttip := utils.CopyBytes(bc.tip)

	bci := &BlockchainIterator{starttip, bc.db}

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
		block, _ := bci.Next()

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
		b := tx.Bucket([]byte(BlocksBucket))
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
		b := tx.Bucket([]byte(BlocksBucket))

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
		b := tx.Bucket([]byte(BlocksBucket))

		blockData := b.Get(blockHash)

		if blockData == nil {
			return errors.New("Block is not found.")
		}
		blocktmp := Block{}
		err := blocktmp.DeserializeBlock(blockData)

		if err != nil {
			return err
		}

		block = *blocktmp.Copy()

		return nil
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
		block, _ := bci.Next()

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
		block, _ := bci.Next()
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
		block, _ := bci.Next()

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
		block, _ := bci.Next()

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
		b := tx.Bucket([]byte(BlocksBucket))
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
	lastHashFinal := utils.CopyBytes(lastHash)
	return lastHashFinal, lastHeight, nil
}

// Returns a chain of blocks starting from a hash and till
// end of blockchain or block from main chain found
// if already in main chain then returns empty list
// Returns also a block from main chain which is the base of the side branch
//
// The function load all hashes to the memory from "main" chain

func (bc *Blockchain) GetSideBranch(hash []byte, currentTip []byte) ([]*Block, []*Block, *Block, error) {
	// get 2 blocks with hashes from arguments
	sideblock_o, err := bc.GetBlock(hash)

	if err != nil {
		return nil, nil, nil, err
	}

	topblock_o, err := bc.GetBlock(currentTip)

	if err != nil {
		return nil, nil, nil, err
	}

	sideblock := &sideblock_o
	topblock := &topblock_o

	bc.Logger.Trace.Printf("States: top %d, side %d", topblock.Height, sideblock.Height)

	if sideblock.Height < 1 || topblock.Height < 1 {
		return nil, nil, nil, errors.New("Can not do this for genesis block")
	}

	sideBlocks := []*Block{}
	mainBlocks := []*Block{}

	if sideblock.Height > topblock.Height {
		// go down from side block till heigh is same as top
		bci := bc.IteratorFrom(sideblock.Hash)

		for {
			block, _ := bci.Next()
			bc.Logger.Trace.Printf("next side %x", block.Hash)
			if block.Height == topblock.Height {
				sideblock = block
				break
			}
			sideBlocks = append(sideBlocks, block)
		}
	} else if sideblock.Height < topblock.Height {
		// go down from top block till heigh is same as side
		bci := bc.IteratorFrom(topblock.Hash)

		for {
			block, _ := bci.Next()
			bc.Logger.Trace.Printf("next top %x", block.Hash)
			if block.Height == sideblock.Height {
				topblock = block
				break
			}
			mainBlocks = append(mainBlocks, block)
		}
	}

	// at this point sideblock and topblock have same heigh
	bcis := bc.IteratorFrom(sideblock.Hash)
	bcit := bc.IteratorFrom(topblock.Hash)

	for {
		sideblock, _ = bcis.Next()
		topblock, _ = bcit.Next()

		bc.Logger.Trace.Printf("parallel %x vs %x", sideblock.Hash, topblock.Hash)

		if bytes.Compare(sideblock.Hash, topblock.Hash) == 0 {

			ReverseBlocksSlice(mainBlocks)

			return sideBlocks, mainBlocks, sideblock, nil
		}
		// side blocks are returned in same order asthey are
		// main blocks must be reversed to add them in correct order
		mainBlocks = append(mainBlocks, topblock)
		sideBlocks = append(sideBlocks, sideblock)

		if len(sideblock.PrevBlockHash) == 0 || len(topblock.PrevBlockHash) == 0 {
			return nil, nil, nil, errors.New("No connect with main blockchain")
		}

	}
	// this point should be never reached
	return nil, nil, nil, errors.New("Chain error")
}

/*
* Returns a chain of blocks starting fron a hash and till
* end of blockchain or block from main chain found
* if already in main chain then returns empty list
*
* The function load all hashes to the memory from "main" chain
 */
func (bc *Blockchain) GetBranchesReplacement(sideBranchHash []byte, tip []byte) ([]*Block, []*Block, error) {
	bc.Logger.Trace.Printf("Go to get branch %x %x", sideBranchHash, tip)
	sideBlocks, mainBlocks, BCBlock, err := bc.GetSideBranch(sideBranchHash, tip)
	bc.Logger.Trace.Printf("Result sideblocks %d mainblocks %d", len(sideBlocks), len(mainBlocks))
	bc.Logger.Trace.Printf("%x", BCBlock.Hash)

	if err != nil {
		return nil, nil, err
	}

	if bytes.Compare(BCBlock.Hash, sideBranchHash) == 0 {
		// side branch is part of the tip chain
		return nil, nil, nil
	}
	bc.Logger.Trace.Println("Main blocks")
	for _, b := range mainBlocks {
		bc.Logger.Trace.Printf("%x", b.Hash)
	}
	bc.Logger.Trace.Println("Side blocks")
	for _, b := range sideBlocks {
		bc.Logger.Trace.Printf("%x", b.Hash)
	}
	return mainBlocks, sideBlocks, nil
}

func (bc *Blockchain) dbExists(dbFile string) bool {
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		return false
	}

	return true
}

// Check block is in the chain
func (bc *Blockchain) GetBlockInTheChain(blockHash []byte, tip []byte) (int, error) {
	itertorstart := []byte{}

	if len(tip) == 0 {
		if val, ok := bc.HashCache[hex.EncodeToString(blockHash)]; ok {
			return val, nil
		}

		if len(bc.HashCache) > 0 && len(bc.LastHashInCache) == 0 {
			return -1, nil
		}
		if len(bc.HashCache) > 0 && len(bc.LastHashInCache) > 0 {
			itertorstart = bc.LastHashInCache[:]
		}
	} else {
		itertorstart = tip[:]
	}

	var bci *BlockchainIterator

	if len(itertorstart) > 0 {
		bci = bc.IteratorFrom(itertorstart)
	} else {
		bci = bc.Iterator()
	}

	for {
		block, _ := bci.Next()

		if len(tip) == 0 {
			bc.HashCache[hex.EncodeToString(block.Hash)] = block.Height

			bc.LastHashInCache = block.PrevBlockHash[:]
		}

		if bytes.Compare(block.Hash, blockHash) == 0 {
			return block.Height, nil
		}

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}
	return -1, nil
}

// Returns geneesis block hash
func (bc *Blockchain) GetGenesisBlockHash() ([]byte, error) {
	var hash []byte

	err := bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BlocksBucket))
		hash = b.Get([]byte("f"))

		if hash != nil {
			hash = utils.CopyBytes(hash)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if hash != nil {
		return hash, nil
	}
	bci := bc.Iterator()

	for {
		block, _ := bci.Next()

		if len(block.PrevBlockHash) == 0 {
			hash = utils.CopyBytes(block.Hash)
			break
		}
	}

	if hash == nil {
		return nil, errors.New("Genesis block is not found")
	}

	err = bc.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BlocksBucket))

		return b.Put([]byte("f"), hash)
	})
	if err != nil {
		return nil, err
	}

	return hash, nil
}
