package main

import (
	"errors"
	"fmt"

	"github.com/gelembjuk/democoin/lib"
	"github.com/gelembjuk/democoin/lib/transaction"
	"github.com/gelembjuk/democoin/lib/wallet"
)

type NodeBlockchain struct {
	Logger        *lib.LoggerMan
	DataDir       string
	MinterAddress string
	BC            *Blockchain
	NodeTX        *NodeTransactions
}

// this are structures with methods to organize blockchain iterator
// ad use it outside
type BlockInfo struct {
	Timestamp     int64
	Transactions  []string
	PrevBlockHash []byte
	Hash          []byte
	Nonce         int
	Height        int
}

type TransactionsHistory struct {
	IOType  bool
	TXID    []byte
	Address string
	Value   float64
}

// iterator for blockchain. can be used to iterate over BC from outside
type BlocksIterator struct {
	BCI *BlockchainIterator
}

// Returns next block in iterator. First will be the top block
func (bci *BlocksIterator) Next() BlockInfo {
	block := bci.BCI.Next()

	Block := BlockInfo{}
	Block.Hash = block.Hash
	Block.Height = block.Height
	Block.PrevBlockHash = block.PrevBlockHash

	Block.Transactions = []string{}

	for _, tx := range block.Transactions {
		Block.Transactions = append(Block.Transactions, tx.String())
	}
	return Block
}

// Close iterator
func (bci *BlocksIterator) Close() {
	bci.BCI.db.Close()
}

/*
* Set pointer to already opened blockchain DB
 */
func (n *NodeBlockchain) SetBlockchain(bc *Blockchain) {
	n.BC = bc
}

/*
* Opens Blockchain DB
 */
func (n *NodeBlockchain) OpenBlockchain() error {

	if n.BC != nil {
		return nil
	}
	bc := Blockchain{}

	bc.Logger = n.Logger

	err := bc.Init(n.DataDir)

	if err != nil {
		return err
	}

	n.BC = &bc

	return nil
}

/*
* Closes Blockchain DB
 */
func (n *NodeBlockchain) CloseBlockchain() {
	if n.BC != nil {
		n.BC.Close()
		n.BC = nil
	}
}

// Returns reference to BC object. If some other structure wants to acces the DB directly
func (n *NodeBlockchain) GetBlockChainObject() *Blockchain {
	return n.BC
}

//Get minimum and maximum number of transaction allowed in block for current chain
func (n *NodeBlockchain) GetTransactionNumbersLimits(block *Block) (int, int, error) {
	var min int

	if block == nil {
		bestHeight, err := n.BC.GetBestHeight()

		if err != nil {
			return 0, 0, err
		}
		min = bestHeight
	} else {
		min = block.Height
	}

	if min > maxMinNumberTransactionInBlock {
		min = maxMinNumberTransactionInBlock
	} else if min < 1 {
		min = 1
	}

	return min, maxNumberTransactionInBlock, nil
}

// Checks if a block exists in the chain. It will go over blocks list
func (n *NodeBlockchain) CheckBlockExists(blockHash []byte) (bool, error) {
	return n.BC.CheckBlockExists(blockHash)
}

// Returns height of the chain. Index of top block
func (n *NodeBlockchain) GetBestHeight() (int, error) {
	if n.BC == nil {
		err := n.OpenBlockchain()

		if err != nil {
			return 0, err
		}
		defer n.CloseBlockchain()
	}

	bestHeight, err := n.BC.GetBestHeight()

	if err != nil {
		return 0, err
	}

	return bestHeight, nil
}

// BUilds a genesis block. It is used only to start new blockchain
func (n *NodeBlockchain) PrepareGenesisBlock(address, genesisCoinbaseData string) (*Block, error) {
	if address == "" {
		return nil, errors.New("Geneisis block wallet address missed")
	}

	w := wallet.Wallet{}

	if !w.ValidateAddress(address) {
		return nil, errors.New("Address is not valid")
	}

	if genesisCoinbaseData == "" {
		return nil, errors.New("Geneisis block text missed")
	}

	cbtx := &transaction.Transaction{}

	errc := cbtx.MakeCoinbaseTX(address, genesisCoinbaseData)

	if errc != nil {
		return nil, errc
	}

	genesis := &Block{}
	genesis.PrepareNewBlock([]*transaction.Transaction{cbtx}, []byte{}, 0)

	return genesis, nil
}

// Create new blockchain from given genesis block
func (n *NodeBlockchain) CreateBlockchain(genesis *Block) error {

	bc := Blockchain{}

	return bc.CreateBlockchain(n.DataDir, genesis)
}

// Creates iterator to go over blockchain
func (n *NodeBlockchain) GetBlockChainIterator() (*BlocksIterator, error) {
	bci := BlocksIterator{n.BC.Iterator()}
	return &bci, nil
}

// Returns history of transactions for given address
func (n *NodeBlockchain) GetAddressHistory(address string) ([]TransactionsHistory, error) {
	result := []TransactionsHistory{}

	if address == "" {
		return result, errors.New("Address is missed")
	}
	w := wallet.Wallet{}

	if !w.ValidateAddress(address) {
		return result, errors.New("Address is not valid")
	}
	if n.BC == nil {
		err := n.OpenBlockchain()

		if err != nil {
			return result, err
		}
		defer n.CloseBlockchain()
	}
	bc := n.BC

	bci := bc.Iterator()

	pubKeyHash, _ := lib.AddresToPubKeyHash(address)

	for {
		block := bci.Next()

		for _, tx := range block.Transactions {

			income := float64(0)

			spent := false
			spentaddress := ""

			// we presume all inputs in tranaction are always from same wallet
			for _, in := range tx.Vin {
				spentaddress, _ = lib.PubKeyToAddres(in.PubKey)

				if in.UsesKey(pubKeyHash) {
					spent = true
					break
				}
			}

			if spent {
				// find how many spent , part of out can be exchange to same address

				spentvalue := float64(0)
				totalvalue := float64(0) // we need to know total if wallet sent to himself

				destaddress := ""

				// we agree that there can be only one destination in transaction. we don't support scripts
				for _, out := range tx.Vout {
					if !out.IsLockedWithKey(pubKeyHash) {
						spentvalue += out.Value
						destaddress, _ = lib.PubKeyHashToAddres(out.PubKeyHash)
					}
				}

				if spentvalue > 0 {
					result = append(result, TransactionsHistory{false, tx.ID, destaddress, spentvalue})
				} else {
					// spent to himself. this should not be usual case
					result = append(result, TransactionsHistory{false, tx.ID, address, totalvalue})
					result = append(result, TransactionsHistory{true, tx.ID, address, totalvalue})
				}
			} else if tx.IsCoinbase() {

				if tx.Vout[0].IsLockedWithKey(pubKeyHash) {
					spentaddress = "Coin base"
					income = tx.Vout[0].Value
				}
			} else {

				for _, out := range tx.Vout {

					if out.IsLockedWithKey(pubKeyHash) {
						income += out.Value
					}
				}
			}

			if income > 0 {
				result = append(result, TransactionsHistory{true, tx.ID, spentaddress, income})
			}
		}

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}

	return result, nil
}

// Drop block from a top of blockchain
func (n *NodeBlockchain) DropBlock() (*Block, error) {
	return n.BC.DeleteBlock()
}

// Add block to blockchain
// Block is not yet verified
func (n *NodeBlockchain) AddBlock(block *Block) (uint, error) {
	// do some checks of the block
	// check if block exists
	blockstate, err := n.CheckBlockState(block.Hash, block.PrevBlockHash)

	if err != nil {
		return 0, err
	}

	if blockstate == 1 {
		// block exists. no sese to continue
		return BCBAddState_notAddedExists, nil
	}

	if blockstate == 2 {
		// previous bock is not found. can not add
		return BCBAddState_notAddedNoPrev, nil
	}

	// verify this block against rules.
	err = n.VerifyBlock(block)

	if err != nil {
		return 0, err
	}

	return n.BC.AddBlock(block)
}

// returns two branches of a block starting from their common block.
// One of branches is primary at this time
func (n *NodeBlockchain) GetBranchesReplacement(sideBranchHash []byte, tip []byte) ([]*Block, []*Block, error) {
	if len(tip) == 0 {
		tip, _, _ = n.BC.GetState()
	}
	return n.BC.GetBranchesReplacement(sideBranchHash, tip)
}

/*
* Checks state of a block by hashes
* returns
* -1 BCBState_error
* 0 BCBState_canAdd if block doesn't exist and prev block exists
* 1 BCBState_exists if block exists
* 2 BCBState_notExistAndPrevNotExist if block doesn't exist and prev block doesn't exist
 */
func (n *NodeBlockchain) CheckBlockState(hash, prevhash []byte) (int, error) {
	exists, err := n.CheckBlockExists(hash)

	if err != nil {
		return -1, err
	}

	if exists {
		return 1, nil
	}
	exists, err = n.CheckBlockExists(prevhash)

	if err != nil {
		return -1, err
	}

	if exists {
		return 0, nil
	}

	return 2, nil
}

// Get next blocks uppper then given
func (n *NodeBlockchain) GetBlocksAfter(hash []byte) ([]*BlockShort, error) {
	exists, err := n.CheckBlockExists(hash)

	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, nil // nil for blocks list means given hash is not found
	}

	// there are 2 cases: block is in main branch , and it is not in main branch
	// this will be nil if a hash is not in a chain
	blocks := n.BC.GetNextBlocks(hash)

	return blocks, nil
}

// Verify a block against blockchain
// RULES
// 0. Verification is done agains blockchain branch starting from prevblock, not current top branch
// 1. There can be only 1 block make reward transaction
// 2. number of transactions must be in correct ranges (reward transaction is not calculated)
// 3. transactions can have as input other transaction from this block and it must be listed BEFORE
//   (output must be before input in same block)
// 4. all inputs must be in blockchain (correct unspent inputs)
// 5. Additionally verify each transaction agains signatures, total amount, balance etc
func (n *NodeBlockchain) VerifyBlock(block *Block) error {
	//TODO

	// 2. check number of TX
	txnum := len(block.Transactions) - 1 /*minus coinbase TX*/

	min, max, err := n.GetTransactionNumbersLimits(block)

	if err != nil {
		return err
	}

	if txnum < min {
		return errors.New("Number of transactions is too low")
	}

	if txnum > max {
		return errors.New("Number of transactions is too high")
	}

	// 1
	coinbaseused := false

	prevTXs := []*transaction.Transaction{}

	for _, tx := range block.Transactions {
		if tx.IsCoinbase() {
			if coinbaseused {
				return errors.New("2 coin base TX in the block")
			}
			coinbaseused = true
		}
		vtx, err := n.NodeTX.VerifyTransactionDeep(tx, prevTXs, block.PrevBlockHash)

		if err != nil {
			return err
		}

		if !vtx {
			return errors.New(fmt.Sprintf("Transaction in a block is not valid: %x", tx.ID))
		}

		prevTXs = append(prevTXs, tx)
	}
	if !coinbaseused {
		return errors.New("No coinbase TX in the block")
	}
	return nil
}
