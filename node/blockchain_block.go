package main

import (
	"bytes"
	"encoding/gob"
	"time"

	"github.com/gelembjuk/democoin/lib"
	"github.com/gelembjuk/democoin/lib/transaction"
)

// Block represents a block in the blockchain
type Block struct {
	Timestamp     int64
	Transactions  []*transaction.Transaction
	PrevBlockHash []byte
	Hash          []byte
	Nonce         int
	Height        int
}

// short info about a block. to exchange over network
type BlockShort struct {
	PrevBlockHash []byte
	Hash          []byte
	Height        int
}

// Serialise BlockShort to bytes
func (b *BlockShort) Serialize() ([]byte, error) {
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)

	err := encoder.Encode(b)
	if err != nil {
		return nil, err
	}

	return result.Bytes(), nil
}

// Deserialize BlockShort from bytes
func (b *BlockShort) DeserializeBlock(d []byte) error {

	decoder := gob.NewDecoder(bytes.NewReader(d))
	err := decoder.Decode(&b)

	if err != nil {
		return err
	}

	return nil
}

// Returns short copy of a block. It is just hash + prevhash
func (b *Block) GetShortCopy() *BlockShort {
	bs := BlockShort{}
	bs.Hash = b.Hash[:]
	bs.PrevBlockHash = b.PrevBlockHash[:]
	bs.Height = b.Height

	return &bs
}

// Creates copy of a block
func (b *Block) Copy() *Block {
	data, _ := b.Serialize()

	bc := Block{}
	bc.DeserializeBlock(data)
	return &bc
}

// Fills a block with transactions. But without signatures
func (b *Block) PrepareNewBlock(transactions []*transaction.Transaction, prevBlockHash []byte, height int) error {
	b.Timestamp = time.Now().Unix()
	b.Transactions = transactions[:]
	b.PrevBlockHash = prevBlockHash[:]
	b.Hash = []byte{}
	b.Nonce = 0
	b.Height = height

	return nil
}

// HashTransactions returns a hash of the transactions in the block
func (b *Block) HashTransactions() ([]byte, error) {
	var transactions [][]byte

	for _, tx := range b.Transactions {
		txser, err := tx.Serialize()

		if err != nil {
			return nil, err
		}
		transactions = append(transactions, txser)
	}
	mTree := lib.NewMerkleTree(transactions)

	return mTree.RootNode.Data, nil
}

// Serialize serializes the block
func (b *Block) Serialize() ([]byte, error) {
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)

	err := encoder.Encode(b)
	if err != nil {
		return nil, err
	}

	return result.Bytes(), nil
}

// DeserializeBlock deserializes a block
func (b *Block) DeserializeBlock(d []byte) error {

	decoder := gob.NewDecoder(bytes.NewReader(d))
	err := decoder.Decode(&b)

	if err != nil {
		return err
	}

	return nil
}
