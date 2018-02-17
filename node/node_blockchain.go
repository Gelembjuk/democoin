package main

import (
	"errors"

	"github.com/gelembjuk/democoin/lib"
	"github.com/gelembjuk/democoin/lib/transaction"
	"github.com/gelembjuk/democoin/lib/wallet"
)

type NodeBlockchain struct {
	Logger        *lib.LoggerMan
	DataDir       string
	MinterAddress string
	BC            *Blockchain
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

// Creates new blockchain DB from given list of blocks
// This would be used when new empty node started and syncs with other nodes
func (n *NodeBlockchain) CreateBlockchainFromBlocks(blocks [][]byte) (int, error) {
	if len(blocks) == 0 {
		return -1, errors.New("No blocks in the list to init from")
	}
	bc := Blockchain{}

	firstblockbytes := blocks[0]

	blocks = blocks[1:]

	block := &Block{}
	err := block.DeserializeBlock(firstblockbytes)

	if err != nil {
		return -1, err
	}

	err = bc.CreateBlockchain(n.DataDir, block)

	if err != nil {
		return -1, err
	}

	maxHeight := block.Height

	n.OpenBlockchain()

	for _, blockdata := range blocks {
		block := &Block{}
		err := block.DeserializeBlock(blockdata)

		if err != nil {
			return -1, err
		}
		n.AddBlock(block)

		maxHeight = block.Height
	}
	return maxHeight, nil
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
func (n *NodeBlockchain) AddBlock(block *Block) (uint, error) {
	// do some checks of the block
	return n.BC.AddBlock(block)
}

// returns two branches of a block starting from their common block.
// One of branches is primary at this time
func (n *NodeBlockchain) GetBranchesReplacement(sideBranchHash []byte) ([]*Block, []*Block, error) {
	return n.BC.GetBranchesReplacement(sideBranchHash)
}

/*
* Checks state of a block by hashes
* returns
* 0 if block doesn't exist and prev block exists
* 1 if block exists
* 2 if block doesn't exist and prev block doesn't exist
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
