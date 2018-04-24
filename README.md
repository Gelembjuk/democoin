# DemoCoin
## Blockchain cryptocurrency in Go

The aim of this project is to create simple cryptocurrency platform with using Blockchain technology.

The application is simpler alternative of bitcoin, created with Go programming language. 
This was created with education purposes. It can be used for learning of how cryptocurrency works.

The application has only command line interface. It has **no a GUI** yet. This project contains two apps - Node and Wallet

Web site of the project http://democoin.gelembjuk.com/

### Node 

This is the server that keeps blockchain and manages it. It is like "Bitcoin Core".

The node can start own blockchain from beginning or can join to some existent blockchain.

### Wallet 

It is a light client. It doesn't need to keep full blockchain. It can manage multiple wallets (addresses) and do money transfers. 

It is possible to use only **Node** without a wallet client. A node has a wallet built in.

## Usage

### Node

```
$./node 
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
  startintnode [-minter ADDRESS] [-host HOST] -port PORT]
        - Start a node server in interactive mode (no deamon). -minter defines minting address, -host - node hostname and -port - listening port
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

#### Your quick test scenario

1. Init import of existent blockchain (DemoCoin cryptocurrency)

```
./node initblockchain
DemoCoin - 0.1 alpha

Done! First part of bockchain loaded. Next part will be loaded on background when node started
```

2. Create your wallet

```
./node createwallet
DemoCoin - 0.1 alpha

Your new address: 18wTEuoYRjEWZZqPKdsJ5ZvBMbiueDGUtT
```

3. Start your node. Use wallet you just created as minter address. Also, you must have public host name and port opened, so, other nodes can connect to you. 

```
./node startnode -minter 18wTEuoYRjEWZZqPKdsJ5ZvBMbiueDGUtT -port 20001 -host YOUR.EXTERNAL.IP.OR.HOSTNAME
```

4. Check your node state. You will see blocks are loaded from public blockchain called "DemoCoin".

```
./node nodestate
Node Server State:
Server is running. Process: 14058, listening on the port 20001
Blockchain state:
  Number of blocks - 15
  Loaded 15 of 772 blocks
  Number of unapproved transactions - 0
  Number of unspent transactions outputs - 148
```

#### Your custom lockchain test scenario

1. Create a first wallet. In your case a wallet addres will be different

```
./node createwallet
DemoCoin - 0.1 alpha

Your new address: 18wTEuoYRjEWZZqPKdsJ5ZvBMbiueDGUtT
```

2. Init a blockchain

2.A. Create new blockchain. Use your wallet address to assign first block reward to it

```
./node createblockchain -address 18wTEuoYRjEWZZqPKdsJ5ZvBMbiueDGUtT -genesis "THis is initial block of new cryptocurrency"
```

2.B. Or connect to existent blockchain. You can provide a node address and port or just use defalt nodes to join the existent coin "DemoCoin"

```
./node initblockchain -nodehost HOST -nodeport PORT
```

3. Display the blocks list

```
./node printchain
```
4. Create one more wallet

```
./node createwallet
Your new address: 1G7aUSsrFkGTMVrAyEsWTumMrods3mxBfv
```

5. Send money from the first wallet to this one

```
./node send -from 18wTEuoYRjEWZZqPKdsJ5ZvBMbiueDGUtT -to 1G7aUSsrFkGTMVrAyEsWTumMrods3mxBfv -amount 2
```

6. Force to make a block (while node is not running, it will not make blocks automatically). Set your first wallet to be a minter. It will get reward for new block.

```
./node makeblock -minter 18wTEuoYRjEWZZqPKdsJ5ZvBMbiueDGUtT
```

7. Display balance of wallets

```
./node getbalances
Balance for all addresses:

18wTEuoYRjEWZZqPKdsJ5ZvBMbiueDGUtT: 18.00000000 (Approved - 18.00000000, Pending - 0.00000000)
1G7aUSsrFkGTMVrAyEsWTumMrods3mxBfv: 2.00000000 (Approved - 2.00000000, Pending - 0.00000000)
```

8. Start a node to run as a server. You have to provide a wallet address who will get rewards for new blocks and a network port to listen connections. Then check a node status. You have to run on a host (IP) and port accesible from outside to other nodes (except, if you build your local network cryptocurrency).

```
./node startnode -minter 18wTEuoYRjEWZZqPKdsJ5ZvBMbiueDGUtT -host myserverexternalhost.com -port 20000
./node nodestate
```

9. Send another 2 transactions and wait for new block (minimum number of transactions per block is 'count of blocks'-1, if your chain has now more blocks, send more transactions to get new block)

```
./node send -from 18wTEuoYRjEWZZqPKdsJ5ZvBMbiueDGUtT -to 1G7aUSsrFkGTMVrAyEsWTumMrods3mxBfv -amount 0.5
./node send -from 18wTEuoYRjEWZZqPKdsJ5ZvBMbiueDGUtT -to 1G7aUSsrFkGTMVrAyEsWTumMrods3mxBfv -amount 0.6
```

10. Verify the chain state

```
./node getbalances
./node printchain
```

11. Stop the node

```
./node stopnode
```

## Author

Roman Gelembjuk , roman@gelembjuk.com 

http://democoin.gelembjuk.com/