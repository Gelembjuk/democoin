import _lib
import _transfers
import _blocks
import re
import time
import random
import startnode
import blocksnodes
import managenodes

datadirs = []

def aftertest(testfilter):
    global datadirs
    
    for datadir in datadirs:
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
    
    # address1_3 becomes a minter. we will send money from other 2 and this will receive rewards
    startnode.StartNode(datadir_tmp, address1,nodeport)
    datadir = datadir_tmp
    datadirs.append(datadir_tmp)
    
    nodes = []
    
    for i in range(1, 6):
        port = str(30000+i)
        d = blocksnodes.StartNodeAndImport(port, nodeport, "Server "+str(i))
        datadir_n = d[0]
        address_n = d[1]
        
        nodes.append({'number':i, 'port':port, 'datadir':datadir_n,'address':address_n})
        datadirs.append(datadir_n)
    
    
    # commmon transfer of blocks between nodes
    _lib.StartTestGroup("Transfer of blocks between nodes")
    
    blocks = _blocks.GetBlocks(datadir)
    blockslen = len(blocks)
    
    balance1 = _transfers.GetBalance(datadir, address1)
    as1 = "%.8f" % round(balance1[0]/5,8)
    
    _transfers.Send(datadir,address1, nodes[0]['address'] ,as1)
    _transfers.Send(datadir,address1, nodes[1]['address'] ,as1)
    _transfers.Send(datadir,address1, nodes[2]['address'] ,as1)
    
    blocks = _blocks.WaitBlocks(datadir, blockslen + 1)
    
    _lib.FatalAssert(len(blocks) == blockslen + 1, "Expected "+str(blockslen +1)+" blocks")
    
    #wait while block is posted to all other nodes
    time.sleep(1)
    # check on each node
    for node in nodes:
        blocks = _blocks.WaitBlocks(node['datadir'], blockslen + 1)
        _lib.FatalAssert(len(blocks) == blockslen + 1, "Expected "+str(blockslen +1)+" blocks o node "+str(node['number']))
    
    _lib.StartTestGroup("Create 2 branches of blockchain")
    
    # remove connection between subnetworks
    managenodes.RemoveAllNodes(nodes[0]['datadir'])
    managenodes.RemoveAllNodes(nodes[1]['datadir'])
    managenodes.RemoveAllNodes(nodes[2]['datadir'])
    managenodes.RemoveAllNodes(nodes[3]['datadir'])
    managenodes.RemoveAllNodes(nodes[4]['datadir'])
    managenodes.RemoveAllNodes(datadir)
    
    # first group - main and 4,5 nodes
    managenodes.AddNode(datadir,"localhost",'30004')
    managenodes.AddNode(datadir,"localhost",'30005')
    managenodes.AddNode(nodes[3]['datadir'],"localhost",'30005')
    
    #second group 1,2,3
    managenodes.AddNode(nodes[0]['datadir'],"localhost",'30002')
    managenodes.AddNode(nodes[0]['datadir'],"localhost",'30003')
    managenodes.AddNode(nodes[1]['datadir'],"localhost",'30003')
    
    time.sleep(1)
    
    #check nodes
    
    nodes0 = managenodes.GetNodes(datadir)
    _lib.FatalAssert("localhost:30005", "Node 5 is not in the list of 0")
    _lib.FatalAssert("localhost:30004", "Node 4 is not in the list of 0")
    
    nodes1 = managenodes.GetNodes(nodes[0]['datadir'])
    
    _lib.FatalAssert("localhost:30002", "Node 2 is not in the list of 1")
    _lib.FatalAssert("localhost:30003", "Node 3 is not in the list of 1")
    
    nodes2 = managenodes.GetNodes(nodes[1]['datadir'])
    
    _lib.FatalAssert("localhost:30001", "Node 1 is not in the list of 2")
    _lib.FatalAssert("localhost:30003", "Node 3 is not in the list of 2")
    
    _lib.StartTestGroup("2 new blocks on first branch")
    
    balance1 = _transfers.GetBalance(datadir, address1)
    as1 = "%.8f" % round(balance1[0]/7,8)
    _transfers.Send(datadir,address1, nodes[3]['address'] ,as1)
    _transfers.Send(datadir,address1, nodes[3]['address'] ,as1)
    _transfers.Send(datadir,address1, nodes[3]['address'] ,as1)
    
    _transfers.Send(datadir,address1, nodes[4]['address'] ,as1)
    _transfers.Send(datadir,address1, nodes[4]['address'] ,as1)
    _transfers.Send(datadir,address1, nodes[4]['address'] ,as1)
    
    blocks = _blocks.WaitBlocks(nodes[4]['datadir'], blockslen + 3)
    
    _lib.StartTestGroup("1 new block on second branch")
    
    balance2 = _transfers.GetBalance(nodes[0]['datadir'], nodes[0]['address'])
    as2 = "%.8f" % round(balance1[0]/5,8)
    
    _transfers.Send(nodes[0]['datadir'], nodes[0]['address'], nodes[1]['address'] ,as2)
    _transfers.Send(nodes[0]['datadir'], nodes[0]['address'], nodes[2]['address'] ,as2)
    _transfers.Send(nodes[0]['datadir'], nodes[0]['address'], nodes[2]['address'] ,as2)
    
    blocks = _blocks.WaitBlocks(nodes[2]['datadir'], blockslen + 2)
    
    for node in nodes:
        startnode.StopNode(node['datadir'])
        datadirs[node['number']] = ""
    
    startnode.StopNode(datadir)
    datadirs[0] = ""
    
    #_lib.RemoveTestFolder(datadir)
    _lib.EndTestGroupSuccess()
    
def CopyBlockchainWithBlocks():
    datadir = _lib.CreateTestFolder()
    _lib.CopyTestData(datadir,"bcwith4blocks")
    
    return datadir
