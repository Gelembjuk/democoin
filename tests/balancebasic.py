import _lib
import re
import time
import blocksnodes
import blocksbasic
import startnode
import transactions

datadir = ""

def aftertest(testfilter):
    global datadir
    
    if datadir != "":
        startnode.StopNode(datadir)
        
def test(testfilter):
    global datadir
    
    _lib.StartTestGroup("Wallet Balance")
    
    _lib.CleanTestFolders()
    
    inf = blocksnodes.MakeBlockchainWithBlocks('30000')
    datadir_tmp = inf[0]
    address1 = inf[1]
    address1_2 = inf[2]
    address1_3 = inf[3]
    
    #get balance when a node is not run
    bal1 = GetBalance(datadir_tmp, address1)
    bal1_2 = GetBalance(datadir_tmp, address1_2)
    bal1_3 = GetBalance(datadir_tmp, address1_3)
    
    s1 = bal1 + bal1_2 + bal1_3
    
    startnode.StartNode(datadir_tmp, address1,'30000')
    datadir = datadir_tmp
    
    #get balaces on nodes wallets
    bal1 = GetBalance(datadir, address1)
    bal1_2 = GetBalance(datadir, address1_2)
    bal1_3 = GetBalance(datadir, address1_3)
    
    s2 = bal1 + bal1_2 + bal1_3
    
    _lib.FatalAssert(s1 == s2, "Balances shoul be equal when a node is On and Off")
    
    #get group balance on a node
    
    
    #create 2 wallet locations and 2 wallets in each of them
    walletdatadir1 = _lib.CreateTestFolder()
    walletdatadir2 = _lib.CreateTestFolder()
    
    waddress1_1 = CreateWallet(walletdatadir1);
    waddress1_2 = CreateWallet(walletdatadir1);
    
    waddress2_1 = CreateWallet(walletdatadir2);
    waddress2_2 = CreateWallet(walletdatadir2);
    
    #send some funds to all that wallets
    amounttosend = bal1/5
    
    transactions.Send(datadir,address1, waddress1_1 ,amounttosend)
    transactions.Send(datadir,address1, waddress1_2 ,amounttosend)
    transactions.Send(datadir,address1, waddress2_1 ,amounttosend)
    
    # we control how blocks are created. here we wait on a block started and then send another 3 TX
    # we will get 2 more blocks here
    time.sleep(1)
    
    transactions.Send(datadir,address1, waddress2_2 ,amounttosend)
    amounttosend2 = bal1_2/5
    transactions.Send(datadir,address1_2, waddress1_1 ,amounttosend2)
    transactions.Send(datadir,address1_2, waddress1_2 ,amounttosend2)
    
    # wait to complete blocks 
    time.sleep(3)
    
    blocks = blocksbasic.GetBlocks(datadir)
    
    print blocks
    
    #get balances on wallets
    
    #get group blances on a wallet loc
    
    #get balances on node wallets
    
    startnode.StopNode(datadir)
    datadir = ""
    
    #_lib.RemoveTestFolder(datadir)
    _lib.EndTestGroupSuccess()
    
def GetBalance(datadir, address):
    _lib.StartTest("Request balance for a node wallet "+address)
    res = _lib.ExecuteNode(['getbalance','-datadir',datadir,"-address",address])
    _lib.FatalAssertSubstr(res,"Balance of","Balance info is not found")

    # get balance from this response 
    match = re.match( r'Balance of \'(.+)\': ([0-9.]+)', res)

    if not match:
        _lib.Fatal("Balance can not be found in "+res)
    
    addr = match.group(1)
    
    _lib.FatalAssert(addr == address, "Address in a response is not same as requested. "+res)
    
    balance = match.group(2)
    
    return float(balance)

def CreateWallet(datadir):
    _lib.StartTest("Create new wallet")
    res = _lib.ExecuteWallet(['createwallet','-datadir',datadir])
    _lib.FatalAssertSubstr(res,"Your new address","Address creation failed")
    match = re.match( r'.+: (.+)', res)

    if not match:
        _lib.Fatal("Address can not be found in "+res)
        
    address = match.group(1)
        