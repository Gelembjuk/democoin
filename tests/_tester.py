import sys
import re
import _lib
from os import listdir
from os.path import isfile, join

test = ""

if len(sys.argv) > 1 :
    test = sys.argv[1]

if test == "":
    test = "all"
    
# read all test files from this dir 
curdir = _lib.getCurrentDir()

testfiles = [f for f in listdir(curdir) if isfile(join(curdir, f)) and re.search(r'^[a-z].+\.py$',f)]

for testscript in testfiles:
    if test == "all" or test+'.py' == testscript:
        test_module = __import__(test)
        methods = dir(test_module)
        
        if "beforetest" in methods:
            test_module.beforetest(test)
            
        test_module.test(test)
        
        if "aftertest" in methods:
            test_module.aftertest(test)
        