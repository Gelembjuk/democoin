package lib

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"

	"golang.org/x/crypto/ripemd160"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// Structure to manage logs
type LoggerMan struct {
	Trace   *log.Logger
	Info    *log.Logger
	Warning *log.Logger
	Error   *log.Logger
}

// Creates logger object. sets all logging to STDOUT
func CreateLogger() *LoggerMan {
	logger := LoggerMan{}

	logger.Trace = log.New(os.Stdout,
		"TRACE: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	logger.Info = log.New(os.Stdout,
		"INFO: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	logger.Warning = log.New(os.Stdout,
		"WARNING: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	logger.Error = log.New(os.Stdout,
		"ERROR: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	return &logger
}

// Changes logging to files
func (logger *LoggerMan) LogToFiles(datadir, trace, info, warning, errorname string) error {

	f1, err1 := os.OpenFile(datadir+trace, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)

	if err1 == nil {
		logger.Trace.SetOutput(f1)
	}

	f2, err2 := os.OpenFile(datadir+info, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)

	if err2 == nil {
		logger.Info.SetOutput(f2)
	}

	f3, err3 := os.OpenFile(datadir+warning, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)

	if err3 == nil {
		logger.Warning.SetOutput(f3)
	}

	f4, err4 := os.OpenFile(datadir+errorname, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)

	if err4 == nil {
		logger.Error.SetOutput(f4)
	}
	return nil
}

// Sets ogging to STDOUT
func (logger *LoggerMan) LogToStdout() error {
	logger.Trace.SetOutput(os.Stdout)
	logger.Info.SetOutput(os.Stdout)
	logger.Warning.SetOutput(os.Stdout)
	logger.Error.SetOutput(os.Stdout)
	return nil
}

// IntToHex converts an int64 to a byte array
func IntToHex(num int64) []byte {
	buff := new(bytes.Buffer)
	err := binary.Write(buff, binary.BigEndian, num)
	if err != nil {
		log.Panic(err)
	}

	return buff.Bytes()
}

// ReverseBytes reverses a byte array
func ReverseBytes(data []byte) {
	for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {
		data[i], data[j] = data[j], data[i]
	}
}

// Converts address string to hash of pubkey
func AddresToPubKeyHash(address string) ([]byte, error) {
	pubKeyHash := Base58Decode([]byte(address))

	if len(pubKeyHash) < 10 {
		return nil, errors.New("Wrong address")
	}

	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-4]

	return pubKeyHash, nil
}

// Converts hash of pubkey to address as a string
func PubKeyHashToAddres(pubKeyHash []byte) (string, error) {
	versionedPayload := append([]byte{Version}, pubKeyHash...)

	checksum := Checksum(versionedPayload)

	fullPayload := append(versionedPayload, checksum...)
	address := Base58Encode(fullPayload)

	return fmt.Sprintf("%s", address), nil
}

// Makes string adres from pub key
func PubKeyToAddres(pubKey []byte) (string, error) {
	pubKeyHash, err := HashPubKey(pubKey)

	if err != nil {
		return "", err
	}
	versionedPayload := append([]byte{Version}, pubKeyHash...)

	checksum := Checksum(versionedPayload)

	fullPayload := append(versionedPayload, checksum...)
	address := Base58Encode(fullPayload)

	return fmt.Sprintf("%s", address), nil
}

// Checksum generates a checksum for a public key
func Checksum(payload []byte) []byte {
	firstSHA := sha256.Sum256(payload)
	secondSHA := sha256.Sum256(firstSHA[:])

	return secondSHA[:AddressChecksumLen]
}

// HashPubKey hashes public key
func HashPubKey(pubKey []byte) ([]byte, error) {
	publicSHA256 := sha256.Sum256(pubKey)

	RIPEMD160Hasher := ripemd160.New()
	_, err := RIPEMD160Hasher.Write(publicSHA256[:])
	if err != nil {
		return nil, err
	}
	publicRIPEMD160 := RIPEMD160Hasher.Sum(nil)

	return publicRIPEMD160, nil
}

func RandString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
