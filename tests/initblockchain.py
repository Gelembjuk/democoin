import _lib
import re
import time
import startnode

#def beforetest(testfilter):
#    print "before test"
#def aftertest(testfilter):
#    print "after test"
def test(testfilter):
    _lib.StartTestGroup("Start/Stop node")

    _lib.CleanTestFolders()
    datadir1 = _lib.CreateTestFolder()
    datadir2 = _lib.CreateTestFolder()

    startnode.StartNodeWithoutBlockchain(datadir1)
    address = startnode.InitBockchain(datadir1)
    #this starts on a port 30000
    startnode.StartNode(datadir1, address,'30000')
    
    #start second node. should fail
    startnode.StartNodeWithoutBlockchain(datadir2)
    
    IportBockchain(datadir2,"localhost",'30000')
    startnode.StartNode(datadir1, address,'30001')
    
    startnode.StopNode(datadir1)
    startnode.StopNode(datadir2)

    _lib.RemoveTestFolder(datadir1)
    _lib.RemoveTestFolder(datadir2)
    _lib.EndTestGroupSuccess()
    
def IportBockchain(datadir,host,port):
    _lib.StartTestGroup("Import blockchain")
    
    _lib.StartTest("Create first address before importing blockchain")
    res = _lib.ExecuteNode(['createwallet','-datadir',datadir])
    _lib.FatalAssertSubstr(res,"Your new address","Address creation returned wrong result")

    _lib.FatalRegex(r'.+: (.+)', res, "Address can not be found in "+res);
    
    _lib.StartTest("Import blockchain from node 1")
    res = _lib.ExecuteNode(['initblockchain','-datadir',datadir, '-nodehost', host, '-nodeport', port])
    print res
    _lib.FatalAssertSubstr(res,"Done!","Blockchain init failed")
    
    return address