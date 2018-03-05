import _lib
import re

def GetBalanceWallet(datadir, address, host, port):
    _lib.StartTest("Request balance for a wallet "+address)
    res = _lib.ExecuteWallet(['getbalance','-datadir',datadir,"-address",address,"-nodehost",host,"-nodeport",port])
    _lib.FatalAssertSubstr(res,"Balance of","Balance info is not found")

    # get balance from this response 
    match = re.match( r'Balance of \'(.+)\': ([0-9.]+)', res)

    if not match:
        _lib.Fatal("Balance can not be found in "+res)
    
    addr = match.group(1)
    
    _lib.FatalAssert(addr == address, "Address in a response is not same as requested. "+res)
    
    balance = match.group(2)
    
    return round(float(balance),8)

def GetGroupBalanceWallet(datadir,host,port):
    _lib.StartTest("Request group balance for addresses in a wallet")
    res = _lib.ExecuteWallet(['listbalances','-datadir',datadir,"-nodehost",host,"-nodeport",port])
    _lib.FatalAssertSubstr(res,"Balance for all addresses:","Balance result not printed")
    
    regex = ur"([a-z0-9A-Z]+): ([0-9.]+)"

    balancesres = re.findall(regex, res)
    
    balances = {}
    
    for r in balancesres:
        balances[r[0]] = round(float(r[1]),8)
    
    return balances

def CreateWallet(datadir):
    _lib.StartTest("Create new wallet")
    res = _lib.ExecuteWallet(['createwallet','-datadir',datadir])
    _lib.FatalAssertSubstr(res,"Your new address","Address creation failed")
    match = re.match( r'.+: (.+)', res)

    if not match:
        _lib.Fatal("Address can not be found in "+res)
        
    address = match.group(1)
    
    return address

def Send(datadir,fromaddr,to,amount,host,port):
    _lib.StartTest("Send money. From "+fromaddr+" to "+to+" amount "+str(amount))
    
    res = _lib.ExecuteWallet(['send','-datadir',datadir,'-from',fromaddr,'-to',to,'-amount',str(amount),"-nodehost",host,"-nodeport",port])
    
    _lib.FatalAssertSubstr(res,"Success. New transaction:","Sending of money failed. NO info about new transaction")
    
    # get transaction from this response 
    match = re.match( r'Success. New transaction: (.+)', res)

    if not match:
        _lib.Fatal("Transaction ID can not be found in "+res)
        
    txid = match.group(1)

    return txid

def SendNoNode(datadir,fromaddr,to,amount):
    _lib.StartTest("Send money. From "+fromaddr+" to "+to+" amount "+str(amount))
    
    res = _lib.ExecuteWallet(['send','-datadir',datadir,'-from',fromaddr,'-to',to,'-amount',str(amount)])
    
    _lib.FatalAssertSubstr(res,"Success. New transaction:","Sending of money failed. NO info about new transaction")
    
    # get transaction from this response 
    match = re.match( r'Success. New transaction: (.+)', res)

    if not match:
        _lib.Fatal("Transaction ID can not be found in "+res)
        
    txid = match.group(1)

    return txid

def SendTooMuch(datadir,fromaddr,to,amount,host,port):
    _lib.StartTest("Send too much money. From "+fromaddr+" to "+to+" amount "+str(amount))
    res = _lib.ExecuteNode(['send','-datadir',datadir,'-from',fromaddr,'-to',to,'-amount',str(amount),"-nodehost",host,"-nodeport",port])
    _lib.FatalAssertSubstr(res,"No anough funds","Sending of money didn't gail as expected")
    
def SendTooMuchNoNode(datadir,fromaddr,to,amount):
    _lib.StartTest("Send too much money. From "+fromaddr+" to "+to+" amount "+str(amount))
    
    res = _lib.ExecuteNode(['send','-datadir',datadir,'-from',fromaddr,'-to',to,'-amount',str(amount),"-nodehost",host,"-nodeport",port])
    
    _lib.FatalAssertSubstr(res,"No anough funds","Sending of money didn't gail as expected")