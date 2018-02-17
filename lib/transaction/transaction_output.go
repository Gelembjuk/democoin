package transaction

import (
	"bytes"
	"encoding/gob"
	"log"

	"github.com/gelembjuk/democoin/lib"
)

// TXOutput represents a transaction output
type TXOutput struct {
	Value      float64
	PubKeyHash []byte
}

// Simplified output format. To use externally
// It has all info in human readable format
// this can be used to display info abut outputs wihout references to transaction object
type TXOutputIndependent struct {
	Value          float64
	DestPubKeyHash []byte
	SendPubKeyHash []byte
	TXID           []byte
	OIndex         int
	IsBase         bool
}

// Lock signs the output
func (out *TXOutput) Lock(address []byte) {
	pubKeyHash := lib.Base58Decode(address)
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-4]
	out.PubKeyHash = pubKeyHash
}

// IsLockedWithKey checks if the output can be used by the owner of the pubkey
func (out *TXOutput) IsLockedWithKey(pubKeyHash []byte) bool {
	return bytes.Compare(out.PubKeyHash, pubKeyHash) == 0
}

// Same as IsLockedWithKey but for simpler structure
func (out *TXOutputIndependent) IsLockedWithKey(pubKeyHash []byte) bool {
	return bytes.Compare(out.DestPubKeyHash, pubKeyHash) == 0
}

// build independed transaction from normal output
func (out *TXOutputIndependent) LoadFromSimple(sout TXOutput, txid []byte, ind int, sender []byte, iscoinbase bool) {
	out.OIndex = ind
	out.DestPubKeyHash = sout.PubKeyHash
	out.SendPubKeyHash = sender
	out.Value = sout.Value
	out.TXID = txid
	out.IsBase = iscoinbase
}

// NewTXOutput create a new TXOutput
func NewTXOutput(value float64, address string) *TXOutput {
	txo := &TXOutput{value, nil}
	txo.Lock([]byte(address))

	return txo
}

// TXOutputs collects TXOutput
type TXOutputs struct {
	Outputs []TXOutput
}

// Serialize serializes TXOutputs
func (outs TXOutputs) Serialize() []byte {
	var buff bytes.Buffer

	enc := gob.NewEncoder(&buff)
	err := enc.Encode(outs)
	if err != nil {
		log.Panic(err)
	}

	return buff.Bytes()
}

// DeserializeOutputs deserializes TXOutputs
func DeserializeOutputs(data []byte) TXOutputs {
	var outputs TXOutputs

	dec := gob.NewDecoder(bytes.NewReader(data))
	err := dec.Decode(&outputs)
	if err != nil {
		log.Panic(err)
	}

	return outputs
}
