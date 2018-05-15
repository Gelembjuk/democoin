package consensus

import (
	"github.com/gelembjuk/democoin/lib/utils"
	"github.com/gelembjuk/democoin/node/database"
)

type Consensus struct {
	DB     database.DBManager
	Logger *utils.LoggerMan
}

// Get consensus management object.

func NewConsensusManager(DB database.DBManager, Logger *utils.LoggerMan) *Consensus {

	return &Consensus{DB, Logger}
}

func (c *Consensus) NewBlockMaker(minter string) *NodeBlockMaker {
	return &NodeBlockMaker{c.DB, c.Logger, minter}

}
