import _lib
import re
import time
import startnode
import transactions

#def beforetest(testfilter):
#    print "before test"
#def aftertest(testfilter):
#    print "after test"
def test(testfilter):
    _lib.StartTestGroup("Blocks making")

    _lib.CleanTestFolders()
    datadir = _lib.CreateTestFolder()

    startnode.StartNodeWithoutBlockchain(datadir)
    address = startnode.InitBockchain(datadir)
    startnode.StartNode(datadir, address, '30000')
    startnode.StopNode(datadir)
    
    # create another 3 addresses
    address2 = transactions.CreateWallet(datadir)
    address3 = transactions.CreateWallet(datadir)

    _lib.StartTestGroup("Do transactions")

    transactions.GetUnapprovedTransactionsEmpty(datadir)
    
    amount1 = '1'
    amount2 = '2'
    amount3 = '3'
    
    txid1 = transactions.Send(datadir,address,address2,amount1)
    
    txlist = transactions.GetUnapprovedTransactions(datadir)
    
    _lib.FatalAssert(len(txlist) == 1,"Should be 1 unapproved transaction")
    
    time.sleep(1)
    txid2 = transactions.Send(datadir,address,address3,amount2)
    
    txlist = transactions.GetUnapprovedTransactions(datadir)
    
    _lib.FatalAssert(len(txlist) == 2,"Should be 2 unapproved transaction")
    
    time.sleep(1)
    txid3 = transactions.Send(datadir,address,address3,amount3)
    
    # node needs some time to make a block, so transaction still will be in list of unapproved
    txlist = transactions.GetUnapprovedTransactions(datadir)
    
    _lib.FatalAssert(len(txlist) == 3,"Should be 3 unapproved transaction")
    
    time.sleep(1)
    txid4 = transactions.Send(datadir,address3,address2,amount1)
    
    # node needs some time to make a block, so transaction still will be in list of unapproved
    txlist = transactions.GetUnapprovedTransactions(datadir)
    
    _lib.FatalAssert(len(txlist) == 4,"Should be 4 unapproved transaction")
    
    if txid1 not in txlist.keys():
        _lib.Fatal("Transaction 1 is not in the list of transactions")
    
    if txid2 not in txlist.keys():
        _lib.Fatal("Transaction 2 is not in the list of transactions")
    
    if txid3 not in txlist.keys():
        _lib.Fatal("Transaction 3 is not in the list of transactions")
    
    if txid4 not in txlist.keys():
        _lib.Fatal("Transaction 4 is not in the list of transactions")
    
    _lib.FatalAssertFloat(amount1, txlist[txid1][2], "Amount of transaction 1 is wrong")
    
    _lib.FatalAssertFloat(amount2, txlist[txid2][2], "Amount of transaction 2 is wrong")
    
    _lib.FatalAssertFloat(amount3, txlist[txid3][2], "Amount of transaction 3 is wrong")
    
    _lib.FatalAssertFloat(amount1, txlist[txid4][2], "Amount of transaction 4 is wrong")
    
    blockchash = MintBlock(datadir,address)
    
    transactions.GetUnapprovedTransactionsEmpty(datadir)

    _lib.StartTestGroup("Send 30 transactions")
    
    microamount = 0.01
    # send many transactions 
    for i in range(1,10):
        _lib.StartTest("Iteration "+str(i))
        txid1 = transactions.Send(datadir,address,address2,microamount)
        txid2 = transactions.Send(datadir,address2,address3,microamount)
        txid3 = transactions.Send(datadir,address3,address,microamount)
        
        txlist = transactions.GetUnapprovedTransactions(datadir)
        
        _lib.FatalAssert(len(txlist) == i * 3,"Should be "+str(i*3)+" unapproved transaction")
        
        if txid1 not in txlist.keys():
            _lib.Fatal("Transaction 1 is not in the list of transactions after iteration "+str(i))
    
        if txid2 not in txlist.keys():
            _lib.Fatal("Transaction 2 is not in the list of transactions after iteration "+str(i))
    
        if txid3 not in txlist.keys():
            _lib.Fatal("Transaction 3 is not in the list of transactions after iteration "+str(i))
            
        time.sleep(1)
    
    blockchash = MintBlock(datadir,address)
    transactions.GetUnapprovedTransactionsEmpty(datadir)
    
    _lib.StartTestGroup("Send 30 transactions. Random value")
    
    microamountmax = 0.01
    microamountmin = 0.0095
    # send many transactions 
    for i in range(1,10):
        _lib.StartTest("Iteration "+str(i))
        a1 = random.uniform(microamountmin, microamountmax)
        a2 = random.uniform(microamountmin, microamountmax)
        a3 = random.uniform(microamountmin, microamountmax)
        txid1 = transactions.Send(datadir,address,address2,a1)
        txid2 = transactions.Send(datadir,address2,address3,a2)
        txid3 = transactions.Send(datadir,address3,address,a3)
        
        txlist = transactions.GetUnapprovedTransactions(datadir)
        
        _lib.FatalAssert(len(txlist) == i * 3,"Should be "+str(i*3)+" unapproved transaction")
        
        if txid1 not in txlist.keys():
            _lib.Fatal("Transaction 1 is not in the list of transactions after iteration "+str(i))
    
        if txid2 not in txlist.keys():
            _lib.Fatal("Transaction 2 is not in the list of transactions after iteration "+str(i))
    
        if txid3 not in txlist.keys():
            _lib.Fatal("Transaction 3 is not in the list of transactions after iteration "+str(i))
            
        time.sleep(1)
    
    blockchash = MintBlock(datadir,address)
    transactions.GetUnapprovedTransactionsEmpty(datadir)
    
    #_lib.RemoveTestFolder(datadir)
    _lib.EndTestGroupSuccess()
    
def MintBlock(datadir,minter):
    _lib.StartTest("Force to Mint a block")
    res = _lib.ExecuteNode(['mineblock','-datadir',datadir,'-minter',minter])
    _lib.FatalAssertSubstr(res,"New block mined with the hash","Block making failed")
    
    match = re.search( r'New block mined with the hash ([0-9a-zA-Z]+).', res)

    if not match:
        _lib.Fatal("New block hash can not be found in response "+res)
        
    blockhash = match.group(1)
    
    return blockhash

