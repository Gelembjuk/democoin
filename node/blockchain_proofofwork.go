package main

import (
	"bytes"
	"crypto/sha256"
	"math"
	"math/big"

	"github.com/gelembjuk/democoin/lib"
)

var (
	maxNonce = math.MaxInt64
)

// ProofOfWork represents a proof-of-work
type ProofOfWork struct {
	block  *Block
	target *big.Int
}

// NewProofOfWork builds and returns a ProofOfWork object
// The object can be used to find a hash for the block
func NewProofOfWork(b *Block) *ProofOfWork {
	target := big.NewInt(1)
	target.Lsh(target, uint(256-targetBits))

	pow := &ProofOfWork{b, target}

	return pow
}

// Prepares data for next iteration of PoW
// this will be hashed
func (pow *ProofOfWork) prepareData() []byte {
	data := bytes.Join(
		[][]byte{
			pow.block.PrevBlockHash,
			pow.block.HashTransactions(),
			lib.IntToHex(pow.block.Timestamp),
			lib.IntToHex(int64(targetBits)),
		},
		[]byte{},
	)

	return data
}

func (pow *ProofOfWork) addNonceToPrepared(data []byte, nonce int) []byte {
	data = append(data, lib.IntToHex(int64(nonce))...)

	return data
}

// Run performs a proof-of-work
func (pow *ProofOfWork) Run() (int, []byte) {
	var hashInt big.Int
	var hash [32]byte
	nonce := 0

	predata := pow.prepareData()

	for nonce < maxNonce {
		// prepare data for next nonce
		data := pow.addNonceToPrepared(predata, nonce)
		// hash
		hash = sha256.Sum256(data)

		// check hash is what we need
		hashInt.SetBytes(hash[:])

		if hashInt.Cmp(pow.target) == -1 {
			break
		} else {
			nonce++
		}
	}

	return nonce, hash[:]
}

// Validate validates block's PoW
// It calculates hash from same data and check if it is equal to block hash
func (pow *ProofOfWork) Validate() bool {
	var hashInt big.Int

	predata := pow.prepareData()
	data := pow.addNonceToPrepared(predata, pow.block.Nonce)
	hash := sha256.Sum256(data)
	hashInt.SetBytes(hash[:])

	isValid := hashInt.Cmp(pow.target) == -1

	return isValid
}
