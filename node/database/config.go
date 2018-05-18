package database

type DatabaseConfig struct {
	DataDir        string
	BlockchainFile string
	NodesFile      string
}

func (dbc *DatabaseConfig) IsEmpty() bool {
	if dbc.BlockchainFile == "" || dbc.NodesFile == "" {
		return true
	}
	return false
}

func (dbc *DatabaseConfig) SetDefault() error {
	dbc.BlockchainFile = "blockchain.db"
	dbc.NodesFile = "nodeslist.db"
	return nil
}
