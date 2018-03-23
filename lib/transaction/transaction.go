package transaction

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
	"math"
	"math/big"
	"strings"
	"time"

	"encoding/gob"
	"fmt"

	"github.com/gelembjuk/democoin/lib"
)

// Transaction represents a Bitcoin transaction
type Transaction struct {
	ID   []byte
	Vin  []TXInput
	Vout []TXOutput
	Time int64
}

// IsCoinbase checks whether the transaction is coinbase
func (tx Transaction) IsCoinbase() bool {
	return len(tx.Vin) == 1 && len(tx.Vin[0].Txid) == 0 && tx.Vin[0].Vout == -1
}

// Serialize returns a serialized Transaction
func (tx Transaction) Serialize() ([]byte, error) {
	var encoded bytes.Buffer

	enc := gob.NewEncoder(&encoded)
	err := enc.Encode(tx)
	if err != nil {
		return nil, err
	}

	return encoded.Bytes(), nil
}

// Hash returns the hash of the Transaction
func (tx *Transaction) TimeNow() {
	tx.Time = time.Now().UTC().UnixNano()
}

// Hash returns the hash of the Transaction
func (tx *Transaction) Hash() ([]byte, error) {
	var hash [32]byte

	txCopy := *tx
	txCopy.ID = []byte{}

	txser, err := txCopy.Serialize()

	if err != nil {
		return nil, err
	}

	hash = sha256.Sum256(txser)

	tx.ID = hash[:]
	return tx.ID, nil
}

// String returns a human-readable representation of a transaction
func (tx Transaction) String() string {
	var lines []string
	from, _ := lib.PubKeyToAddres(tx.Vin[0].PubKey)
	fromhash, _ := lib.HashPubKey(tx.Vin[0].PubKey)
	to := ""
	amount := 0.0

	for _, output := range tx.Vout {
		if bytes.Compare(fromhash, output.PubKeyHash) != 0 {
			to, _ = lib.PubKeyHashToAddres(output.PubKeyHash)
			amount = output.Value
			break
		}
	}

	lines = append(lines, fmt.Sprintf("--- Transaction %x:", tx.ID))
	lines = append(lines, fmt.Sprintf("    FROM %s TO %s VALUE %f", from, to, amount))
	lines = append(lines, fmt.Sprintf("    Time %d (%s)", tx.Time, time.Unix(0, tx.Time)))

	for i, input := range tx.Vin {
		address, _ := lib.PubKeyToAddres(input.PubKey)
		lines = append(lines, fmt.Sprintf("     Input %d:", i))
		lines = append(lines, fmt.Sprintf("       TXID:      %x", input.Txid))
		lines = append(lines, fmt.Sprintf("       Out:       %d", input.Vout))
		lines = append(lines, fmt.Sprintf("       Signature: %x", input.Signature))
		lines = append(lines, fmt.Sprintf("       PubKey:    %x", input.PubKey))
		lines = append(lines, fmt.Sprintf("       Address:   %s", address))
	}

	for i, output := range tx.Vout {
		address, _ := lib.PubKeyHashToAddres(output.PubKeyHash)
		lines = append(lines, fmt.Sprintf("     Output %d:", i))
		lines = append(lines, fmt.Sprintf("       Value:  %f", output.Value))
		lines = append(lines, fmt.Sprintf("       Script: %x", output.PubKeyHash))
		lines = append(lines, fmt.Sprintf("       Address: %s", address))
	}

	return strings.Join(lines, "\n")
}

// TrimmedCopy creates a trimmed copy of Transaction to be used in signing
func (tx *Transaction) TrimmedCopy() Transaction {
	var inputs []TXInput
	var outputs []TXOutput

	for _, vin := range tx.Vin {
		inputs = append(inputs, TXInput{vin.Txid, vin.Vout, nil, nil})
	}

	for _, vout := range tx.Vout {
		outputs = append(outputs, TXOutput{vout.Value, vout.PubKeyHash})
	}

	txCopy := Transaction{tx.ID, inputs, outputs, tx.Time}

	return txCopy
}

// TrimmedCopy creates a trimmed copy of Transaction to be used in signing
func (tx *Transaction) Copy() (Transaction, error) {
	txCopy := Transaction{}

	d, err := tx.Serialize()

	if err != nil {
		return txCopy, err
	}

	err = txCopy.DeserializeTransaction(d)

	if err != nil {
		return txCopy, err
	}
	return txCopy, nil
}

// prepare data to sign as part of transaction
// this return slice of slices. Every of them must be signed for each TX Input
func (tx *Transaction) PrepareSignData(prevTXs map[int]*Transaction) ([][]byte, error) {
	if tx.IsCoinbase() {
		return nil, nil
	}

	for vinInd, vin := range tx.Vin {
		if _, ok := prevTXs[vinInd]; !ok {
			return nil, errors.New("Previous transaction is not correct")
		}
		if bytes.Compare(prevTXs[vinInd].ID, vin.Txid) != 0 {
			return nil, errors.New("Previous transaction is not correct")
		}
	}

	signdata := make([][]byte, len(tx.Vin))

	txCopy := tx.TrimmedCopy()
	txCopy.ID = []byte{}

	for inID, _ := range txCopy.Vin {
		txCopy.Vin[inID].Signature = nil
	}

	for inID, vin := range txCopy.Vin {
		prevTx := prevTXs[inID]

		txCopy.Vin[inID].PubKey = prevTx.Vout[vin.Vout].PubKeyHash

		h := md5.New()
		io.WriteString(h, fmt.Sprintf("%x\n", txCopy))
		dataToSign := h.Sum(nil)

		signdata[inID] = dataToSign

		txCopy.Vin[inID].PubKey = nil
	}

	return signdata, nil
}

// Sign Inouts for transaction
// DataToSign is output of the function PrepareSignData
func (tx *Transaction) SignData(privKey ecdsa.PrivateKey, DataToSign [][]byte) error {
	if tx.IsCoinbase() {
		return nil
	}

	for inID, _ := range tx.Vin {
		dataToSign := DataToSign[inID]

		r, s, err := ecdsa.Sign(rand.Reader, &privKey, dataToSign)

		if err != nil {
			return err
		}
		signature := append(r.Bytes(), s.Bytes()...)

		tx.Vin[inID].Signature = signature
	}
	// when transaction i complete, we can add ID to it
	tx.Hash()

	return nil
}

// Verify verifies signatures of Transaction inputs
// And total amount of inputs and outputs
func (tx *Transaction) Verify(prevTXs map[int]*Transaction) error {
	if tx.IsCoinbase() {
		// coinbase has only 1 output and it must have value equal to constant
		if tx.Vout[0].Value != lib.PaymentForBlockMade {
			return errors.New("Value of coinbase transaction is wrong")
		}
		if len(tx.Vout) > 1 {
			return errors.New("Coinbase transaction can have only 1 output")
		}
		return nil
	}
	// calculate total input
	totalinput := float64(0)

	for vind, vin := range tx.Vin {
		if prevTXs[vind].ID == nil {
			return errors.New("Previous transaction is not correct")
		}
		amount := prevTXs[vind].Vout[vin.Vout].Value
		totalinput += amount
	}

	txCopy := tx.TrimmedCopy()
	txCopy.ID = []byte{}

	curve := elliptic.P256()

	for inID, _ := range tx.Vin {
		txCopy.Vin[inID].Signature = nil
		txCopy.Vin[inID].PubKey = nil
	}
	for inID, vin := range tx.Vin {
		// full input transaction
		prevTx := prevTXs[inID]

		//hash of key who signed this input
		signPubKeyHash, _ := lib.HashPubKey(vin.PubKey)

		if bytes.Compare(prevTx.Vout[vin.Vout].PubKeyHash, signPubKeyHash) != 0 {
			return errors.New(fmt.Sprintf("Sign Key Hash for input %x is different from output hash", vin.Txid))
		}

		// replace pub key with its hash. same was done when signing
		txCopy.Vin[inID].PubKey = prevTx.Vout[vin.Vout].PubKeyHash

		// build key and verify data
		r := big.Int{}
		s := big.Int{}
		sigLen := len(vin.Signature)
		r.SetBytes(vin.Signature[:(sigLen / 2)])
		s.SetBytes(vin.Signature[(sigLen / 2):])

		x := big.Int{}
		y := big.Int{}
		keyLen := len(vin.PubKey)
		x.SetBytes(vin.PubKey[:(keyLen / 2)])
		y.SetBytes(vin.PubKey[(keyLen / 2):])

		h := md5.New()
		io.WriteString(h, fmt.Sprintf("%x\n", txCopy))
		dataToVerify := h.Sum(nil)

		rawPubKey := ecdsa.PublicKey{Curve: curve, X: &x, Y: &y}

		if ecdsa.Verify(&rawPubKey, dataToVerify, &r, &s) == false {
			return errors.New(fmt.Sprintf("Signatire doe not match for TX %x . \nData to verify %x\nPubKey: %x", vin.Txid, dataToVerify, vin.PubKey))
		}
		txCopy.Vin[inID].PubKey = nil
	}

	// calculate total output of transaction
	totaloutput := float64(0)

	for _, vout := range tx.Vout {
		if vout.Value < lib.SmallestUnit {
			return errors.New(fmt.Sprintf("Too small output value %f", vout.Value))
		}
		totaloutput += vout.Value
	}

	if math.Abs(totalinput-totaloutput) >= lib.SmallestUnit {
		return errors.New(fmt.Sprintf("Input and output values of a transaction are not same: %.10f vs %.10f . Diff %.10f", totalinput, totaloutput, totalinput-totaloutput))
	}

	return nil
}

/*
* Make a transaction to be coinbase.
 */
func (tx *Transaction) MakeCoinbaseTX(to, data string) error {
	if data == "" {
		randData := make([]byte, 20)
		_, err := rand.Read(randData)

		if err != nil {
			return err
		}

		data = fmt.Sprintf("%x", randData)
	}

	txin := TXInput{[]byte{}, -1, nil, []byte(data)}
	txout := NewTXOutput(lib.PaymentForBlockMade, to)
	tx.Vin = []TXInput{txin}
	tx.Vout = []TXOutput{*txout}

	tx.Hash()

	return nil
}

// DeserializeTransaction deserializes a transaction
func (tx *Transaction) DeserializeTransaction(data []byte) error {
	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(tx)

	if err != nil {
		return err
	}

	return nil
}

// Sorting of transactions slice
type Transactions []*Transaction

func (c Transactions) Len() int           { return len(c) }
func (c Transactions) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
func (c Transactions) Less(i, j int) bool { return c[i].Time < c[j].Time }
