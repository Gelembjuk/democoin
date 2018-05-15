package nodemanager

import (
	"github.com/gelembjuk/democoin/lib/net"
)

type NodesListStorage struct {
	DBConn *Database
}

func (s NodesListStorage) GetNodes() ([]net.NodeAddr, error) {
	s.DBConn.OpenConnection("get nodes")

	defer s.DBConn.CloseConnection()

	nddb, err := s.DBConn.DB.GetNodesObject()

	if err != nil {
		return nil, err
	}

	nodes := []net.NodeAddr{}

	nddb.ForEach(func(k, v []byte) {
		addr := string(v)
		node := net.NodeAddr{}
		node.LoadFromString(addr)

		nodes = append(nodes, node)

	})

	return nodes, nil
}
func (s NodesListStorage) AddNodeToKnown(addr net.NodeAddr) {
	s.DBConn.OpenConnection("add node")

	defer s.DBConn.CloseConnection()

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
	s.DBConn.OpenConnection("remove node")

	defer s.DBConn.CloseConnection()

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
	s.DBConn.OpenConnection("get count of nodes")

	defer s.DBConn.CloseConnection()

	nddb, err := s.DBConn.DB.GetNodesObject()

	if err != nil {
		return 0, err
	}

	count := 0

	nddb.ForEach(func(k, v []byte) {
		count++
	})

	return count, nil
}
