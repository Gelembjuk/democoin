import _lib
import re
import time

def StartNodeInteractive(datadir, address, port,comment = ""):
    _lib.StartTest("Start node (debug) "+comment)
    res = _lib.ExecuteHangNode(['startintnode','-datadir',datadir,'-port',port,'-minter',address],datadir)
    _lib.FatalAssertSubstr(res,"Process started","No process start marker")
