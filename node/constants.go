package main

// ==========================================================
// this can be altered to experiment with blockchain

const applicationtitle = "DemoCoin"
const applicationversion = "0.1 alpha"

// this defines how strong miming is needed. 16 is simple mining 10 sec in simple desktop
// 24 will need 3-4 minutes
const targetBits = 16

// MAx and Min number of transactions per block
const minNumberTransactionInBlock = 3
const maxNumberTransactionInBlock = 100

// ==========================================================
//No need to change this

// File names
const dbFile = "blockchain.db"
const pidFileName = "server.pid"

// DB settings.
const blocksBucket = "blocks"
const transactionsBucket = "unapprovedtransactions"
const utxoBucket = "chainstate"

// other internal constant
const daemonprocesscommandline = "daemonnode"
