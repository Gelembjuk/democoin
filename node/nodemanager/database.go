package nodemanager

import (
	"runtime/debug"

	"github.com/gelembjuk/democoin/lib/utils"
	"github.com/gelembjuk/democoin/node/database"
)

type Database struct {
	DB     database.DBManager
	Logger *utils.LoggerMan
	Config database.DatabaseConfig
}

func (db *Database) Clone() Database {
	ndb := Database{}
	ndb.SetLogger(db.Logger)
	ndb.SetConfig(db.Config)

	return ndb
}

func (db *Database) SetLogger(Logger *utils.LoggerMan) {
	db.Logger = Logger
}

func (db *Database) SetConfig(config database.DatabaseConfig) {
	db.Config = config
}

func (db *Database) OpenConnection(reason string) error {
	db.Logger.Trace.Printf("OpenConn in DB man %s", reason)

	if db.DB != nil {
		db.Logger.Trace.Printf("OpenConn connection is already open. ERROR")
		debug.PrintStack()
	}
	db.PrepareConnection()
	return db.DB.OpenConnection(reason)
}

func (db *Database) PrepareConnection() {
	db.DB = &database.BoltDBManager{}
	db.DB.SetLogger(db.Logger)
	db.DB.SetConfig(db.Config)
}

func (db *Database) CloseConnection() error {
	db.Logger.Trace.Printf("CloseConn")
	if db.DB == nil {
		db.Logger.Trace.Printf("Already closed. ERROR")
		return nil
	}

	db.DB.CloseConnection()

	db.CleanConnection()

	return nil
}

func (db *Database) CleanConnection() {
	db.DB = nil
}

func (db *Database) OpenConnectionIfNeeded(reason string) bool {
	if db.DB != nil {
		return false
	}

	err := db.OpenConnection(reason)

	if err != nil {
		return false
	}

	return true
}
