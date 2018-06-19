package nodemanager

import (
	"errors"

	"github.com/gelembjuk/democoin/lib/utils"
	"github.com/gelembjuk/democoin/lib/wallet"
	"github.com/gelembjuk/democoin/node/blockchain"
	"github.com/gelembjuk/democoin/node/consensus"
	"github.com/gelembjuk/democoin/node/structures"
	"github.com/gelembjuk/democoin/node/transactions"
)

type NodeBlockchain struct {
	Logger        *utils.LoggerMan
	MinterAddress string
	DBConn        *Database
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
	BCI *blockchain.BlockchainIterator
}

// Returns next block in iterator. First will be the top block
func (bci *BlocksIterator) Next() *BlockInfo {
	block, err := bci.BCI.Next()

	if err != nil || block == nil {
		return nil
	}

	Block := BlockInfo{}
	Block.Hash = block.Hash
	Block.Height = block.Height
	Block.PrevBlockHash = block.PrevBlockHash

	Block.Transactions = []string{}

	for _, tx := range block.Transactions {
		Block.Transactions = append(Block.Transactions, tx.String())
	}
	return &Block
}

func (n *NodeBlockchain) GetBCManager() *blockchain.Blockchain {
	bcm, _ := blockchain.NewBlockchainManager(n.DBConn.DB(), n.Logger)
	return bcm
}

func (n *NodeBlockchain) getTransactionsManager() transactions.TransactionsManagerInterface {
	return transactions.NewManager(n.DBConn.DB(), n.Logger)
}

// Checks if a block exists in the chain. It will go over blocks list
func (n *NodeBlockchain) CheckBlockExists(blockHash []byte) (bool, error) {
	return n.GetBCManager().CheckBlockExists(blockHash)
}

// Get block objet by hash
func (n *NodeBlockchain) GetBlock(hash []byte) (*structures.Block, error) {
	block, err := n.GetBCManager().GetBlock(hash)

	return &block, err
}

// Returns height of the chain. Index of top block
func (n *NodeBlockchain) GetBestHeight() (int, error) {
	bcm := n.GetBCManager()

	bestHeight, err := bcm.GetBestHeight()

	if err != nil {
		return 0, err
	}

	return bestHeight, nil
}

// Return top hash
func (n *NodeBlockchain) GetTopBlockHash() ([]byte, error) {
	bcm := n.GetBCManager()

	topHash, _, err := bcm.GetState()

	if err != nil {
		return nil, err
	}

	return topHash, nil
}

// BUilds a genesis block. It is used only to start new blockchain
func (n *NodeBlockchain) PrepareGenesisBlock(address, genesisCoinbaseData string) (*structures.Block, error) {
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

	cbtx := &structures.Transaction{}

	errc := cbtx.MakeCoinbaseTX(address, genesisCoinbaseData)

	if errc != nil {
		return nil, errc
	}

	genesis := &structures.Block{}
	genesis.PrepareNewBlock([]*structures.Transaction{cbtx}, []byte{}, 0)

	return genesis, nil
}

// Create new blockchain from given genesis block
func (n *NodeBlockchain) CreateBlockchain(genesis *structures.Block) error {
	n.Logger.Trace.Println("Init DB")

	n.DBConn.CloseConnection() // close in case if it was opened before

	err := n.DBConn.InitDatabase()

	if err != nil {
		n.Logger.Error.Printf("Can not init DB: %s", err.Error())
		return err
	}

	bcdb, err := n.DBConn.DB().GetBlockchainObject()

	if err != nil {
		n.Logger.Error.Printf("Can not create conn object: %s", err.Error())
		return err
	}

	blockdata, err := genesis.Serialize()

	if err != nil {
		return err
	}

	err = bcdb.PutBlockOnTop(genesis.Hash, blockdata)

	if err != nil {
		return err
	}

	err = bcdb.SaveFirstHash(genesis.Hash)

	if err != nil {
		return err
	}

	// add first rec to chain list
	bcdb.AddToChain(genesis.Hash, []byte{})

	return err
}

// Creates iterator to go over blockchain
func (n *NodeBlockchain) GetBlockChainIterator() (*BlocksIterator, error) {
	bcicore, err := blockchain.NewBlockchainIterator(n.DBConn.DB())

	if err != nil {
		return nil, err
	}
	bci := BlocksIterator{bcicore}
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
	bci, err := blockchain.NewBlockchainIterator(n.DBConn.DB())

	if err != nil {
		return nil, err
	}

	pubKeyHash, _ := utils.AddresToPubKeyHash(address)

	for {
		block, _ := bci.Next()

		for _, tx := range block.Transactions {

			income := float64(0)

			spent := false
			spentaddress := ""

			// we presume all inputs in tranaction are always from same wallet
			for _, in := range tx.Vin {
				spentaddress, _ = utils.PubKeyToAddres(in.PubKey)

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
						destaddress, _ = utils.PubKeyHashToAddres(out.PubKeyHash)
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
func (n *NodeBlockchain) DropBlock() (*structures.Block, error) {
	return n.GetBCManager().DeleteBlock()
}

// Add block to blockchain
// Block is not yet verified
func (n *NodeBlockchain) AddBlock(block *structures.Block) (uint, error) {
	// do some checks of the block
	// check if block exists
	blockstate, err := n.CheckBlockState(block.Hash, block.PrevBlockHash)

	if err != nil {
		return 0, err
	}

	if blockstate == 1 {
		// block exists. no sese to continue
		return blockchain.BCBAddState_notAddedExists, nil
	}

	if blockstate == 2 {
		// previous bock is not found. can not add
		return blockchain.BCBAddState_notAddedNoPrev, nil
	}

	Minter, err := consensus.NewConsensusManager(n.MinterAddress, n.DBConn.DB(), n.Logger)

	if err != nil {
		return 0, err
	}
	// verify this block against rules.
	err = Minter.VerifyBlock(block)

	if err != nil {
		return 0, err
	}

	return n.GetBCManager().AddBlock(block)
}

// returns two branches of a block starting from their common block.
// One of branches is primary at this time
func (n *NodeBlockchain) GetBranchesReplacement(sideBranchHash []byte, tip []byte) ([]*structures.Block, []*structures.Block, error) {
	bcm := n.GetBCManager()

	if len(tip) == 0 {
		tip, _, _ = bcm.GetState()
	}
	return bcm.GetBranchesReplacement(sideBranchHash, tip)
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
func (n *NodeBlockchain) GetBlocksAfter(hash []byte) ([]*structures.BlockShort, error) {
	exists, err := n.CheckBlockExists(hash)

	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, nil // nil for blocks list means given hash is not found
	}

	// there are 2 cases: block is in main branch , and it is not in main branch
	// this will be nil if a hash is not in a chain

	blocks := n.GetBCManager().GetNextBlocks(hash)

	return blocks, nil
}
