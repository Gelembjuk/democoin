import _lib
import _transfers
import _blocks
import re
import os
import time
import random
import startnode
import blocksnodes
import managenodes
import transactions

def PrepareNodes():
    
    nodeport = '30000'
    
    _lib.StartTestGroup("Wallet Balance")
    
    _lib.CleanTestFolders()
    
    datadir_tmp = CopyBlockchainWithBlocks()
    
    balances = _transfers.GetGroupBalance(datadir_tmp)
    
    datadirs = []
    
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
    
    _lib.StartTestGroup("Temp Data Dirs")
    _lib.StartTest("Node 0 "+os.path.basename(datadir))
    
    for node in nodes:
        _lib.StartTest("Node "+str(node['number'])+" "+os.path.basename(node['datadir']))
    
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
    
    tx = [""] * 6
    tx[0] = _transfers.Send(datadir,address1, nodes[3]['address'] ,as1)
    tx[1] = _transfers.Send(datadir,address1, nodes[3]['address'] ,as1)
    tx[2] = _transfers.Send(datadir,address1, nodes[3]['address'] ,as1)
    
    tx[3] = _transfers.Send(datadir,address1, nodes[4]['address'] ,as1)
    tx[4] = _transfers.Send(datadir,address1, nodes[4]['address'] ,as1)
    tx[5] = _transfers.Send(datadir,address1, nodes[4]['address'] ,as1)
    
    blocks1 = _blocks.WaitBlocks(nodes[4]['datadir'], blockslen + 3)
    
    _lib.FatalAssert(len(blocks1) == blockslen + 3, "Expected "+str(blockslen +3)+" blocks for branch 1")
    
    _lib.StartTestGroup("1 new block on second branch")
    
    balance2 = _transfers.GetBalance(nodes[0]['datadir'], nodes[0]['address'])
    as2 = "%.8f" % round(balance2[0]/5,8)
    
    tx1 = _transfers.Send(nodes[0]['datadir'], nodes[0]['address'], nodes[1]['address'] ,as2)
    tx2 = _transfers.Send(nodes[0]['datadir'], nodes[0]['address'], nodes[2]['address'] ,as2)
    tx3 = _transfers.Send(nodes[0]['datadir'], nodes[0]['address'], nodes[2]['address'] ,as2)
    
    blocks2 = _blocks.WaitBlocks(nodes[2]['datadir'], blockslen + 2)
    _lib.FatalAssert(len(blocks2) == blockslen + 2, "Expected "+str(blockslen +2)+" blocks for branch 2")
    
    #configs for cluster 1
    configfile = "{\"MinterAddress\":\""+address1+"\",\"Port\": "+str(nodeport)+",\"Nodes\":[{\"Host\": \"localhost\",\"Port\":"+str(nodes[3]['port'])+"}, {\"Host\": \"localhost\",\"Port\":"+str(nodes[4]['port'])+"}]}"
    _lib.SaveConfigFile(datadir, configfile)
    
    configfile = "{\"MinterAddress\":\""+nodes[3]['address']+"\",\"Port\": "+str(nodes[3]['port'])+",\"Nodes\":[{\"Host\": \"localhost\",\"Port\":"+str(nodeport)+"}, {\"Host\": \"localhost\",\"Port\":"+str(nodes[4]['port'])+"}]}"
    _lib.SaveConfigFile(nodes[3]['datadir'], configfile)
    
    configfile = "{\"MinterAddress\":\""+nodes[4]['address']+"\",\"Port\": "+str(nodes[4]['port'])+",\"Nodes\":[{\"Host\": \"localhost\",\"Port\":"+str(nodeport)+"}, {\"Host\": \"localhost\",\"Port\":"+str(nodes[3]['port'])+"}]}"
    _lib.SaveConfigFile(nodes[4]['datadir'], configfile)
    
    #config for cluster 2
    configfile = "{\"MinterAddress\":\""+nodes[0]['address']+"\",\"Port\": "+str(nodes[0]['port'])+",\"Nodes\":[{\"Host\": \"localhost\",\"Port\":"+str(nodes[1]['port'])+"}, {\"Host\": \"localhost\",\"Port\":"+str(nodes[2]['port'])+"}]}"
    _lib.SaveConfigFile(nodes[0]['datadir'], configfile)
    
    configfile = "{\"MinterAddress\":\""+nodes[1]['address']+"\",\"Port\": "+str(nodes[1]['port'])+",\"Nodes\":[{\"Host\": \"localhost\",\"Port\":"+str(nodes[0]['port'])+"}, {\"Host\": \"localhost\",\"Port\":"+str(nodes[2]['port'])+"}]}"
    _lib.SaveConfigFile(nodes[1]['datadir'], configfile)
    
    configfile = "{\"MinterAddress\":\""+nodes[2]['address']+"\",\"Port\": "+str(nodes[2]['port'])+",\"Nodes\":[{\"Host\": \"localhost\",\"Port\":"+str(nodes[0]['port'])+"}, {\"Host\": \"localhost\",\"Port\":"+str(nodes[1]['port'])+"}]}"
    _lib.SaveConfigFile(nodes[1]['datadir'], configfile)
    
    return [datadir, address1, nodes, blockslen+1, datadirs]
   
def CopyBlockchainWithBlocks():
    datadir = _lib.CreateTestFolder()
    _lib.CopyTestData(datadir,"bcwith4blocks")
    
    return datadir

def Copy6Nodes():
    datadirs = [""] * 6
    datadirs[0] = _lib.CreateTestFolder()
    _lib.CopyTestData(datadirs[0],"bc6nodes_1")
    
    datadirs[1] = _lib.CreateTestFolder()
    _lib.CopyTestData(datadirs[1],"bc6nodes_2")
    
    datadirs[2] = _lib.CreateTestFolder()
    _lib.CopyTestData(datadirs[2],"bc6nodes_3")
    
    datadirs[3] = _lib.CreateTestFolder()
    _lib.CopyTestData(datadirs[3],"bc6nodes_4")
    
    datadirs[4] = _lib.CreateTestFolder()
    _lib.CopyTestData(datadirs[4],"bc6nodes_5")
    
    datadirs[5] = _lib.CreateTestFolder()
    _lib.CopyTestData(datadirs[5],"bc6nodes_6")
    
    return datadirs