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
    
    nodeport = '30000'
    
    _lib.StartTestGroup("Wallet Balance")
    
    _lib.CleanTestFolders()
    
    inf = blocksnodes.MakeBlockchainWithBlocks(nodeport)
    datadir_tmp = inf[0]
    address1 = inf[1]
    address1_2 = inf[2]
    address1_3 = inf[3]
    
    balances = GetGroupBalance(datadir_tmp)
    
    #get balance when a node is not run
    bal1 = GetBalance(datadir_tmp, address1)
    bal1_2 = GetBalance(datadir_tmp, address1_2)
    bal1_3 = GetBalance(datadir_tmp, address1_3)
    
    _lib.FatalAssert(bal1 == balances[address1], "Balance is different from group rec for 1")
    _lib.FatalAssert(bal1_2 == balances[address1_2], "Balance is different from group rec for 2")
    _lib.FatalAssert(bal1_3 == balances[address1_3], "Balance is different from group rec for 3")
    
    s1 = bal1 + bal1_2 + bal1_3
    
    startnode.StartNode(datadir_tmp, address1,nodeport)
    datadir = datadir_tmp
    
    #get balaces on nodes wallets
    bal1 = GetBalance(datadir, address1)
    bal1_2 = GetBalance(datadir, address1_2)
    bal1_3 = GetBalance(datadir, address1_3)
    
    s2 = bal1 + bal1_2 + bal1_3
    
    _lib.FatalAssert(s1 == s2, "Balances shoul be equal when a node is On and Off")
    
    #get group balance on a node
    balances = GetGroupBalance(datadir)
    _lib.FatalAssert(bal1 == balances[address1], "Balance is different from group rec for 1")
    _lib.FatalAssert(bal1_2 == balances[address1_2], "Balance is different from group rec for 2")
    _lib.FatalAssert(bal1_3 == balances[address1_3], "Balance is different from group rec for 3")
    
    #create 2 wallet locations and 2 wallets in each of them
    walletdatadir1 = _lib.CreateTestFolder("wallet")
    walletdatadir2 = _lib.CreateTestFolder("wallet")
    
    waddress1_1 = CreateWallet(walletdatadir1);
    waddress1_2 = CreateWallet(walletdatadir1);
    
    waddress2_1 = CreateWallet(walletdatadir2);
    waddress2_2 = CreateWallet(walletdatadir2);
    
    #send some funds to all that wallets
    amounttosend = "%.8f" % round(bal1/5,8)
    
    transactions.Send(datadir,address1, waddress1_1 ,amounttosend)
    transactions.Send(datadir,address1, waddress1_2 ,amounttosend)
    transactions.Send(datadir,address1, waddress2_1 ,amounttosend)
    
    # we control how blocks are created. here we wait on a block started and then send another 3 TX
    # we will get 2 more blocks here
    time.sleep(4)
    
    transactions.Send(datadir,address1, waddress2_2 ,amounttosend)
    amounttosend2 = "%.8f" % round(bal1_2/5,8)
    transactions.Send(datadir,address1_2, waddress1_1 ,amounttosend2)
    transactions.Send(datadir,address1_2, waddress1_2 ,amounttosend2)
    
    # wait to complete blocks 
    time.sleep(3)
    
    blocks = blocksbasic.GetBlocks(datadir)
    
    _lib.FatalAssert(len(blocks) == 6, "Expected 6 blocks")
    
    #get balances on wallets
    am1 = GetBalanceWallet(walletdatadir1, waddress1_1, "localhost", nodeport)
    am2 = GetBalanceWallet(walletdatadir1, waddress1_2, "localhost", nodeport)
    am3 = GetBalanceWallet(walletdatadir2, waddress2_1, "localhost", nodeport)
    am4 = GetBalanceWallet(walletdatadir2, waddress2_2, "localhost", nodeport)
    
    _lib.FatalAssert(am1 == float(amounttosend) + float(amounttosend2), "Expected balance is different for wallet 1_1")
    _lib.FatalAssert(am2 == float(amounttosend) + float(amounttosend2), "Expected balance is different for wallet 1_2")
    _lib.FatalAssert(am3 == float(amounttosend), "Expected balance is different for wallet 2_1")
    _lib.FatalAssert(am4 == float(amounttosend), "Expected balance is different for wallet 2_2")
    
    #get group blances on a wallet loc
    balances = GetGroupBalance(datadir)
    #get balances on node wallets
    
    balances1 = GetGroupBalanceWallet(walletdatadir1,"localhost", nodeport)
    balances2 = GetGroupBalanceWallet(walletdatadir2,"localhost", nodeport)
    
    _lib.FatalAssert(am1 == balances1[waddress1_1], "Expected balance is different from group listing for 1_1")
    _lib.FatalAssert(am2 == balances1[waddress1_2], "Expected balance is different from group listing for 1_2")
    _lib.FatalAssert(am3 == balances2[waddress2_1], "Expected balance is different from group listing for 2_1")
    _lib.FatalAssert(am4 == balances2[waddress2_2], "Expected balance is different from group listing for 2_2")
    
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
    
    return round(float(balance),8)

def GetGroupBalance(datadir):
    _lib.StartTest("Request group balance for addresses on a node")
    res = _lib.ExecuteNode(['getbalances','-datadir',datadir])
    _lib.FatalAssertSubstr(res,"Balance for all addresses:","Balance result not printed")
    
    regex = ur"([a-z0-9A-Z]+): ([0-9.]+)"

    balancesres = re.findall(regex, res)
    
    balances = {}
    
    for r in balancesres:
        balances[r[0]] = round(float(r[1]),8)
    
    return balances

def GetBalanceWallet(datadir, address, host, port):
    _lib.StartTest("Request balance for a wallet "+address)
    res = _lib.ExecuteWallet(['getbalance','-datadir',datadir,"-address",address,"-nodehost",host,"-nodeport",port])
    _lib.FatalAssertSubstr(res,"Balance of","Balance info is not found")

    # get balance from this response 
    match = re.match( r'Balance of \'(.+)\': ([0-9.]+)', res)

    if not match:
        _lib.Fatal("Balance can not be found in "+res)
    
    addr = match.group(1)
    
    _lib.FatalAssert(addr == address, "Address in a response is not same as requested. "+res)
    
    balance = match.group(2)
    
    return round(float(balance),8)

def GetGroupBalanceWallet(datadir,host,port):
    _lib.StartTest("Request group balance for addresses in a wallet")
    res = _lib.ExecuteWallet(['listbalances','-datadir',datadir,"-nodehost",host,"-nodeport",port])
    _lib.FatalAssertSubstr(res,"Balance for all addresses:","Balance result not printed")
    
    regex = ur"([a-z0-9A-Z]+): ([0-9.]+)"

    balancesres = re.findall(regex, res)
    
    balances = {}
    
    for r in balancesres:
        balances[r[0]] = round(float(r[1]),8)
    
    return balances

def CreateWallet(datadir):
    _lib.StartTest("Create new wallet")
    res = _lib.ExecuteWallet(['createwallet','-datadir',datadir])
    _lib.FatalAssertSubstr(res,"Your new address","Address creation failed")
    match = re.match( r'.+: (.+)', res)

    if not match:
        _lib.Fatal("Address can not be found in "+res)
        
    address = match.group(1)
    
    return address
        