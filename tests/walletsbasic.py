import _lib
import _transfers
import _wallet
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
    
    datadir_tmp = CopyBlockchainWithBlocks()
    
    balances = _transfers.GetGroupBalance(datadir_tmp)
    
    address1 = balances.keys()[0]
    address1_2 = balances.keys()[1]
    address1_3 = balances.keys()[2]
    
    # address1_3 becomes a minter. we will send money from other 2 and this will receive rewards
    startnode.StartNode(datadir_tmp, address1_3,nodeport)
    datadir = datadir_tmp
    
    blocks = blocksbasic.GetBlocks(datadir)
    blockslen = len(blocks)
    
    #create 2 wallet locations and 2 wallets in each of them
    walletdatadir1 = _lib.CreateTestFolder("wallet")
    walletdatadir2 = _lib.CreateTestFolder("wallet")
    
    waddress1_1 = _wallet.CreateWallet(walletdatadir1);
    waddress1_2 = _wallet.CreateWallet(walletdatadir1);
    
    waddress2_1 = _wallet.CreateWallet(walletdatadir2);
    waddress2_2 = _wallet.CreateWallet(walletdatadir2);
    
    #send some funds to all that wallets
    amounttosend = "%.8f" % round(balances[address1]/5,8)
    
    _transfers.Send(datadir,address1, waddress1_1 ,amounttosend)
    _transfers.Send(datadir,address1, waddress1_2 ,amounttosend)
    _transfers.Send(datadir,address1, waddress2_1 ,amounttosend)
    
    # we control how blocks are created. here we wait on a block started and then send another 3 TX
    # we will get 2 more blocks here
    #blocks = WaitBlocks(datadir, blockslen + 1)
    time.sleep(1)
    #_lib.FatalAssert(len(blocks) == blockslen + 1, "Expected "+str(blockslen + 1)+" blocks")
    
    _transfers.Send(datadir,address1, waddress2_2 ,amounttosend)
    
    amounttosend2 = "%.8f" % round(balances[address1_2]/5,8)
    _transfers.Send(datadir,address1_2, waddress1_1 ,amounttosend2)
    _transfers.Send(datadir,address1_2, waddress1_2 ,amounttosend2)
    
    # wait to complete blocks 
    blocks = WaitBlocks(datadir, blockslen + 2)
        
    _lib.FatalAssert(len(blocks) == blockslen + 2, "Expected "+str(blockslen + 2)+" blocks")
    
    #get balances on wallets
    am1 = _wallet.GetBalanceWallet(walletdatadir1, waddress1_1, "localhost", nodeport)
    am2 = _wallet.GetBalanceWallet(walletdatadir1, waddress1_2, "localhost", nodeport)
    am3 = _wallet.GetBalanceWallet(walletdatadir2, waddress2_1, "localhost", nodeport)
    am4 = _wallet.GetBalanceWallet(walletdatadir2, waddress2_2, "localhost", nodeport)
    
    _lib.FatalAssert(am1 == float(amounttosend) + float(amounttosend2), "Expected balance is different for wallet 1_1")
    _lib.FatalAssert(am2 == float(amounttosend) + float(amounttosend2), "Expected balance is different for wallet 1_2")
    _lib.FatalAssert(am3 == float(amounttosend), "Expected balance is different for wallet 2_1")
    _lib.FatalAssert(am4 == float(amounttosend), "Expected balance is different for wallet 2_2")
    
    #get group blances on a wallet loc
    balances_new = _transfers.GetGroupBalance(datadir)
    
    #get balances on node wallets
    
    balances1 = _wallet.GetGroupBalanceWallet(walletdatadir1,"localhost", nodeport)
    balances2 = _wallet.GetGroupBalanceWallet(walletdatadir2,"localhost", nodeport)
    
    _lib.FatalAssert(am1 == balances1[waddress1_1], "Expected balance is different from group listing for 1_1")
    _lib.FatalAssert(am2 == balances1[waddress1_2], "Expected balance is different from group listing for 1_2")
    _lib.FatalAssert(am3 == balances2[waddress2_1], "Expected balance is different from group listing for 2_1")
    _lib.FatalAssert(am4 == balances2[waddress2_2], "Expected balance is different from group listing for 2_2")
    
    newbalance1 = round(balances[address1]  - float(amounttosend) * 4,8) 
    
    _lib.FatalAssert(newbalance1 == balances_new[address1], "Expected balance is different after spending")
    
    #send from wallets 
    _wallet.Send(walletdatadir1,waddress1_1, address1 ,amounttosend,"localhost", nodeport)
    _wallet.Send(walletdatadir1,waddress1_1, address1_2 ,amounttosend2,"localhost", nodeport)
    _wallet.Send(walletdatadir1,waddress1_2 ,address1, amounttosend,"localhost", nodeport)
    
    blocks = WaitBlocks(datadir, blockslen + 3)
    _lib.FatalAssert(len(blocks) == blockslen + 3, "Expected "+str(blockslen + 3)+" blocks")
    
    
    am1_back = _wallet.GetBalanceWallet(walletdatadir1, waddress1_1, "localhost", nodeport)
    am1_expected = round(am1 - float(amounttosend) - float(amounttosend2),8)
    
    _lib.FatalAssert(am1_back == am1_expected, "Expected balance after sending from wallet 1_1 is wrong: "+str(am1_back)+", expected "+str(am1_expected))
    
    startnode.StopNode(datadir)
    datadir = ""
    
    #_lib.RemoveTestFolder(datadir)
    _lib.EndTestGroupSuccess()
    
def WaitBlocks(datadir, explen):
    blocks = []
    i = 0
    while True:
        blocks = blocksbasic.GetBlocks(datadir)
        
        if len(blocks) >= explen or i >= 5:
            break
        time.sleep(1)
        i = i + 1
        
    return blocks
def CopyBlockchainWithBlocks():
    datadir = _lib.CreateTestFolder()
    _lib.CopyTestData(datadir,"bcwith4blocks")
    
    return datadir