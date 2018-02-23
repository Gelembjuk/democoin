package transaction

import (
	"bytes"
	"fmt"
	"strings"
)
import "github.com/gelembjuk/democoin/lib"

// TXInput represents a transaction input
type TXInput struct {
	Txid      []byte
	Vout      int
	Signature []byte
	PubKey    []byte // this is the wallet who spends transaction
}

// UsesKey checks whether the address initiated the transaction
func (in *TXInput) UsesKey(pubKeyHash []byte) bool {
	lockingHash, _ := lib.HashPubKey(in.PubKey)

	return bytes.Compare(lockingHash, pubKeyHash) == 0
}

func (input TXInput) String() string {
	lines := []string{}

	lines = append(lines, fmt.Sprintf("       TXID:      %x", input.Txid))
	lines = append(lines, fmt.Sprintf("       Out:       %d", input.Vout))
	lines = append(lines, fmt.Sprintf("       Signature: %x", input.Signature))
	lines = append(lines, fmt.Sprintf("       PubKey:    %x", input.PubKey))

	return strings.Join(lines, "\n")
}
