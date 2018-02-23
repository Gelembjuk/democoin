package main

import (
	"crypto/ecdsa"
	"errors"
	"math/rand"
	"time"

	"github.com/gelembjuk/democoin/lib"
	"github.com/gelembjuk/democoin/lib/nodeclient"
	"github.com/gelembjuk/democoin/lib/transaction"
	"github.com/gelembjuk/democoin/lib/wallet"
)

/*
* This structure is central part of the application. only it can acces to blockchain and inside it all operation are done
 */

type Node struct {
	NodeBC        NodeBlockchain
	NodeTX        NodeTransactions
	NodeNet       lib.NodeNetwork
	Logger        *lib.LoggerMan
	DataDir       string
	MinterAddress string
	NodeClient    *nodeclient.NodeClient
	OtherNodes    []lib.NodeAddr
}

/*
* Init node.
* Init interfaces of all DBs, blockchain, unspent transactions, unapproved transactions
 */
func (n *Node) Init() {
	n.NodeNet.Logger = n.Logger
	n.NodeBC.Logger = n.Logger

	n.NodeTX.DataDir = n.DataDir
	n.NodeTX.Logger = n.Logger
	n.NodeTX.UnapprovedTXs.Logger = n.Logger
	n.NodeTX.UnspentTXs.Logger = n.Logger

	n.NodeBC.DataDir = n.DataDir
	n.NodeBC.MinterAddress = n.MinterAddress
	// load list of nodes from config
	n.NodeNet.LoadNodes([]lib.NodeAddr{}, true)

	n.NodeBC.NodeTX = &n.NodeTX

	n.InitClient()

	rand.Seed(time.Now().UTC().UnixNano())
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
func (n *Node) InitNodes(list []lib.NodeAddr) error {
	if len(list) == 0 {
		if n.NodeNet.GetCountOfKnownNodes() == 0 {
			// there are no any known nodes.
			// load them from some external resource
			n.NodeNet.LoadInitialNodes()
		}
	} else {
		n.NodeNet.LoadNodes(list, true)
	}
	return nil
}

/*
* Init block maker object. It is used to make new blocks
 */
func (n *Node) initBlockMaker() (*NodeBlockMaker, error) {
	Minter := &NodeBlockMaker{}
	Minter.MinterAddress = n.MinterAddress
	Minter.BC = n.NodeBC.BC
	Minter.Logger = n.Logger
	Minter.NodeTX = &n.NodeTX

	return Minter, nil
}

// Open Blockchain  DB. This must be called before any operation with blockchain or cached data

func (n *Node) OpenBlockchain() error {
	err := n.NodeBC.OpenBlockchain()

	if err != nil {
		return err
	}
	n.NodeTX.BC = n.NodeBC.BC
	n.NodeTX.UnspentTXs.SetBlockchain(n.NodeBC.BC)
	n.NodeTX.UnapprovedTXs.SetBlockchain(n.NodeBC.BC)

	return nil
}

/*
* Closes Blockchain DB
 */
func (n *Node) CloseBlockchain() {
	n.NodeBC.CloseBlockchain()

	n.NodeTX.UnspentTXs.SetBlockchain(nil)
	n.NodeTX.UnapprovedTXs.SetBlockchain(nil)
	n.NodeTX.BC = nil
}

/*
* Check if blockchain already exists. If no, we will not allow most of operations
* It is needed to create it first
 */
func (n *Node) BlockchainExist() bool {
	if n.NodeBC.BC == nil {
		err := n.OpenBlockchain()

		if err != nil {
			return false
		}

		defer n.CloseBlockchain()
	}
	// block chain DB is opened. check if there is any block
	_, _, err := n.NodeBC.BC.GetState()

	if err != nil {
		// DB exists but it is empty or broken
		return false
	}

	return true
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

	n.OpenBlockchain()
	n.NodeTX.UnspentTXs.Reindex()
	n.NodeTX.UnapprovedTXs.InitDB()
	n.CloseBlockchain()

	n.Logger.Trace.Printf("Blockchain ready!\n")

	return nil
}

/*
* Load start blocks of blockchain from other node.
 */
func (n *Node) InitBlockchainFromOther(host string, port int) (bool, error) {
	if host == "" {
		// get node from known nodes
		if len(n.NodeNet.Nodes) == 0 {
			return false, errors.New("No known nodes to request a blockchain")
		}
		nd := n.NodeNet.Nodes[rand.Intn(len(n.NodeNet.Nodes))]

		host = nd.Host
		port = nd.Port
	}
	addr := lib.NodeAddr{host, port}
	n.Logger.Trace.Printf("Try to init blockchain from %s:%d", addr.Host, addr.Port)

	result, err := n.NodeClient.SendGetFirstBlocks(addr)

	if err != nil {
		return false, err
	}

	MH, err := n.NodeBC.CreateBlockchainFromBlocks(result.Blocks)

	if err != nil {
		return false, err
	}

	// init DB for unspent and unapproved transactions
	n.OpenBlockchain()

	n.NodeTX.UnspentTXs.Reindex()
	n.NodeTX.UnapprovedTXs.InitDB()

	n.CloseBlockchain()

	return MH == result.Height, nil
}

/*
* Send transaction to all known nodes. This wil send only hash and node hash to check if hash exists or no
 */
func (n *Node) SendTransactionToAll(tx *transaction.Transaction) {
	n.Logger.Trace.Printf("Send transaction to %d nodes", len(n.NodeNet.Nodes))

	for _, node := range n.NodeNet.Nodes {
		n.Logger.Trace.Printf("Send TX %x to %s", tx.ID, node.NodeAddrToString())
		n.NodeClient.SendInv(node, "tx", [][]byte{tx.ID})
	}
}

/*
* Send block to all known nodes
 */
func (n *Node) SendBlockToAll(newBlock *Block) {
	for _, node := range n.NodeNet.Nodes {
		blockshortdata, err := newBlock.GetShortCopy().Serialize()
		if err == nil {
			n.NodeClient.SendInv(node, "block", [][]byte{blockshortdata})
		}

	}
}

/*
* Send own version to all known nodes
 */
func (n *Node) SendVersionToNodes(nodes []lib.NodeAddr) {
	bestHeight, err := n.NodeBC.GetBestHeight()

	if err != nil {
		return
	}

	if len(nodes) == 0 {
		nodes = n.NodeNet.Nodes
	}

	for _, node := range nodes {
		n.NodeClient.SendVersion(node, bestHeight)
	}
}

/*
* Check if the address is known . If not then add to known
* and send list of all addresses to that node
 */
func (n *Node) CheckAddressKnown(addr lib.NodeAddr) {
	if !n.NodeNet.CheckIsKnown(addr) {
		// send him all addresses
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

	tx, err := n.NodeTX.Send(PubKey, privKey, to, amount)

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

	err := n.OpenBlockchain()

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

	var block *Block
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
		n.OpenBlockchain()

		// add new block to local blockchain. this will check a block again
		// TODO we need to skip checking. no sense, we did it right
		err = n.AddBlock(block)

		if err != nil {
			return nil, err
		}
		// send new block to all known nodes
		n.SendBlockToAll(block)

		n.Logger.Trace.Println("Block done. Sent to all")

		return block.Hash, nil
	}
	n.Logger.Trace.Println("No anough transactions to make a block")
	return nil, nil
}

// Add new block to blockchain.
// It can be executed when new block was created locally or received from other node

func (n *Node) AddBlock(block *Block) error {
	curLastHash, _, err := n.NodeBC.BC.GetState()

	// we need to know how the block was added to managed transactions caches correctly
	addstate, err := n.NodeBC.AddBlock(block)

	if err != nil {
		return err
	}
	if addstate == BCBAddState_addedToTop {
		// only if a block was really added
		// delete block transaction from list of unapproved
		n.NodeTX.UnapprovedTXs.DeleteFromBlock(block)

		// reindes unspent transactions cache based on block added
		n.NodeTX.UnspentTXs.UpdateOnBlockAdd(block)

	} else if addstate == BCBAddState_addedToParallelTop {
		// get 2 blocks branches that replaced each other
		newChain, oldChain, err := n.NodeBC.GetBranchesReplacement(curLastHash)

		if err != nil {
			return err
		}

		if newChain != nil && oldChain != nil {
			// add old blocks back to unspent tranactions
			n.NodeTX.UnspentTXs.UpdateOnBlocksCancel(oldChain)
			// remove new blocks from unspent transactions
			n.NodeTX.UnspentTXs.UpdateOnBlocksAdd(newChain)

			// transactions from old blocks become unapproved
			n.NodeTX.UnapprovedTXs.AddFromBlocksCancel(oldChain)
			// transactions from new chain becomes approved
			n.NodeTX.UnapprovedTXs.DeleteFromBlocks(newChain)
		}
	}

	return nil
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

	n.NodeTX.UnapprovedTXs.AddFromCanceled(block.Transactions)

	n.NodeTX.UnspentTXs.UpdateOnBlockCancel(block)

	return nil
}

// New block info received from oher node. It is only Hash and PrevHash, not full block
// Check if this is new block and if previous block is fine
// returns state of processing. if a block data was requested or exists or prev doesn't exist
func (n *Node) ReceivedBlockFromOtherNode(addrfrom lib.NodeAddr, bsdata []byte) (int, error) {

	bs := &BlockShort{}
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
func (n *Node) ReceivedFullBlockFromOtherNode(blockdata []byte) (int, error) {
	block := &Block{}
	err := block.DeserializeBlock(blockdata)

	if err != nil {
		return -1, err
	}

	n.Logger.Trace.Println("Recevied a new block!")

	// check state of this block
	blockstate, err := n.NodeBC.CheckBlockState(block.Hash, block.PrevBlockHash)

	if err != nil {
		return 0, err
	}

	if blockstate == 0 {
		// only in this case we can add a block!
		// addblock should also verify the block
		err = n.AddBlock(block)

		if err != nil {
			return -1, err
		}
		n.Logger.Trace.Printf("Added block %x\n", block.Hash)
	} else {
		n.Logger.Trace.Printf("Block can not be added. State is %d\n", blockstate)
	}
	return blockstate, nil
}
