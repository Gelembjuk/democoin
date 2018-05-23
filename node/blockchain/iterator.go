package blockchain

import (
	"github.com/gelembjuk/democoin/node/database"
	"github.com/gelembjuk/democoin/node/structures"
)

// BlockchainIterator is used to iterate over blockchain blocks
type BlockchainIterator struct {
	currentHash []byte
	DB          database.DBManager
}

// Next returns next block starting from the tip
func (i *BlockchainIterator) Next() (*structures.Block, error) {
	var block *structures.Block

	bcdb, err := i.DB.GetBlockchainObject()

	if err != nil {
		return nil, err
	}

	encodedBlock, err := bcdb.GetBlock(i.currentHash)

	if err != nil {
		return nil, err
	}

	block = &structures.Block{}
	err = block.DeserializeBlock(encodedBlock)

	if err != nil {
		return nil, err
	}

	i.currentHash = block.PrevBlockHash

	return block, nil
}

// Creates new Blockchain Iterator . Can be used to do something with blockchain from outside

func NewBlockchainIterator(DB database.DBManager) (*BlockchainIterator, error) {

	bcdb, err := DB.GetBlockchainObject()

	if err != nil {
		return nil, err
	}

	starttip, err := bcdb.GetTopHash()

	if err != nil {
		return nil, err
	}

	return &BlockchainIterator{starttip, DB}, nil
}

// Creates new Blockchain Iterator from given block hash. Can be used to do something with blockchain from outside
//
func NewBlockchainIteratorFrom(DB database.DBManager, startHash []byte) (*BlockchainIterator, error) {
	return &BlockchainIterator{startHash, DB}, nil
}
