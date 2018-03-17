import _lib
import re

def GetBalance(datadir, address):
    _lib.StartTest("Request balance for a node wallet "+address)
    res = _lib.ExecuteNode(['getbalance','-datadir',datadir,"-address",address])
    _lib.FatalAssertSubstr(res,"Balance of","Balance info is not found")
    
    # get balance from this response 
    match = re.search( r'Balance of \'([^\']+)\':', res)

    if not match:
        _lib.Fatal("Address can not be found in "+res)
    
    addr = match.group(1)
    
    _lib.FatalAssert(addr == address, "Address in a response is not same as requested. "+res)
    
    balance = [0,0,0];
    
    match = re.search( r'Approved\s+-\s+([0-9.]+)', res)

    if not match:
        _lib.Fatal("Approved Balance can not be found in "+res)
    
    balance[1] = round(float(match.group(1)),8)
    
    match = re.search( r'Total\s+-\s+([0-9.]+)', res)

    if not match:
        _lib.Fatal("Total Balance can not be found in "+res)
    
    balance[0] = round(float(match.group(1)),8)
    
    match = re.search( r'Pending\s+-\s+([0-9.-]+)', res)

    if not match:
        _lib.Fatal("Pending Balance can not be found in "+res)
    
    balance[2] = round(float(match.group(1)),8)
    
    return balance

def GetGroupBalance(datadir):
    _lib.StartTest("Request group balance for addresses on a node")
    res = _lib.ExecuteNode(['getbalances','-datadir',datadir])
    
    _lib.FatalAssertSubstr(res,"Balance for all addresses:","Balance result not printed")
    
    regex = ur"([a-z0-9A-Z]+): ([0-9.]+) .Approved - ([0-9.]+), Pending - ([0-9.]+)"

    balancesres = re.findall(regex, res)
    balances = {}
    
    for r in balancesres:
        balances[r[0]] = [round(float(r[1]),8),round(float(r[2]),8),round(float(r[3]),8)]
    
    return balances

def Send(datadir,fromaddr,to,amount):
    _lib.StartTest("Send money. From "+fromaddr+" to "+to+" amount "+str(amount))
    
    res = _lib.ExecuteNode(['send','-datadir',datadir,'-from',fromaddr,'-to',to,'-amount',str(amount)])
    
    _lib.FatalAssertSubstr(res,"Success. New transaction:","Sending of money failed. NO info about new transaction")
    
    # get transaction from this response 
    match = re.search( r'Success. New transaction: (.+)', res)

    if not match:
        _lib.Fatal("Transaction ID can not be found in "+res)
        
    txid = match.group(1)

    return txid

def SendTooMuch(datadir,fromaddr,to,amount):
    _lib.StartTest("Send too much money. From "+fromaddr+" to "+to+" amount "+str(amount))
    
    res = _lib.ExecuteNode(['send','-datadir',datadir,'-from',fromaddr,'-to',to,'-amount',str(amount)])
    
    _lib.FatalAssertSubstr(res,"No anough funds","Sending of money didn't gail as expected")