import inspect, os, sys
import shutil
import subprocess
import random, string

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

def CreateTestFolder():
    curdir = getCurrentDir()
    
    newfolder = 'test' + ''.join(random.choice(string.ascii_uppercase + string.digits) for _ in range(5))
    
    newfolder = curdir + '/' + newfolder
    
    os.makedirs(newfolder)
    
    return newfolder

def Execute(command):
    if VERBOSE:
        commandtext = ' '.join(command)
        print commandtext
        
    res = subprocess.check_output(command)
    
    if VERBOSE:
        print res
    
    return res
    
def ExecuteNode(args):
    command = [NODE_BIN] + args
    return Execute(command)

def ExecuteWallet(args):
    command = [WALLET_BIN] + args
    return Execute(command)

def StartTestGroup(title):
    print "==================="+title+"======================"

def StartTest(title):
    print "\t----------------"+title
def EndTestSuccess():
    print "\tPASS"
    
def EndTestGroupSuccess():
    print "PASS ==="
    
def Exit():
    sys.exit(0)
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
        
