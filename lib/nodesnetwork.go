package lib

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
)

// INterface for extra storage for a nodes.
// TODO
// This is not used yet
type NodeNetworkStorage interface {
	LoadNodes(nodeslist *[]NodeAddr) error
	AddNodeToKnown(addr NodeAddr)
	RemoveNodeFromKnown(addr NodeAddr)
	GetCountOfKnownNodes() int
}

// This manages list of known nodes by a node
type NodeNetwork struct {
	Logger  *LoggerMan
	Nodes   []NodeAddr
	Storage NodeNetworkStorage
}

type NodesListJSON struct {
	Nodes []NodeAddr
}

// Set extra storage for a nodes
func (n *NodeNetwork) SetExtraManager(storage NodeNetworkStorage) {
	n.Storage = storage
}

// Set nodes list. This can be used to do initial nodes loading from  config or so
func (n *NodeNetwork) LoadNodes(nodes []NodeAddr, replace bool) {
	if replace {
		n.Nodes = nodes
	} else {
		n.Nodes = append(n.Nodes, nodes...)
	}

	if n.Storage != nil {
		// remember what is not yet remembered
		for _, node := range nodes {
			n.Storage.AddNodeToKnown(node)
		}
		n.Storage.LoadNodes(&n.Nodes)
	}
}

// If n any known nodes then it will be loaded from the url on a host
func (n *NodeNetwork) LoadInitialNodes() error {
	response, err := http.Get(InitialNodesList)

	if err != nil {
		return err
	}

	jsondoc, err := ioutil.ReadAll(response.Body)

	if err != nil {
		return err
	}

	nodes := NodesListJSON{}

	err = json.Unmarshal(jsondoc, &nodes)

	if err != nil {
		return err
	}

	n.Nodes = append(n.Nodes, nodes.Nodes...)

	if n.Storage != nil {
		// remember loaded nodes in local storage
		for _, node := range nodes.Nodes {
			node.Host = strings.Trim(node.Host, " ")
			n.Storage.AddNodeToKnown(node)
		}
	}

	return nil
}
func (n *NodeNetwork) GetNodes() []NodeAddr {
	return n.Nodes
}

// Returns number of known nodes
func (n *NodeNetwork) GetCountOfKnownNodes() int {
	l := len(n.Nodes)

	if n.Storage != nil {
		l += n.Storage.GetCountOfKnownNodes()
	}

	return l
}

// Check if node address is known
func (n *NodeNetwork) CheckIsKnown(addr NodeAddr) bool {
	exists := false

	for _, node := range n.Nodes {
		if node.CompareToAddress(addr) {
			exists = true
			break
		}
	}

	return exists
}

/*
* Checks if a node exists in list of known nodes and adds it if no
* Returns true if was added
 */
func (n *NodeNetwork) AddNodeToKnown(addr NodeAddr) bool {
	exists := false

	for _, node := range n.Nodes {
		if node.CompareToAddress(addr) {
			exists = true
			break
		}
	}
	if !exists {
		n.Nodes = append(n.Nodes, addr)
	}

	if n.Storage != nil {
		n.Storage.AddNodeToKnown(addr)
	}

	return !exists
}

// Removes a node from known
func (n *NodeNetwork) RemoveNodeFromKnown(addr NodeAddr) {

	updatedlist := []NodeAddr{}

	for _, node := range n.Nodes {
		if !node.CompareToAddress(addr) {
			updatedlist = append(updatedlist, node)
		}
	}

	n.Nodes = updatedlist

	if n.Storage != nil {
		n.Storage.RemoveNodeFromKnown(addr)
	}
}
