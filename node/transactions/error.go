package transactions

// Custom errors

import (
	"fmt"
)

const TXVerifyErrorNoInput = "noinput"

type TXVerifyError struct {
	err  string
	kind string
	TX   []byte
}

func (e *TXVerifyError) Error() string {
	return fmt.Sprintf("Transaction verify failed: %s, for TX %x", e.err, e.TX)
}

func (e *TXVerifyError) GetKind() string {
	return e.kind
}

func NewTXVerifyError(err string, kind string, TX []byte) error {
	return &TXVerifyError{err, kind, TX}
}
