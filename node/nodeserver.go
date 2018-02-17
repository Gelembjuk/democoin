package main

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/gelembjuk/democoin/lib"
	"github.com/gelembjuk/democoin/lib/nodeclient"
)

type NodeServer struct {
	DataDir string
	Node    *Node

	NodeAddress lib.NodeAddr

	BlocksInTransit [][]byte

	Logger *lib.LoggerMan
	// Channels to manipulate roitunes
	StopMainChan        chan struct{}
	StopMainConfirmChan chan struct{}
	BlockBilderChan     chan []byte
}

func (s *NodeServer) GetClient() *nodeclient.NodeClient {

	return s.Node.NodeClient
}

/*
* Reads and parses request from network data
 */
func (s *NodeServer) parseRequestData(request []byte, payload interface{}) error {
	var buff bytes.Buffer

	buff.Write(request)
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(payload)

	if err != nil {
		return errors.New("1. " + err.Error())
	}

	return nil
}

/*
* Reads and parses request from network data
 */
func (s *NodeServer) readRequest(conn net.Conn) (string, []byte, error) {
	// 1. Read command
	commandbuffer := make([]byte, lib.CommandLength)
	read, err := conn.Read(commandbuffer)

	if err != nil {
		return "", nil, err
	}

	if read != lib.CommandLength {
		return "", nil, errors.New("Wrong number of bytes received for a request")
	}

	command := lib.BytesToCommand(commandbuffer)

	// 2. Get length of command data

	lengthbuffer := make([]byte, 4)

	read, err = conn.Read(lengthbuffer)

	if err != nil {
		return "", nil, err
	}

	if read != 4 {
		return "", nil, errors.New("Wrong number of bytes received for a request")
	}
	var datalength uint32
	binary.Read(bytes.NewReader(lengthbuffer), binary.LittleEndian, &datalength)

	// 3. read command data by length
	databuffer := make([]byte, datalength)

	if datalength > 0 {
		read, err = conn.Read(databuffer)

		if err != nil {
			return "", nil, err
		}

		if uint32(read) != datalength {
			return "", nil, errors.New(fmt.Sprintf("Expected %d bytes, but received %d", datalength, read))
		}
	}

	return command, databuffer, nil
}

/*
* handle received data. It can be one way command or a request for some data
 */
func (s *NodeServer) handleConnection(conn net.Conn) {
	s.Logger.Trace.Printf("New command. Start reading\n")

	command, request, err := s.readRequest(conn)

	if err != nil {
		s.Logger.Error.Println("Network Data Reading Error: ", err.Error())
		return
	}

	s.Logger.Trace.Printf("Received %s command\n", command)

	expectsresponse := false

	// open blockchain. and close in the end ofthis function
	err = s.Node.OpenBlockchain()

	if err != nil {
		s.Logger.Error.Println("Can not open blockchain: ", err.Error())
		return
	}

	defer s.Node.CloseBlockchain() // blockchain is opened while this function is runnning

	var rerr error
	var response []byte
	response = nil

	switch command {
	case "addr":
		rerr = s.handleAddr(request)
	case "viod":
		// do nothing
		s.Logger.Trace.Println("Void command reveived")
	case "block":
		rerr = s.handleBlock(request)
	case "inv":
		rerr = s.handleInv(request)
	case "getblocks":
		rerr = s.handleGetBlocks(request)

	case "getblocksup":
		rerr = s.handleGetBlocksUpper(request)

	case "getdata":
		rerr = s.handleGetData(request)

	case "getunspent":
		expectsresponse = true
		response, rerr = s.handleGetUnspent(request)

	case "gethistory":
		expectsresponse = true
		response, rerr = s.handleGetHistory(request)

	case "getfblocks":
		expectsresponse = true
		response, rerr = s.handleGetFirstBlocks()

	case "tx":
		rerr = s.handleTx(request)

	case "txfull":
		expectsresponse = true
		response, rerr = s.handleTxFull(request)

	case "txrequest":
		expectsresponse = true
		response, rerr = s.handleTxRequest(request)

	case "version":
		rerr = s.handleVersion(request)
	default:
		rerr = errors.New("Unknown command!")
	}

	if rerr != nil {
		s.Logger.Error.Println("Network Command Handle Error: ", rerr.Error())

		if expectsresponse {
			// return error to the client
			// first byte is bool false to indicate there was error
			payload, err := lib.GobEncode(rerr.Error())

			if err == nil {
				dataresponse := append([]byte{0}, payload...)

				s.Logger.Trace.Printf("Responding %d bytes as error message\n", len(dataresponse))

				_, err := conn.Write(dataresponse)

				if err != nil {
					s.Logger.Error.Println("Sending response error: ", err.Error())
				}
			}

		}
	}

	if response != nil && rerr == nil {
		// send this response back
		// first byte is bool true to indicate request was success
		dataresponse := append([]byte{1}, response...)

		s.Logger.Trace.Printf("Responding %d bytes\n", len(dataresponse))

		_, err := conn.Write(dataresponse)

		if err != nil {
			s.Logger.Error.Println("Sending response error: ", err.Error())
		}
	}
	s.Logger.Trace.Printf("Complete processing %s command\n", command)
	conn.Close()
}

/*
* Starts a server for node. It listens TPC port and communicates with other nodes and lite clients
 */
func (s *NodeServer) StartServer() error {
	s.Logger.Trace.Println("Prepare server to start ", s.NodeAddress.NodeAddrToString())

	ln, err := net.Listen(lib.Protocol, ":"+strconv.Itoa(s.NodeAddress.Port))

	if err != nil {
		return err
	}
	defer ln.Close()

	// client will use the address to include it in requests
	s.Node.NodeClient.SetNodeAddress(s.NodeAddress)

	s.Node.SendVersionToNodes([]lib.NodeAddr{})

	s.Logger.Trace.Println("Start block bilding routine")
	s.BlockBilderChan = make(chan []byte)

	go s.BlockBuilder()

	s.Logger.Trace.Println("Start listening connections on port ", s.NodeAddress.Port)

	for {
		conn, err := ln.Accept()

		if err != nil {
			return err
		}
		// check if is a time to stop this loop
		stop := false

		// check if a channel is still open. It can be closed in agoroutine when receive external stop signal
		select {
		case _, ok := <-s.StopMainChan:

			if !ok {
				stop = true
			}
		default:
		}

		if stop {

			// complete all tasks. save data if needed
			ln.Close()

			close(s.StopMainConfirmChan)

			s.BlockBilderChan <- []byte{} // send signal to block building thread to exit
			// empty slice means this is exit signal

			s.Logger.Trace.Println("Stop Listing Network. Correct exit")
			break
		}

		go s.handleConnection(conn)
	}
	return nil
}

/*
* Sends signal to routine where we make blocks. This makes the routine to check transactions in unapproved cache
* And try to make a block if there are enough transactions
 */
func (s *NodeServer) TryToMakeNewBlock(tx []byte) {
	s.BlockBilderChan <- tx // send signal to block building thread to try to make new block now
}

/*
* The routine that tries to make blocks.
* The routine reads last added transaction ID
* The ID will be real tranaction ID only if this transaction wa new created on this node
* in this case, if block is not created, the transaction will be sent to all other nodes
* it is needed to delay sending of transaction to be able to create a block first, before all other eceive new transaction
* This ID can be also {0} (one byte slice). it means try to create a block but don't send transaction
* and it can be empty slice . it means to exit from teh routibe
 */
func (s *NodeServer) BlockBuilder() {
	for {
		txID := <-s.BlockBilderChan

		s.Logger.Trace.Printf("BlockBuilder new transaction %x", txID)

		if len(txID) == 0 {
			// this is return signal from main thread
			close(s.BlockBilderChan)
			s.Logger.Trace.Printf("Exit BlockBuilder thread")
			return
		}

		s.Logger.Trace.Printf("Go to make new block attempt")
		// try to buid new block
		newBlockHash, err := s.Node.TryToMakeBlock()

		if err != nil {
			s.Logger.Trace.Printf("Block building error %s\n", err.Error())
		}

		s.Logger.Trace.Printf("Attempt finished")

		if newBlockHash == nil && len(txID) > 1 {
			s.Logger.Trace.Printf("Send this new transaction to all other")
			// block was not created and txID is real transaction ID
			// send this transaction to all other nodes.
			// blockchain should be closed in this place
			s.Node.OpenBlockchainIfClosed()

			defer s.Node.CloseBlockchainIfWasOpen()

			tx, err := s.Node.NodeTX.UnapprovedTXs.GetIfExists(txID)

			if err == nil && tx != nil {
				s.Logger.Trace.Printf("Sending...")
				s.Node.SendTransactionToAll(tx)
			} else if err != nil {
				s.Logger.Trace.Printf("Error: %s", err.Error())
			} else if tx == nil {
				s.Logger.Trace.Printf("Error: TX %x is not found", txID)
			}
		}
	}
}
