import _lib
import re
import time

def StartNodeWithoutBlockchain(datadir):
    _lib.StartTest("Try to start without blockchain")
    res = _lib.ExecuteNode(['startnode','-datadir',datadir])
    _lib.FatalAssertSubstr(res,"Blockchain is not found","Blockchain is not yet inited. Should fail")
    
def InitBockchain(datadir):
    _lib.StartTestGroup("Init blockchain")
    
    _lib.StartTest("Create first address")
    res = _lib.ExecuteNode(['createwallet','-datadir',datadir])
    _lib.FatalAssertSubstr(res,"Your new address","Address creation returned wrong result")

    # get address from this response 
    match = re.match( r'.+: (.+)', res)

    if not match:
        _lib.Fatal("Address can not be found in "+res)
        
    address = match.group(1)

    _lib.StartTest("Create blockchain")
    res = _lib.ExecuteNode(['createblockchain','-datadir',datadir, '-address', address, '-genesis', 'This is the initial block in chain'])
    _lib.FatalAssertSubstr(res,"Done!","Blockchain init failed")
    
    return address

def StartNode(datadir, address):
    _lib.StartTestGroup("Start node")
    
    _lib.StartTest("Start normal")
    res = _lib.ExecuteNode(['startnode','-datadir',datadir,'-port','30000','-minter',address])
    _lib.FatalAssertStr(res,"","Should not be any output on succes start")

    # get process of the node. find this process exists
    _lib.StartTest("Check node state")
    res = _lib.ExecuteNode(['nodestate','-datadir',datadir])
    _lib.FatalAssertSubstr(res,"Server is running","Server should be runnning")

    # get address from this response 
    match = re.search( r'Process: (\d+),', res, re.M)

    if not match:
        _lib.Fatal("Can not get process ID from the response "+res)

    PID = int(match.group(1))

    _lib.FatalAssertPIDRunning(PID, "Can not find process with ID "+str(PID))

    _lib.StartTest("Start node again. should not be allowed")
    res = _lib.ExecuteNode(['startnode','-datadir',datadir])
    _lib.FatalAssertSubstr(res,"Already running or PID file exists","Second attempt to run should fail")

def StopNode(datadir):
    _lib.StartTestGroup("Stop node")
    # get process of the node. find this process exists
    _lib.StartTest("Check node state")
    res = _lib.ExecuteNode(['nodestate','-datadir',datadir])
    _lib.FatalAssertSubstr(res,"Server is running","Server should be runnning")

    # get address from this response 
    match = re.search( r'Process: (\d+),', res, re.M)

    if not match:
        _lib.Fatal("Can not get process ID from the response "+res)

    PID = int(match.group(1))
    
    _lib.FatalAssertPIDRunning(PID, "Can not find process with ID "+str(PID))
        
    _lib.StartTest("Stop node")
    res = _lib.ExecuteNode(['stopnode','-datadir',datadir])
    _lib.FatalAssert(res=="","Should not be any output on succes stop")

    time.sleep(1)
    _lib.FatalAssertPIDNotRunning(PID, "Process with ID "+str(PID)+" should not exist")
        
    _lib.StartTest("Stop node again")
    res = _lib.ExecuteNode(['stopnode','-datadir',datadir])
    _lib.FatalAssert(res=="","Should not be any output on succes stop")

_lib.StartTestGroup("Start/Stop node")

_lib.CleanTestFolders()
datadir = _lib.CreateTestFolder()

StartNodeWithoutBlockchain(datadir)
address = InitBockchain(datadir)
StartNode(datadir, address)
StopNode(datadir)

#_lib.RemoveTestFolder(datadir)
_lib.EndTestGroupSuccess()