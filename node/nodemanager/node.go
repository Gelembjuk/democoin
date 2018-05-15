package nodemanager

import (
	"crypto/ecdsa"
	"errors"
	"math/rand"
	"time"

	"github.com/gelembjuk/democoin/lib/net"
	"github.com/gelembjuk/democoin/lib/nodeclient"
	"github.com/gelembjuk/democoin/lib/utils"
	"github.com/gelembjuk/democoin/lib/wallet"
	"github.com/gelembjuk/democoin/node/blockchain"
	"github.com/gelembjuk/democoin/node/consensus"
	"github.com/gelembjuk/democoin/node/transaction"
	"github.com/gelembjuk/democoin/node/transactions"
)

/*
* This structure is central part of the application. only it can acces to blockchain and inside it all operation are done
 */

type Node struct {
	NodeBC  NodeBlockchain
	NodeNet net.NodeNetwork
	Logger  *utils.LoggerMan
	DataDir string

	MinterAddress string
	NodeClient    *nodeclient.NodeClient
	OtherNodes    []net.NodeAddr
	DBConn        *Database
}

/*
* Init node.
* Init interfaces of all DBs, blockchain, unspent transactions, unapproved transactions
 */
func (n *Node) Init() {

	n.NodeNet.Logger = n.Logger
	n.NodeBC.Logger = n.Logger

	n.NodeBC.MinterAddress = n.MinterAddress

	n.NodeBC.DBConn = n.DBConn

	// Nodes list storage
	n.NodeNet.SetExtraManager(NodesListStorage{n.DBConn})
	// load list of nodes from config
	n.NodeNet.SetNodes([]net.NodeAddr{}, true)

	n.InitClient()

	rand.Seed(time.Now().UTC().UnixNano())
}

func (n *Node) GetTransactionsManager() *transactions.Manager {
	return transactions.NewManager(n.DBConn.DB, n.Logger)
}
func (n *Node) GetBCManager() (*blockchain.Blockchain, error) {
	return blockchain.NewBlockchainManager(n.DBConn.DB, n.Logger)
}

/*
* Init network client object. It is used to communicate with other nodes
 */
func (n *Node) InitClient() error {
	if n.NodeClient != nil {
		return nil
	}

	client := nodeclient.NodeClient{}

	client.Logger = n.Logger
	client.NodeNet = &n.NodeNet

	n.NodeClient = &client

	return nil
}

/*
* Load list of other nodes addresses
 */
func (n *Node) InitNodes(list []net.NodeAddr, force bool) error {
	if len(list) == 0 && !force {
		n.NodeNet.LoadNodes()
		// load nodes from local storage of nodes
		if n.NodeNet.GetCountOfKnownNodes() == 0 && n.BlockchainExist() {
			// there are no any known nodes.
			n.OpenBlockchain("Check genesis block")

			bcm := n.NodeBC.GetBCManager()

			geenesisHash, err := bcm.GetGenesisBlockHash()
			n.CloseBlockchain()

			if err == nil {
				// load them from some external resource
				n.NodeNet.LoadInitialNodes(geenesisHash)
			}
		}
	} else {
		n.NodeNet.SetNodes(list, true)
	}
	return nil
}

/*
* Init block maker object. It is used to make new blocks
 */
func (n *Node) initBlockMaker() (*consensus.NodeBlockMaker, error) {
	return consensus.NewConsensusManager(n.DBConn.DB, n.Logger).NewBlockMaker(n.MinterAddress), nil
}

// Open Blockchain  DB. This must be called before any operation with blockchain or cached data
func (n *Node) OpenBlockchain(reason string) error {
	err := n.DBConn.OpenConnection(reason)

	if err != nil {
		return err
	}

	return nil
}

// Closes Blockchain DB connection
func (n *Node) CloseBlockchain() {
	n.DBConn.CloseConnection()
}

/*
* Check if blockchain already exists. If no, we will not allow most of operations
* It is needed to create it first
 */
func (n *Node) BlockchainExist() bool {
	n.DBConn.OpenConnection("checkBCexists")
	exists, _ := n.DBConn.DB.CheckDBExists()
	n.DBConn.CloseConnection()
	return exists
}

/*
* Create new blockchain, add genesis block witha given text
 */
func (n *Node) CreateBlockchain(address, genesisCoinbaseData string) error {
	genesisBlock, err := n.NodeBC.PrepareGenesisBlock(address, genesisCoinbaseData)

	if err != nil {
		return err
	}

	Minter, _ := n.initBlockMaker()

	n.Logger.Trace.Printf("Complete genesis block proof of work\n")

	err = Minter.CompleteBlock(genesisBlock)

	if err != nil {
		return err
	}

	n.Logger.Trace.Printf("Block ready. Init block chain file\n")

	err = n.NodeBC.CreateBlockchain(genesisBlock)

	if err != nil {
		return err
	}
	n.Logger.Trace.Printf("Prepare TX caches\n")

	n.DBConn.OpenConnection("InitAfterCreate")

	n.GetTransactionsManager().BlockAdddedToTop(genesisBlock)

	n.CloseBlockchain()

	n.Logger.Trace.Printf("Blockchain ready!\n")

	return nil
}

// Creates new blockchain DB from given list of blocks
// This would be used when new empty node started and syncs with other nodes

func (n *Node) InitBlockchainFromOther(host string, port int) (bool, error) {
	if host == "" {
		// load node from special hardcoded url
		n.NodeNet.LoadInitialNodes(nil)
		// get node from known nodes
		if len(n.NodeNet.Nodes) == 0 {

			return false, errors.New("No known nodes to request a blockchain")
		}
		nd := n.NodeNet.Nodes[rand.Intn(len(n.NodeNet.Nodes))]

		host = nd.Host
		port = nd.Port
	}
	addr := net.NodeAddr{host, port}
	n.Logger.Trace.Printf("Try to init blockchain from %s:%d", addr.Host, addr.Port)

	result, err := n.NodeClient.SendGetFirstBlocks(addr)

	if err != nil {
		return false, err
	}

	if len(result.Blocks) == 0 {
		return false, errors.New("No blocks found on taht node")
	}

	firstblockbytes := result.Blocks[0]

	block := &blockchain.Block{}
	err = block.DeserializeBlock(firstblockbytes)

	if err != nil {
		return false, err
	}
	// make blockchain with single block
	err = n.NodeBC.CreateBlockchain(block)

	if err != nil {
		return false, err
	}
	// open block chain now
	n.OpenBlockchain("InitAfterImport")

	n.GetTransactionsManager().BlockAdddedToTop(block)

	MH := block.Height

	if len(result.Blocks) > 1 {
		// add all blocks

		skip := true
		for _, blockdata := range result.Blocks {
			if skip {
				skip = false
				continue
			}
			// add this block
			block := &blockchain.Block{}
			err := block.DeserializeBlock(blockdata)

			if err != nil {
				return false, err
			}

			_, err = n.NodeBC.AddBlock(block)

			if err != nil {
				return false, err
			}

			n.GetTransactionsManager().BlockAdddedToTop(block)

			MH = block.Height
		}
	}

	n.CloseBlockchain()

	// add that node to list of known nodes.
	n.NodeNet.AddNodeToKnown(addr)

	return MH == result.Height, nil
}

/*
* Send transaction to all known nodes. This wil send only hash and node hash to check if hash exists or no
 */
func (n *Node) SendTransactionToAll(tx *transaction.Transaction) {
	n.Logger.Trace.Printf("Send transaction to %d nodes", len(n.NodeNet.Nodes))

	for _, node := range n.NodeNet.Nodes {
		if node.CompareToAddress(n.NodeClient.NodeAddress) {
			continue
		}
		n.Logger.Trace.Printf("Send TX %x to %s", tx.ID, node.NodeAddrToString())
		n.NodeClient.SendInv(node, "tx", [][]byte{tx.ID})
	}
}

// Send block to all known nodes
// This is used in case when new block was received from other node or
// created by this node. We will notify our network about new block
// But not send full block, only hash and previous hash. So, other can copy it
// Address from where we get it will be skipped
func (n *Node) SendBlockToAll(newBlock *blockchain.Block, skipaddr net.NodeAddr) {
	for _, node := range n.NodeNet.Nodes {
		if node.CompareToAddress(n.NodeClient.NodeAddress) {
			continue
		}
		blockshortdata, err := newBlock.GetShortCopy().Serialize()
		if err == nil {
			n.NodeClient.SendInv(node, "block", [][]byte{blockshortdata})
		}
	}
}

/*
* Send own version to all known nodes
 */
func (n *Node) SendVersionToNodes(nodes []net.NodeAddr) {
	bestHeight, err := n.NodeBC.GetBestHeight()

	if err != nil {
		return
	}

	if len(nodes) == 0 {
		nodes = n.NodeNet.Nodes
	}

	for _, node := range nodes {
		if node.CompareToAddress(n.NodeClient.NodeAddress) {
			continue
		}
		n.NodeClient.SendVersion(node, bestHeight)
	}
}

/*
* Check if the address is known . If not then add to known
* and send list of all addresses to that node
 */
func (n *Node) CheckAddressKnown(addr net.NodeAddr) {
	if !n.NodeNet.CheckIsKnown(addr) {
		// send him all addresses
		n.Logger.Trace.Printf("sending list of address to %s , %s", addr.NodeAddrToString(), n.NodeNet.Nodes)
		n.NodeClient.SendAddrList(addr, n.NodeNet.Nodes)

		n.NodeNet.AddNodeToKnown(addr)
	}
}

/*
* Send money .
* This adds a transaction directly to the DB. Can be executed when a node server is not running
 */
func (n *Node) Send(PubKey []byte, privKey ecdsa.PrivateKey, to string, amount float64) ([]byte, error) {
	// get pubkey of the wallet with "from" address

	tx, err := n.GetTransactionsManager().Send(PubKey, privKey, to, amount)

	if err != nil {
		return nil, err
	}
	n.SendTransactionToAll(tx)

	return tx.ID, nil
}

/*
* Try to make a block. Check if there are enough transactions in list of unapproved
 */
func (n *Node) TryToMakeBlock() ([]byte, error) {
	n.Logger.Trace.Println("Try to make new block")

	w := wallet.Wallet{}

	if n.MinterAddress == "" || !w.ValidateAddress(n.MinterAddress) {
		return nil, errors.New("Minter address is not provided")
	}

	err := n.OpenBlockchain("TryToMakeBlock1")

	if err != nil {
		return nil, err
	}

	defer n.CloseBlockchain()

	n.Logger.Trace.Println("Create block maker")
	// check how many transactions are ready to be added to a block
	Minter, _ := n.initBlockMaker()

	// check how many transactions are ready to be added to a block
	if !Minter.CheckUnapprovedCache() {
		n.Logger.Trace.Println("No anough transactions for block")

		return nil, nil
	}

	if !Minter.CheckGoodTimeToMakeBlock() {
		n.Logger.Trace.Println("Not good time to do a block")
		return nil, nil
	}
	n.Logger.Trace.Println("Prepare new bock making")
	// makes a block transactions list. This will not yet have a block hash
	// can return nil if no enough transactions to make a block
	blockorig, err := Minter.PrepareNewBlock()

	if err != nil {
		return []byte{}, err
	}

	var block *blockchain.Block
	block = nil

	if blockorig != nil {
		n.Logger.Trace.Printf("Prepared new block with %d transactions", len(blockorig.Transactions))
		// do copy of a block. as original can contains some references to the DB and it will be closed
		block = blockorig.Copy()
	}

	// close it while doing the proof of work
	n.CloseBlockchain()

	// complete the block. add proof of work
	if block != nil {

		n.Logger.Trace.Printf("Prepared new block with %d transactions", len(block.Transactions))

		// this will do MINING of a block

		err := Minter.CompleteBlock(block)

		if err != nil {
			return nil, err
		}

		n.Logger.Trace.Printf("Add block to the blockchain. Hash %x\n", block.Hash)

		// open BC DB again
		n.OpenBlockchain("AddNewMadeBlock")
		Minter.SetDBManager(n.DBConn.DB)

		// final correction. while we did minting, there can be something changes
		// some other block could be added to the blockchain in parallel process

		err = Minter.FinalBlockCheck(block)

		if err != nil {
			n.Logger.Trace.Printf("Final check is not passed. Error: %s", err.Error())
			return nil, err
		}

		// add new block to local blockchain. this will check a block again
		// TODO we need to skip checking. no sense, we did it right
		_, err = n.AddBlock(block)

		if err != nil {
			return nil, err
		}
		// send new block to all known nodes
		n.SendBlockToAll(block, net.NodeAddr{} /*nothing to skip*/)

		n.Logger.Trace.Println("Block done. Sent to all")

		return block.Hash, nil
	}
	n.Logger.Trace.Println("No anough transactions to make a block")
	return nil, nil
}

// Add new block to blockchain.
// It can be executed when new block was created locally or received from other node

func (n *Node) AddBlock(block *blockchain.Block) (uint, error) {
	bcm, err := n.GetBCManager()

	if err != nil {
		return 0, err
	}

	curLastHash, _, err := bcm.GetState()

	// we need to know how the block was added to managed transactions caches correctly
	addstate, err := n.NodeBC.AddBlock(block)

	if err != nil {
		return 0, err
	}

	if addstate == blockchain.BCBAddState_addedToTop ||
		addstate == blockchain.BCBAddState_addedToParallelTop ||
		addstate == blockchain.BCBAddState_addedToParallel {
		// block was added. add transactions to caches
		n.GetTransactionsManager().GetIndexManager().BlockAdded(block)
	}

	if addstate == blockchain.BCBAddState_addedToTop {
		// only if a block was really added
		// delete block transaction from list of unapproved
		n.GetTransactionsManager().GetUnapprovedTransactionsManager().DeleteFromBlock(block)

		// reindes unspent transactions cache based on block added
		n.GetTransactionsManager().GetUnspentOutputsManager().UpdateOnBlockAdd(block)

	} else if addstate == blockchain.BCBAddState_addedToParallelTop {
		// get 2 blocks branches that replaced each other
		newChain, oldChain, err := n.NodeBC.GetBranchesReplacement(curLastHash, []byte{})

		if err != nil {
			return 0, err
		}

		if newChain != nil && oldChain != nil {
			// add old blocks back to unspent tranactions
			n.GetTransactionsManager().GetUnspentOutputsManager().UpdateOnBlocksCancel(oldChain)
			// remove new blocks from unspent transactions
			n.GetTransactionsManager().GetUnspentOutputsManager().UpdateOnBlocksAdd(newChain)

			// transactions from old blocks become unapproved
			n.GetTransactionsManager().GetUnapprovedTransactionsManager().AddFromBlocksCancel(oldChain)
			// transactions from new chain becomes approved
			n.GetTransactionsManager().GetUnapprovedTransactionsManager().DeleteFromBlocks(newChain)
		}
	}

	return addstate, nil
}

/*
* Drop block from the top of blockchain
* This will not check if there are other branch that can now be longest and becomes main branch
 */
func (n *Node) DropBlock() error {
	block, err := n.NodeBC.DropBlock()

	if err != nil {
		return err
	}

	n.GetTransactionsManager().GetUnapprovedTransactionsManager().AddFromCanceled(block.Transactions)

	n.GetTransactionsManager().GetUnspentOutputsManager().UpdateOnBlockCancel(block)

	return nil
}

// New block info received from oher node. It is only Hash and PrevHash, not full block
// Check if this is new block and if previous block is fine
// returns state of processing. if a block data was requested or exists or prev doesn't exist
func (n *Node) ReceivedBlockFromOtherNode(addrfrom net.NodeAddr, bsdata []byte) (int, error) {

	bs := &blockchain.BlockShort{}
	err := bs.DeserializeBlock(bsdata)

	if err != nil {
		return 0, err
	}
	// check if block exists
	blockstate, err := n.NodeBC.CheckBlockState(bs.Hash, bs.PrevBlockHash)

	if err != nil {
		return 0, err
	}

	if blockstate == 0 {
		// in this case we can request this block full info
		n.NodeClient.SendGetData(addrfrom, "block", bs.Hash)
		return 0, nil // 0 means a block can be added and now we requested info about it
	}
	return blockstate, nil
}

/*
* New block info received from oher node
* Check if this is new block and if previous block is fine
* returns state of processing. if a block data was requested or exists or prev doesn't exist
 */
func (n *Node) ReceivedFullBlockFromOtherNode(blockdata []byte) (int, uint, *blockchain.Block, error) {
	addstate := uint(blockchain.BCBAddState_error)

	block := &blockchain.Block{}
	err := block.DeserializeBlock(blockdata)

	if err != nil {
		return -1, addstate, nil, err
	}

	n.Logger.Trace.Printf("Recevied a new block %x", block.Hash)

	// check state of this block
	blockstate, err := n.NodeBC.CheckBlockState(block.Hash, block.PrevBlockHash)

	if err != nil {
		return 0, addstate, nil, err
	}

	if blockstate == 0 {
		// only in this case we can add a block!
		// addblock should also verify the block
		addstate, err = n.AddBlock(block)

		if err != nil {
			return -1, addstate, nil, err
		}
		n.Logger.Trace.Printf("Added block %x\n", block.Hash)
	} else {
		n.Logger.Trace.Printf("Block can not be added. State is %d\n", blockstate)
	}
	return blockstate, addstate, block, nil
}

// Get node state

func (n *Node) GetNodeState() (nodeclient.ComGetNodeState, error) {
	result := nodeclient.ComGetNodeState{}

	result.ExpectingBlocksHeight = 0

	err := n.OpenBlockchain("ShowState")

	if err != nil {
		return result, err
	}
	defer n.CloseBlockchain()

	bh, err := n.NodeBC.GetBestHeight()

	if err != nil {
		return result, err
	}
	result.BlocksNumber = bh + 1

	unappr, err := n.GetTransactionsManager().GetUnapprovedTransactionsManager().GetCount()

	if err != nil {
		return result, err
	}

	result.TransactionsCached = unappr

	unspent, err := n.GetTransactionsManager().GetUnspentOutputsManager().CountUnspentOutputs()

	if err != nil {
		return result, err
	}

	result.UnspentOutputs = unspent

	return result, nil
}
