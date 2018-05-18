import inspect, os, sys
import shutil
import subprocess
import random, string
import re
from shutil import copyfile
import base64
import json

NODE_BIN = '../node/node'
WALLET_BIN = '../wallet/wallet'
VERBOSE = False

def getCurrentDir():
    return os.path.dirname(os.path.abspath(inspect.getfile(inspect.currentframe())))

def CleanTestFolders():
    # Delete all subfolders looking like prev tests
    curdir = getCurrentDir()
    
    dirs = os.listdir( curdir )
    
    for folder in dirs:
        if os.path.isdir(curdir + '/' + folder) and folder.startswith("test"):
            shutil.rmtree(curdir + '/' + folder)
    
    return True

def RemoveTestFolder(path):
    curdir = getCurrentDir()
    
    shutil.rmtree(path)

def CreateTestFolder(suffix = ""):
    curdir = getCurrentDir()
    
    newfolder = 'test'+ suffix + ''.join(random.choice(string.ascii_uppercase + string.digits) for _ in range(5))
    
    newfolder = curdir + '/' + newfolder
    
    os.makedirs(newfolder)
    
    return newfolder

def Execute(command, verbose = False):
    if verbose:
        commandtext = ' '.join(command)
        print commandtext
        sys.Exit(0)
        
    res = subprocess.check_output(command)
    
    if verbose:
        print res
    
    return res

def ExecuteHang(command, folder, verbose = False):
    if verbose:
        commandtext = ' '.join(command)
        print commandtext
        sys.exit(0)
    
    commanddata = base64.b64encode(json.dumps(command))
    folderdata = base64.b64encode(folder)
    
    res = subprocess.check_output(["python","_nodeinteractive.py", commanddata, folderdata])
    
    if verbose:
        print res
    
    return res
    
def ExecuteNode(args,verbose=False):
    command = [NODE_BIN] + args
    return Execute(command,verbose)

def ExecuteWallet(args,verbose=False):
    command = [WALLET_BIN] + args
    return Execute(command,verbose)

def ExecuteHangNode(args, folder, verbose=False):
    command = [NODE_BIN] + args
    return ExecuteHang(command, folder, verbose)

def ExecuteWallet(args,verbose=False):
    command = [WALLET_BIN] + args
    return Execute(command,verbose)

def StartTestGroup(title):
    print "==================="+title+"======================"

def StartTest(title):
    print "\t----------------"+title
def EndTestSuccess():
    print "\tPASS"
    
def EndTestGroupSuccess():
    print "PASS ==="
    
def SaveConfigFile(datadir, contents):
    text_file = open(datadir+"/config.json", "w")
    text_file.write(contents)
    text_file.close()
    
def Exit():
    raise NameError('Test failed')

def CopyTestData(todir,testset):
    srcdir = getCurrentDir()+"/datafortests/"+testset+"/"
    
    copyfile(srcdir+"blockchain.t", todir + "/blockchain.db")
    copyfile(srcdir+"wallet.t", todir + "/wallet.dat")
    
    if os.path.isfile(srcdir+"config.t"):
        copyfile(srcdir+"config.t", todir + "/config.json")
        
    if os.path.isfile(srcdir+"nodeslist.t"):
        copyfile(srcdir+"nodeslist.t", todir + "/nodeslist.db")
#=============================================================================================================
# Assert functions
def Fatal(comment):
    print "\t\tFAIL: "+comment
    Exit()

def AssertStr(s1,s2,comment):
    if s1 != s2:
        print "\t\tFAIL: "+comment
        print s1
        return False
    return True

def FatalAssertStr(s1,s2,comment):
    if not AssertStr(s1,s2,comment):
        Exit()

def AssertSubstr(s1,s2,comment):
    if s2 not in s1:
        print "\t\tFAIL: "+comment
        print s1
        return False
    return True

def FatalAssertSubstr(s1,s2,comment):
    if not AssertSubstr(s1,s2,comment):
        Exit()

def FatalAssertFloat(f1,f2,comment):
    if float(f1) != float(f2):
        print "\t\tFAIL: "+comment
        print "Expected: "+str(f1)+" got: "+str(f2)
        Exit()

def Assert(cond,comment):
    if not cond:
        print "\t\tFAIL: "+comment
        return False
    return True

def FatalAssert(cond,comment):
    if not Assert(cond,comment):
        Exit()
        
def FatalAssertPIDRunning(pid,comment):
    """ Check For the existence of a unix pid. """
    try:
        os.kill(pid, 0)
    except OSError:
        print "\t\tFAIL: "+comment
        Exit()
        
def FatalAssertPIDNotRunning(pid, comment):
    """ Check For the existence of a unix pid. """
    try:
        os.kill(pid, 0)
    except OSError:
        return True
    else:
        print "\t\tFAIL: "+comment
        Exit()
        
def FatalRegex(expr,text,comment):
    if not re.search(expr,text):
        print "\t\tFAIL: "+comment
        Exit()
