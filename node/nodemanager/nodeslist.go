package nodemanager

import (
	"github.com/gelembjuk/democoin/lib/net"
)

type NodesListStorage struct {
	DBConn    *Database
	SessionID string
}

func (s NodesListStorage) GetNodes() ([]net.NodeAddr, error) {
	if s.DBConn.OpenConnectionIfNeeded("GetNodes", s.SessionID) {
		defer s.DBConn.CloseConnection()
	}
	nddb, err := s.DBConn.DB.GetNodesObject()

	if err != nil {
		return nil, err
	}

	nodes := []net.NodeAddr{}

	nddb.ForEach(func(k, v []byte) error {
		addr := string(v)
		node := net.NodeAddr{}
		node.LoadFromString(addr)

		nodes = append(nodes, node)
		return nil
	})

	return nodes, nil
}
func (s NodesListStorage) AddNodeToKnown(addr net.NodeAddr) {
	s.DBConn.Logger.Trace.Printf("AddNodeToKnown %s", addr.NodeAddrToString())

	if s.DBConn.OpenConnectionIfNeeded("AddNode", s.SessionID) {

		defer s.DBConn.CloseConnection()
	}
	nddb, err := s.DBConn.DB.GetNodesObject()

	if err != nil {
		return
	}
	address := addr.NodeAddrToString()
	key := []byte(address)

	nddb.PutNode(key, key)

	return
}
func (s NodesListStorage) RemoveNodeFromKnown(addr net.NodeAddr) {
	if s.DBConn.OpenConnectionIfNeeded("RemoveNode", s.SessionID) {
		defer s.DBConn.CloseConnection()
	}
	nddb, err := s.DBConn.DB.GetNodesObject()

	if err != nil {
		return
	}
	address := addr.NodeAddrToString()
	key := []byte(address)
	nddb.DeleteNode(key)
	return
}
func (s NodesListStorage) GetCountOfKnownNodes() (int, error) {
	if s.DBConn.OpenConnectionIfNeeded("GetCount", s.SessionID) {
		defer s.DBConn.CloseConnection()
	}
	nddb, err := s.DBConn.DB.GetNodesObject()

	if err != nil {
		return 0, err
	}

	return nddb.GetCount()
}
