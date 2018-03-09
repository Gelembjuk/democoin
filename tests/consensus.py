import _lib
import _transfers
import _blocks
import _complex
import re
import os
import time
import random
import startnode
import blocksnodes
import managenodes
import transactions

datadirs = []

def aftertest(testfilter):
    global datadirs
    
    for datadir in datadirs:
        if datadir != "":
            startnode.StopNode(datadir)
        
def test(testfilter):
    global datadirs
    
    dirs = _complex.Copy6Nodes()
    
    nodes = []
    
    for d in dirs:
        print d
    '''    
    for node in nodes:
        startnode.StopNode(node['datadir'])
        datadirs[node['index']] = ""
    '''
    #_lib.RemoveTestFolder(datadir)
    _lib.EndTestGroupSuccess()


