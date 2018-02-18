import _lib
import re

_lib.StartTestGroup("Start/Stop node")

_lib.CleanTestFolders()
datadir = _lib.CreateTestFolder()

_lib.StartTest("Try to start without blockchain")
res = _lib.ExecuteNode(['startnode','-datadir',datadir])
_lib.FatalAssertSubstr(res,"Blockchain is not found","Blockchain is not yet inited. Should fail")

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

_lib.StartTest("Start normal")
res = _lib.ExecuteNode(['startnode','-datadir',datadir])
_lib.FatalAssert(res=="","Should not be any output on succes start")

# get process of the node. find this process exists

_lib.StartTest("Stop node")
res = _lib.ExecuteNode(['stopnode','-datadir',datadir])
_lib.FatalAssert(res=="","Should not be any output on succes stop")

# check process exists

_lib.EndTestGroupSuccess()