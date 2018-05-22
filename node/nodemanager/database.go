package nodemanager

import (
	"runtime/debug"
	"sync"

	"github.com/gelembjuk/democoin/lib/utils"
	"github.com/gelembjuk/democoin/node/database"
)

type Database struct {
	DB        database.DBManager
	Logger    *utils.LoggerMan
	Config    database.DatabaseConfig
	lockerObj database.DatabaseLocker
	locallock *sync.Mutex
}

// do initial actions
func (db *Database) Init() {
	// init locking system. one object will be used
	// in all goroutines

	db.locallock = &sync.Mutex{}
	db.PrepareConnection("")
	db.lockerObj = db.DB.GetLockerObject()
	db.CleanConnection()
}

func (db *Database) Clone() Database {
	ndb := Database{}
	ndb.locallock = &sync.Mutex{}
	ndb.SetLogger(db.Logger)
	ndb.SetConfig(db.Config)
	ndb.lockerObj = db.lockerObj

	return ndb
}

func (db *Database) SetLogger(Logger *utils.LoggerMan) {
	db.Logger = Logger
}

func (db *Database) SetConfig(config database.DatabaseConfig) {
	db.Config = config
}

func (db *Database) OpenConnection(reason string, sessid string) error {
	db.Logger.Trace.Printf("OpenConn in DB man %s", reason)

	if db.DB != nil {
		db.Logger.Trace.Printf("OpenConn connection is already open. ERROR")
		debug.PrintStack()
	}
	db.PrepareConnection(sessid)

	// this will prevent creation of this object from other go routine
	db.locallock.Lock()

	return db.DB.OpenConnection(reason)
}

func (db *Database) PrepareConnection(sessid string) {
	obj := &database.BoltDBManager{}
	obj.SessID = sessid
	db.DB = obj
	db.DB.SetLogger(db.Logger)
	db.DB.SetConfig(db.Config)

	if db.lockerObj != nil {
		db.DB.SetLockerObject(db.lockerObj)
	}
}

func (db *Database) CloseConnection() error {
	db.Logger.Trace.Printf("CloseConn")
	if db.DB == nil {
		db.Logger.Trace.Printf("Already closed. ERROR")
		return nil
	}
	// now allow other go routine to create connection using same object
	db.locallock.Unlock()

	db.DB.CloseConnection()

	db.CleanConnection()

	return nil
}

func (db *Database) CleanConnection() {
	db.DB = nil
}

func (db *Database) OpenConnectionIfNeeded(reason string, sessid string) bool {
	if db.DB != nil {
		return false
	}

	err := db.OpenConnection(reason, sessid)

	if err != nil {
		return false
	}

	return true
}

func (db *Database) CloseConnectionIfNeeded() {
	if db.DB != nil {
		db.CloseConnection()
	}
}
