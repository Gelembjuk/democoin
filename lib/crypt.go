package lib

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	"crypto/rand"
	"io"
	"math/big"
)

func SignData(privKey ecdsa.PrivateKey, dataToSign []byte) ([]byte, error) {
	h := md5.New()
	str := string(dataToSign)
	io.WriteString(h, str)
	data := h.Sum(nil)

	r, s, err := ecdsa.Sign(rand.Reader, &privKey, data)

	if err != nil {
		return nil, err
	}
	signature := append(r.Bytes(), s.Bytes()...)

	return signature, nil
}

func VerifySignature(signature []byte, message []byte, PubKey []byte) (bool, error) {
	h := md5.New()
	str := string(message)
	io.WriteString(h, str)
	data := h.Sum(nil)

	// build key and verify data
	r := big.Int{}
	s := big.Int{}
	sigLen := len(signature)
	r.SetBytes(signature[:(sigLen / 2)])
	s.SetBytes(signature[(sigLen / 2):])

	x := big.Int{}
	y := big.Int{}
	keyLen := len(PubKey)
	x.SetBytes(PubKey[:(keyLen / 2)])
	y.SetBytes(PubKey[(keyLen / 2):])

	curve := elliptic.P256()

	rawPubKey := ecdsa.PublicKey{Curve: curve, X: &x, Y: &y}

	return ecdsa.Verify(&rawPubKey, data, &r, &s), nil
}
