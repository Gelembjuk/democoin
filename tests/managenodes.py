import _lib
import _transfers
import _blocks
import re
import os
import time
import startnode
import initblockchain
import blocksbasic
import transactions

datadir1 = ""
datadir2 = ""
datadir3 = ""

def aftertest(testfilter):
    global datadir1
    global datadir2
    global datadir3
    
    if datadir1 != "" or datadir2 != "" or datadir3 != "":
        _lib.StartTestGroup("Ending After failure of the test")
    
    if datadir1 != "":
        startnode.StopNode(datadir1,"Server 1")
    if datadir2 != "":    
        startnode.StopNode(datadir2,"Server 2")
    if datadir3 != "":    
        startnode.StopNode(datadir3,"Server 3")
    
def test(testfilter):
    global datadir1
    global datadir2
    global datadir3
    
    _lib.StartTestGroup("Manage nodes list")

    _lib.CleanTestFolders()
    datadir1 = _lib.CreateTestFolder()
    datadir2 = _lib.CreateTestFolder()
    
    _lib.StartTestGroup("Create blockchain and run node 1")
    r = blocksbasic.PrepareBlockchain(datadir1,'30000')
    address = r[0]
    
    startnode.StartNode(datadir1, address,'30000', "Server 1")

    address2 = initblockchain.ImportBockchain(datadir2,"localhost",'30000')
    
    RemoveAllNodes(datadir1)
    
    AddNode(datadir1, "localhost",'30001')
    
    nodes = GetNodes(datadir1)
    _lib.FatalAssert(len(nodes) == 1,"Should be 1 nodes in output")
    
    RemoveNode(datadir1, "localhost",'30001')
    
    nodes = GetNodes(datadir1)
    _lib.FatalAssert(len(nodes) == 0,"Should be 0 nodes in output")
    
    startnode.StartNode(datadir2, address2,'30001', "Server 2")
    
    RemoveAllNodes(datadir2)
    
    AddNode(datadir1, "localhost",'30001')
    
    nodes = GetNodes(datadir1)
    _lib.FatalAssert(len(nodes) == 1,"Should be 1 nodes in output")
    
    # check transactions work fine between nodes
    _transfers.Send(datadir1,address,address2,'3')
    
    txlist = transactions.GetUnapprovedTransactions(datadir1)
    
    _lib.FatalAssert(len(txlist) == 1,"Should be 1 unapproved transaction")
    
    blocks = _blocks.WaitBlocks(datadir1,2)
    
    # send another 2 TX to make a block
    _transfers.Send(datadir1,address,address2,'0.01')
    _transfers.Send(datadir1,address,address2,'0.01')
    
    blocks = _blocks.WaitBlocks(datadir1,3)
    
    txid1 = _transfers.Send(datadir1,address,address2,'1')
    
    txlist = transactions.GetUnapprovedTransactions(datadir1)
    
    _lib.FatalAssert(len(txlist) == 1,"Should be 1 unapproved transaction")
    
    time.sleep(1)
    
    # and now get transactions from second node
    txlist = transactions.GetUnapprovedTransactions(datadir2)
    
    _lib.FatalAssert(len(txlist) == 1,"Should be 1 unapproved transaction on second node")
    
    if txid1 not in txlist.keys():
        _lib.Fatal("Transaction 1 is not in the list of transactions on second node")
    
    # start one more node 
    datadir3 = _lib.CreateTestFolder()
    address3 = initblockchain.ImportBockchain(datadir3,"localhost",'30000')
    
    startnode.StartNode(datadir3, address3,'30002', "Server 3")
    
    RemoveAllNodes(datadir3)
    
    AddNode(datadir3, "localhost",'30001')
    
    time.sleep(2)# wait while nodes exchange addresses
    nodes = GetNodes(datadir1)
    _lib.FatalAssert(len(nodes) == 2,"Should be 2 nodes in output of 1")
    
    nodes = GetNodes(datadir2)
    _lib.FatalAssert(len(nodes) == 2,"Should be 2 nodes in output of 2")
    
    nodes = GetNodes(datadir3)
    _lib.FatalAssert(len(nodes) == 2,"Should be 2 nodes in output of 3")
    
    txid1 = _transfers.Send(datadir1,address,address3,'4') 
    
    txlist1 = transactions.GetUnapprovedTransactions(datadir1)
    transactions.GetUnapprovedTransactionsEmpty(datadir3)
    
    time.sleep(3) # we need to give a chance to sync all
    
    txlist2 = transactions.GetUnapprovedTransactions(datadir2)
   
    _lib.FatalAssert(len(txlist1) == 2,"Should be 2 unapproved transactions on 1")
    _lib.FatalAssert(len(txlist2) == 2,"Should be 2 unapproved transactions on 2")
    
    if txid1 not in txlist1.keys():
        _lib.Fatal("Transaction 2 is not in the list of transactions on node 1")
        
    if txid1 not in txlist2.keys():
        _lib.Fatal("Transaction 2 is not in the list of transactions on node 2")
        
    # send one more TX. Block must be created    
    txid3 = _transfers.Send(datadir1,address,address2,'1')
    
    time.sleep(4) # wait while a block is minted and posted to other nodes 
    transactions.GetUnapprovedTransactionsEmpty(datadir1)
    transactions.GetUnapprovedTransactionsEmpty(datadir2)
    transactions.GetUnapprovedTransactionsEmpty(datadir3)
    
    # check if a block is present on all nodes. it must be 2 block on every node
    blockshashes = _blocks.GetBlocks(datadir1)
    
    _lib.FatalAssert(len(blockshashes) == 4,"Should be 4 blocks in blockchain on 1")
    
    blockshashes = _blocks.GetBlocks(datadir2)
    _lib.FatalAssert(len(blockshashes) == 4,"Should be 4 blocks in blockchain on 2")
    
    blockshashes = _blocks.GetBlocks(datadir3)
    _lib.FatalAssert(len(blockshashes) == 4,"Should be 4 blocks in blockchain on 3")
    
    startnode.StopNode(datadir1,"Server 1")
    startnode.StopNode(datadir2,"Server 2")
    startnode.StopNode(datadir3,"Server 3")

    #_lib.RemoveTestFolder(datadir1)
    #_lib.RemoveTestFolder(datadir2)
    
    datadir1 = ""
    datadir2 = ""
    datadir3 = ""
    
    _lib.EndTestGroupSuccess()
    
def GetNodes(datadir):
    _lib.StartTest("Get nodes")
    res = _lib.ExecuteNode(['shownodes','-datadir',datadir])
    
    _lib.FatalAssertSubstr(res,"Nodes:","Output should contain list of nodes")

    regex = ur"  ([^: ]+):(\d+)"

    nodes = re.findall(regex, res)
    
    nodeslist={}
    
    for n in nodes:
        nodeslist[n[0]+':'+n[1]] = n

    return nodeslist

def RemoveNode(datadir, nodehost,nodeport):
    _lib.StartTest("Remove node "+nodehost+":"+str(nodeport))
    res = _lib.ExecuteNode(['removenode','-datadir',datadir,'-nodehost',nodehost,'-nodeport',nodeport])
    _lib.FatalAssertSubstr(res,"Success!","Output should contain success message")
    
def AddNode(datadir, nodehost,nodeport):
    _lib.StartTest("Add node "+nodehost+":"+str(nodeport))
    res = _lib.ExecuteNode(['addnode','-datadir',datadir,'-nodehost',nodehost,'-nodeport',nodeport])
    _lib.FatalAssertSubstr(res,"Success!","Output should contain success message")

def RemoveAllNodes(datadir):
    nodes = GetNodes(datadir)
    
    for n in nodes:
        nv = nodes[n]
        RemoveNode(datadir, nv[0],nv[1])
        
    nodes = GetNodes(datadir)
    
    _lib.FatalAssert(len(nodes) == 0,"Should be 0 nodes in output")
        