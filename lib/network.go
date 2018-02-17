package lib

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"strconv"
)

const Protocol = "tcp"
const NodeVersion = 1
const CommandLength = 12

// Represents a node address
type NodeAddr struct {
	Host string
	Port int
}

// Convert to string in format host:port
func (n NodeAddr) NodeAddrToString() string {
	return n.Host + ":" + strconv.Itoa(n.Port)
}

// Compare to other node address if is same
func (n NodeAddr) CompareToAddress(addr NodeAddr) bool {
	return addr.Host == n.Host && addr.Port == n.Port
}

// Converts a command to bytes in fixed length
func CommandToBytes(command string) []byte {
	var bytes [CommandLength]byte

	for i, c := range command {
		bytes[i] = byte(c)
	}

	return bytes[:]
}

// Convert bytes back to command
func BytesToCommand(bytes []byte) string {
	var command []byte

	for _, b := range bytes {
		if b != 0x0 {
			command = append(command, b)
		}
	}

	return fmt.Sprintf("%s", command)
}

// Get command part from request string
func ExtractCommand(request []byte) []byte {
	return request[:CommandLength]
}

// Encode structure to bytes
func GobEncode(data interface{}) ([]byte, error) {
	var buff bytes.Buffer

	enc := gob.NewEncoder(&buff)
	err := enc.Encode(data)
	if err != nil {
		return []byte{}, err
	}

	return buff.Bytes(), nil
}
