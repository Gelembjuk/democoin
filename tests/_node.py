import _lib
import re
import time

def StartNodeInteractive(datadir, address, port,comment = ""):
    _lib.StartTest("Start node (debug) "+comment)
    res = _lib.ExecuteHangNode(['startintnode','-datadir',datadir,'-port',port,'-minter',address],datadir)
    _lib.FatalAssertSubstr(res,"Process started","No process start marker")

def GetWallets(datadir):
    _lib.StartTest("Get node wallets")
    res = _lib.ExecuteNode(['listaddresses','-datadir',datadir])
    
    _lib.FatalAssertSubstr(res,"Wallets (addresses)","No list of wallets")
    
    regex = ur"(1[a-zA-Z0-9]{30,100})"

    addresses = re.findall(regex, res)
    
    return addresses