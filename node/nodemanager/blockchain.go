package nodemanager

import (
	"errors"
	"fmt"

	"github.com/gelembjuk/democoin/lib/utils"
	"github.com/gelembjuk/democoin/lib/wallet"
	"github.com/gelembjuk/democoin/node/blockchain"
	"github.com/gelembjuk/democoin/node/consensus"
	"github.com/gelembjuk/democoin/node/transaction"
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
	bcm, _ := blockchain.NewBlockchainManager(n.DBConn.DB, n.Logger)
	return bcm
}

func (n *NodeBlockchain) getTransactionsManager() *transactions.Manager {
	return transactions.NewManager(n.DBConn.DB, n.Logger)
}

// Checks if a block exists in the chain. It will go over blocks list
func (n *NodeBlockchain) CheckBlockExists(blockHash []byte) (bool, error) {
	return n.GetBCManager().CheckBlockExists(blockHash)
}

// Get block objet by hash
func (n *NodeBlockchain) GetBlock(hash []byte) (*blockchain.Block, error) {
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
func (n *NodeBlockchain) PrepareGenesisBlock(address, genesisCoinbaseData string) (*blockchain.Block, error) {
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

	genesis := &blockchain.Block{}
	genesis.PrepareNewBlock([]*transaction.Transaction{cbtx}, []byte{}, 0)

	return genesis, nil
}

// Create new blockchain from given genesis block
func (n *NodeBlockchain) CreateBlockchain(genesis *blockchain.Block) error {
	n.Logger.Trace.Println("Init DB")

	// this creates DB connection object but doesn't try to connect to DB
	n.DBConn.PrepareConnection("")

	err := n.DBConn.DB.InitDatabase()

	// clean DB connection object. it will be opened later again
	n.DBConn.CleanConnection()

	if err != nil {
		n.Logger.Error.Printf("Can not init DB: %s", err.Error())
		return err
	}
	n.DBConn.OpenConnection("AddGeneis", "")

	defer n.DBConn.CloseConnection()

	n.Logger.Trace.Println("Go to create DB connection")
	bcdb, err := n.DBConn.DB.GetBlockchainObject()

	if err != nil {
		n.Logger.Error.Printf("Can not create conn object: %s", err.Error())
		return err
	}

	blockdata, err := genesis.Serialize()

	if err != nil {
		return err
	}

	err = bcdb.PutBlockOnTop(genesis.Hash, blockdata)

	return err
}

// Creates iterator to go over blockchain
func (n *NodeBlockchain) GetBlockChainIterator() (*BlocksIterator, error) {
	bcicore, err := blockchain.NewBlockchainIterator(n.DBConn.DB)

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
	bci, err := blockchain.NewBlockchainIterator(n.DBConn.DB)

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
func (n *NodeBlockchain) DropBlock() (*blockchain.Block, error) {
	return n.GetBCManager().DeleteBlock()
}

// Add block to blockchain
// Block is not yet verified
func (n *NodeBlockchain) AddBlock(block *blockchain.Block) (uint, error) {
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

	// verify this block against rules.
	err = n.VerifyBlock(block)

	if err != nil {
		return 0, err
	}

	return n.GetBCManager().AddBlock(block)
}

// returns two branches of a block starting from their common block.
// One of branches is primary at this time
func (n *NodeBlockchain) GetBranchesReplacement(sideBranchHash []byte, tip []byte) ([]*blockchain.Block, []*blockchain.Block, error) {
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
func (n *NodeBlockchain) GetBlocksAfter(hash []byte) ([]*blockchain.BlockShort, error) {
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

// Verify a block against blockchain
// RULES
// 0. Verification is done agains blockchain branch starting from prevblock, not current top branch
// 1. There can be only 1 transaction make reward per block
// 2. number of transactions must be in correct ranges (reward transaction is not calculated)
// 3. transactions can have as input other transaction from this block and it must be listed BEFORE
//   (output must be before input in same block)
// 4. all inputs must be in blockchain (correct unspent inputs)
// 5. Additionally verify each transaction agains signatures, total amount, balance etc
// 6. Verify hash is correc agains rules
func (n *NodeBlockchain) VerifyBlock(block *blockchain.Block) error {
	//6. Verify hash

	pow := consensus.NewProofOfWork(block)

	valid, err := pow.Validate()

	if err != nil {
		return err
	}

	if !valid {
		return errors.New("Block hash is not valid")
	}
	n.Logger.Trace.Println("block hash verified")
	// 2. check number of TX
	txnum := len(block.Transactions) - 1 /*minus coinbase TX*/

	bcm := n.GetBCManager()

	min, max, err := bcm.GetTransactionNumbersLimits(block)

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
		vtx, err := n.getTransactionsManager().VerifyTransactionDeep(tx, prevTXs, block.PrevBlockHash)

		if err != nil {
			return err
		}

		if !vtx {
			return errors.New(fmt.Sprintf("Transaction in a block is not valid: %x", tx.ID))
		}

		prevTXs = append(prevTXs, tx)
	}
	// 1.
	if !coinbaseused {
		return errors.New("No coinbase TX in the block")
	}
	return nil
}
