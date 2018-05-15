package nodemanager

import (
	"github.com/gelembjuk/democoin/lib/utils"
	"github.com/gelembjuk/democoin/node/database"
)

type Database struct {
	DB      database.DBManager
	Logger  *utils.LoggerMan
	DataDir string
	Config  database.DatabaseConfig
}

func (db *Database) SetLogger(Logger *utils.LoggerMan) {
	db.Logger = Logger
}

func (db *Database) SetConfig(config database.DatabaseConfig) {
	db.Config = config
}

func (db *Database) OpenConnection(reason string) error {
	db.PrepareConnection()
	return db.DB.OpenConnection(reason)
}

func (db *Database) PrepareConnection() {
	db.DB = &database.BoltDBManager{}
	db.DB.SetLogger(db.Logger)
	db.DB.SetConfig(db.Config)
}

func (db *Database) CloseConnection() error {
	if db.DB == nil {
		return nil
	}

	db.DB.CloseConnection()

	db.CleanConnection()

	return nil
}

func (db *Database) CleanConnection() {
	db.DB = nil
}
