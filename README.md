# DemoCoin
## Blockchain cryptocurrency in Go

The aim of this project is to create simple cryptocurrency platform with using Blockchain technology.

The application is simpler alternative of bitcoin, created with Go programming language. 
This was created with education purposes. It can be used for learning of how cryptocurrency works.

The application has only command line interface. It has **no a GUI** yet. This project contains two apps - Node and Wallet

### Node 

This is the server that keeps blockchain and manages it. It is like "Bitcoin Core".

The node can start own blockchain from beginning or can join to some existent blockchain.

### Wallet 

It is a lite client. It doesn't need to keep full blockchain. It can manage multiple wallets (addresses) and do money transfers. 

It is possible to use only **Node** without a wallet client. A node has a wallet built in.

## Usage

### Node

```
$****./node 
Usage  
  help - Prints this help
  == Any of next commands can have optional argument [-datadir /path/to/dir] [-logdest stdout]==
  createwallet
        - Generates a new key-pair and saves it into the wallet file
  createblockchain -address ADDRESS -genesis GENESISTEXT
        - Create a blockchain and send genesis block reward to ADDRESS
  initblockchain [-nodehost HOST] [-nodeport PORT]
        - Loads a blockchain from other node to init the DB.
  printchain [-view short|long]
        - Print all the blocks of the blockchain. Default view is long
  makeblock [-minter ADDRESS]
        - Try to mine new block if there are enough transactions
  dropblock
        - Delete last block fro the block chain. All transaction are returned back to unapproved state
  reindexunspent
        - Rebuilds the database of unspent transactions outputs
  showunspent -address ADDRESS
        - Print the list of all unspent transactions and balance
  unapprovedtransactions
        - Print the list of transactions not included in any block yet
  getbalance -address ADDRESS
        - Get balance of ADDRESS
  listaddresses
        - Lists all addresses from the wallet file
  getbalances
        - Lists all addresses from the wallet file and show balance for each
  addrhistory -address ADDRESS
        - Shows all transactions for a wallet address
  send -from FROM -to TO -amount AMOUNT
        - Send AMOUNT of coins from FROM address to TO. 
  canceltransaction -transaction TRANSACTIONID
        - Cancel unapproved transaction. NOTE!. This cancels only from local cache!
  startnode [-minter ADDRESS] [-port PORT]
        - Start a node server. -minter defines minting address and -port - listening port
  startintnode [-minter ADDRESS] [-port PORT]
        - Start a node server in interactive mode (no deamon). -minter defines minting address and -port - listening port
  stopnode
        - Stop runnning node
  nodestate
        - Print state of the node process
  shownodes
        - Display list of nodes addresses, including inactive
  addnode -nodehost HOST -nodeport PORT
        - Adds new node to list of connections
  removenode -nodehost HOST -nodeport PORT
        - Removes a node from list of connections
```
### Wallet

```
$./wallet 


Usage:
  help - Prints this help
  == Any of next commands can have optional argument [-datadir /path/to/dir] [-logdest stdout] ==
  createwallet
        - Generates a new key-pair and saves it into the wallet file
  showunspent -address ADDRESS
        - Displays the list of all unspent transactions and total balance
  showhistory -address ADDRESS
        - Displays the wallet history. All In/Out transactions
  getbalance -address ADDRESS
        - Get balance of ADDRESS
  listaddresses
        - Lists all addresses from the wallet file
  listbalances
        - Lists all addresses from the wallet file and show balance for each
  send -from FROM -to TO -amount AMOUNT
        - Send AMOUNT of coins from FROM address to TO. 
  setnode -nodehost HOST -nodeport PORT
        - Saves a node host and port to configfile.
```

### Your test scenario

#### Download and compile

Install dependencies

```
go get github.com/boltdb/bolt
go get github.com/btcsuite/btcutil
```

Now get the code of DemoCoin

```
go get github.com/gelembjuk/democoin
```

Go to sources. On Ubuntu this looks like

```
cd $GOPATH/src/github.com/gelembjuk/democoin/
```

Finally, build the node and the wallet 

```
cd node/
go build

cd ../wallet
go build
```

Now, you can run ./node and ./wallet commands (or node.exe and wallet.exe on Windows)

## Author

Roman Gelembjuk , roman@gelembjuk.com 