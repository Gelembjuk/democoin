package main

// ==========================================================
// this can be altered to experiment with blockchain

// this defines how strong miming is needed. 16 is simple mining less 5 sec in simple desktop
// 24 will need 30 seconds in average
const targetBits = 16

// MAx and Min number of transactions per block
const maxMinNumberTransactionInBlock = 3
const maxNumberTransactionInBlock = 10000

// ==========================================================
//No need to change this

// File names
const dbFile = "blockchain.db"
const dbFileLock = "blockchain.lock"
const pidFileName = "server.pid"

// DB settings.
const blocksBucket = "blocks"
const transactionsBucket = "unapprovedtransactions"
const utxoBucket = "chainstate"

// other internal constant
const daemonprocesscommandline = "daemonnode"

// ==========================================================
// Testing mode constants
// we need this for testing purposes. can be set to 0 on production system
const MinimumBlockBuildingTime = 3 // seconds
