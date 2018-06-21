package database

import (
	"errors"
	"fmt"
	"os"
	"path"

	"strconv"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gelembjuk/democoin/lib/utils"
)

const (
	ClassNameNodes                  = "nodes"
	ClassNameBlockchain             = "blockchain"
	ClassNameTransactions           = "transactions"
	ClassNameUnapprovedTransactions = "unapprovedtransactions"
	ClassNameUnspentOutputs         = "unspentoutputs"
)

type BoltDBManager struct {
	Logger     *utils.LoggerMan
	Config     DatabaseConfig
	connBC     *BoltDB
	connNodes  *BoltDB
	openedConn bool
	locker     *BoltDBLocker
	SessID     string
}

type BoltDBLocker struct {
	lockBC    *sync.Mutex
	lockNodes *sync.Mutex
}

func (bdm *BoltDBManager) GetLockerObject() DatabaseLocker {
	locker := &BoltDBLocker{}
	locker.lockBC = &sync.Mutex{}
	locker.lockNodes = &sync.Mutex{}

	return locker
}

func (bdm *BoltDBManager) SetLockerObject(lockerobj DatabaseLocker) {
	bdm.locker = lockerobj.(*BoltDBLocker)
}
func (bdm *BoltDBManager) SetConfig(config DatabaseConfig) error {
	bdm.Config = config

	return nil
}
func (bdm *BoltDBManager) SetLogger(logger *utils.LoggerMan) error {
	bdm.Logger = logger

	return nil
}

func (bdm *BoltDBManager) OpenConnection(reason string) error {
	//bdm.Logger.Trace.Println("open connection for " + reason)
	if bdm.openedConn {
		return nil
	}
	// real connection will be done when first object is created
	bdm.openedConn = true

	bdm.connBC = nil
	bdm.connNodes = nil

	return nil
}
func (bdm *BoltDBManager) CloseConnection() error {
	if !bdm.openedConn {
		return nil
	}

	if bdm.connBC != nil {
		bdm.connBC.Close()
		bdm.unLockDB(bdm.connBC.lockFile)
		bdm.connBC = nil
	}
	if bdm.connNodes != nil {
		bdm.connNodes.Close()
		bdm.unLockDB(bdm.connNodes.lockFile)
		bdm.connNodes = nil
	}

	bdm.openedConn = false
	return nil
}

func (bdm *BoltDBManager) IsConnectionOpen() bool {
	return bdm.openedConn
}

// create empty database. must create all
func (bdm *BoltDBManager) InitDatabase() error {

	bdm.OpenConnection("InitBC")

	defer bdm.CloseConnection()

	_, err := bdm.getConnectionForObjectWithCheck(ClassNameBlockchain, true)

	if err != nil {
		return err
	}

	_, err = bdm.getConnectionForObjectWithCheck(ClassNameNodes, true)

	if err != nil {
		return err
	}

	bc, err := bdm.GetBlockchainObject()

	if err != nil {
		return err
	}

	err = bc.InitDB()

	if err != nil {
		return err
	}

	txs, err := bdm.GetTransactionsObject()

	if err != nil {
		return err
	}

	err = txs.InitDB()

	if err != nil {
		return err
	}

	utx, err := bdm.GetUnapprovedTransactionsObject()

	if err != nil {
		return err
	}

	err = utx.InitDB()

	if err != nil {
		return err
	}

	uos, err := bdm.GetUnspentOutputsObject()

	if err != nil {
		return err
	}

	err = uos.InitDB()

	if err != nil {
		return err
	}

	ns, err := bdm.GetNodesObject()

	if err != nil {
		return err
	}

	err = ns.InitDB()

	if err != nil {
		return err
	}

	return nil
}

// Check if database was already inited
func (bdm *BoltDBManager) CheckDBExists() (bool, error) {
	bc, err := bdm.GetBlockchainObject()

	if err != nil {
		return false, nil
	}

	tophash, err := bc.GetTopHash()

	if err != nil {
		return false, nil
	}

	if len(tophash) > 0 {
		return true, nil
	}

	return false, nil
}

// returns BlockChain Database structure. does al init
func (bdm *BoltDBManager) GetBlockchainObject() (BlockchainInterface, error) {
	conn, err := bdm.getConnectionForObject(ClassNameBlockchain)

	if err != nil {
		return nil, err
	}

	bc := Blockchain{}
	bc.DB = conn

	return &bc, nil
}

// returns Transaction Index Database structure. does al init
func (bdm *BoltDBManager) GetTransactionsObject() (TranactionsInterface, error) {
	conn, err := bdm.getConnectionForObject(ClassNameTransactions)

	if err != nil {
		return nil, err
	}

	txs := Tranactions{}
	txs.DB = conn

	return &txs, nil
}

// returns Unapproved Transaction Database structure. does al init
func (bdm *BoltDBManager) GetUnapprovedTransactionsObject() (UnapprovedTransactionsInterface, error) {
	conn, err := bdm.getConnectionForObject(ClassNameUnspentOutputs)

	if err != nil {
		return nil, err
	}

	uos := UnapprovedTransactions{}
	uos.DB = conn

	return &uos, nil
}

// returns Unspent Transactions Database structure. does al init
func (bdm *BoltDBManager) GetUnspentOutputsObject() (UnspentOutputsInterface, error) {
	conn, err := bdm.getConnectionForObject(ClassNameUnapprovedTransactions)

	if err != nil {
		return nil, err
	}

	uts := UnspentOutputs{}
	uts.DB = conn

	return &uts, nil
}

// returns Nodes Database structure. does al init
func (bdm *BoltDBManager) GetNodesObject() (NodesInterface, error) {
	conn, err := bdm.getConnectionForObject(ClassNameNodes)

	if err != nil {
		return nil, err
	}

	ns := Nodes{}
	ns.DB = conn

	return &ns, nil
}

// returns
func (bdm *BoltDBManager) getConnectionForObject(name string) (*BoltDB, error) {
	return bdm.getConnectionForObjectWithCheck(name, false)
}

// returns DB connection, creates it if needed .
func (bdm *BoltDBManager) getConnectionForObjectWithCheck(name string, ignoremissed bool) (*BoltDB, error) {
	if !bdm.openedConn {
		return nil, errors.New("Connection was not inited")
	}

	if bdm.isBCDB(name) && bdm.connBC != nil {
		//bdm.Logger.Trace.Println("bc connection exists. rteurn it")
		return bdm.connBC, nil
	}

	if bdm.isNodesDB(name) && bdm.connNodes != nil {
		return bdm.connNodes, nil
	}

	// create new connection
	boltdbfile, err := bdm.getDBFileForObject(name)

	if err != nil {
		return nil, err
	}

	if bdm.dbExists(boltdbfile) == false && !ignoremissed {
		return nil, errors.New(fmt.Sprintf("Database file %s not found", boltdbfile))
	}

	err = bdm.lockDB(name, bdm.SessID)

	if err != nil {
		return nil, err
	}

	db, err := bolt.Open(boltdbfile, 0600, &bolt.Options{Timeout: 10 * time.Second})

	if err != nil {
		bdm.Logger.Trace.Printf("Error opening DB %s for %s", err.Error(), name)
		return nil, err
	}

	// if success create object and assign connection
	boltDB := BoltDB{db, name}

	if bdm.isBCDB(name) {
		bdm.connBC = &boltDB
	}

	if bdm.isNodesDB(name) {
		bdm.connNodes = &boltDB
	}

	return &boltDB, nil
}

// Creates a lock file for DB access. We need this to controll parallel access to the DB
func (bdm *BoltDBManager) lockDB(name string, locksess string) error {
	if locksess == "" {
		locksess = utils.RandString(5)
		//bdm.Logger.Trace.Println(string(debug.Stack()))
	}

	var locker *sync.Mutex

	if bdm.isNodesDB(name) {
		locker = bdm.locker.lockNodes
	} else {
		locker = bdm.locker.lockBC
	}

	locker.Lock()

	lockfile, err := bdm.getDBLockFileForObject(name)

	if err != nil {
		locker.Unlock()
		return err
	}

	i := 0

	info, err := os.Stat(lockfile)

	if err == nil {
		t := time.Since(info.ModTime())

		// this is for case when something goes very wrong , process fails and lock is not removed
		if t.Minutes() > 60 {
			os.Remove(lockfile)
		}
	}

	for bdm.dbExists(lockfile) != false {

		if i > 5000 {
			time.Sleep(1 * time.Second)
		} else {
			time.Sleep(50 * time.Millisecond)
		}

		i++

		if i > 10000 {
			locker.Unlock()
			bdm.Logger.Trace.Println("too long lock. return with error")
			return errors.New("Can not open DB. Lock failed after many attempts")
		}
	}

	file, err := os.Create(lockfile)

	if err != nil {
		locker.Unlock()
		return err
	}

	defer file.Close()

	starttime := time.Now().UTC().UnixNano()

	_, err = file.WriteString(strconv.Itoa(int(starttime)))

	if err != nil {
		locker.Unlock()
		return err
	}

	file.WriteString(" " + locksess)

	file.Sync() // flush to disk

	return nil
}

// Removes DB lock file
func (bdm *BoltDBManager) unLockDB(name string) {

	var locker *sync.Mutex

	if bdm.isNodesDB(name) {
		locker = bdm.locker.lockNodes
	} else {
		locker = bdm.locker.lockBC
	}

	lockfile, err := bdm.getDBLockFileForObject(name)

	if err != nil {
		locker.Unlock()
		return
	}
	if bdm.dbExists(lockfile) != false {
		/*
			lockinfobytes, err := ioutil.ReadFile(lockfile)

			if err == nil {
				lockinfo := string(lockinfobytes)

				parts := strings.Split(lockinfo, " ")

				starttime, err := strconv.Atoi(parts[0])

				if err == nil {
					duration := time.Since(time.Unix(0, int64(starttime)))
					ms := duration.Nanoseconds() / int64(time.Millisecond)
					bdm.Logger.Trace.Printf("UnLocked %s after %d ms , %s", name, ms, parts[1])
				}
			}
		*/
		os.Remove(lockfile)
	}
	locker.Unlock()
}
func (bdm *BoltDBManager) getDBFileForObject(name string) (string, error) {
	switch name {
	case ClassNameNodes:
		return bdm.Config.DataDir + bdm.Config.NodesFile, nil
	case ClassNameBlockchain, ClassNameTransactions, ClassNameUnapprovedTransactions, ClassNameUnspentOutputs:
		return bdm.Config.DataDir + bdm.Config.BlockchainFile, nil
	}
	return "", errors.New("Unknown DB object name " + name)
}

func (bdm *BoltDBManager) getDBLockFileForObject(name string) (string, error) {
	dbfileName, err := bdm.getDBFileForObject(name)

	if err != nil {
		return "", err
	}

	// replace extension to .lock
	ext := path.Ext(dbfileName)
	dbfileName = dbfileName[0:len(dbfileName)-len(ext)] + ".lock"
	return dbfileName, nil
}

func (bdm *BoltDBManager) isBCDB(name string) bool {
	switch name {
	case ClassNameBlockchain, ClassNameTransactions, ClassNameUnapprovedTransactions, ClassNameUnspentOutputs:
		return true
	}
	return false
}
func (bdm *BoltDBManager) isNodesDB(name string) bool {
	if ClassNameNodes == name {
		return true
	}
	return false
}

func (bdm *BoltDBManager) dbExists(dbFile string) bool {
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		return false
	}

	return true
}
