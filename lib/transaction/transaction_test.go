package transaction

import (
	"bytes"

	"testing"
)

func TestHash(t *testing.T) {
	PubKey := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}

	inputs := []TXInput{
		TXInput{[]byte{1, 2, 3}, 0, []byte{}, PubKey},
		TXInput{[]byte{4, 5, 6}, 1, []byte{}, PubKey},
	}

	outputs := []TXOutput{
		TXOutput{1, []byte{4, 3, 2, 1}},
		TXOutput{2, PubKey},
	}

	newTX := Transaction{nil, inputs, outputs}

	newTX.Hash()

	expected := []byte{1, 242, 163, 245, 228, 60, 148, 31, 53, 134, 209, 63, 108, 0, 212, 191, 154, 252, 1, 41, 177, 23, 138, 121, 41, 221, 116, 132, 197, 208, 167, 106}

	if bytes.Compare(expected, newTX.ID) != 0 {
		t.Fatalf("Got \n%x\nexpected\n%x", newTX.ID, expected)
	}
}
