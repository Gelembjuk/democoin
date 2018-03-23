package transaction

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"time"

	"testing"

	"github.com/gelembjuk/democoin/lib/wallet"
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

	newTX := Transaction{nil, inputs, outputs, 0}

	layout := "2006-01-02T15:04:05.000Z"
	str := "2014-11-12T11:45:26.371Z"
	time, _ := time.Parse(layout, str)
	newTX.Time = time.UnixNano()

	newTX.Hash()

	expected := "913acaf3f296c048df72565c3940b33afdbdba1b0683bd71969491a549a86ba3"

	expectedBytes, _ := hex.DecodeString(expected)

	if bytes.Compare(expectedBytes, newTX.ID) != 0 {
		t.Fatalf("Got \n%x\nexpected\n%x", newTX.ID, expected)
	}
}

func TestSignature(t *testing.T) {
	// wallet wallet address, wallets file, transaction, input transactions
	testSets := [][]string{
		[]string{
			"1yPg8JYsMepEBSyTFj1ZhBXcZXbrzrvtM",
			"wallet_1yPg8JYsMepEBSyTFj1ZhBXcZXbrzrvtM.dat",
			"3cff810301010b5472616e73616374696f6e01ff8200010401024944010a00010356696e01ff86000104566f757401ff8a00010454696d65010400000024ff85020101155b5d7472616e73616374696f6e2e5458496e70757401ff860001ff84000040ff83030101075458496e70757401ff84000104010454786964010a000104566f757401040001095369676e6174757265010a0001065075624b6579010a00000025ff89020101165b5d7472616e73616374696f6e2e54584f757470757401ff8a0001ff8800002fff870301010854584f757470757401ff88000102010556616c7565010800010a5075624b657948617368010a000000ffacff820201012072d458be3467230355ce5d996ff980dc5922327c64bb80b3b16e348476bc8c460340f7a860a846d40e7cd834c612dedaeba6bd9bdba7fab343325e3b73f6a3811f3394e2800c00d91f08cba5cf22483c3cad93c37e31df6c0ad6a2120a99b6ab686000010201fef03f0114bdb034351ac4c903f4108ed6fc25d2079cbdf5440001fe224001140aaa38be11a67bed469208615f137beb4d5ed5fd0001f82a3d2e1a47018a8e00",
			"0fff8b040102ff8c00010401ff8200003cff810301010b5472616e73616374696f6e01ff8200010401024944010a00010356696e01ff86000104566f757401ff8a00010454696d65010400000024ff85020101155b5d7472616e73616374696f6e2e5458496e70757401ff860001ff84000040ff83030101075458496e70757401ff84000104010454786964010a000104566f757401040001095369676e6174757265010a0001065075624b6579010a00000025ff89020101165b5d7472616e73616374696f6e2e54584f757470757401ff8a0001ff8800002fff870301010854584f757470757401ff88000102010556616c7565010800010a5075624b657948617368010a0000006eff8c000100012072d458be3467230355ce5d996ff980dc5922327c64bb80b3b16e348476bc8c46010102010222546869732069732074686520696e697469616c20626c6f636b20696e20636861696e00010101fe244001140aaa38be11a67bed469208615f137beb4d5ed5fd0000",
		},
	}

	for ind, test := range testSets {
		ws := wallet.Wallets{}
		ws.WalletsFile = "testsdata/" + test[1]
		ws.LoadFromFile()

		w, e := ws.GetWallet(test[0])

		if e != nil {
			t.Fatalf("Error: %s", e.Error())
		}

		tb, _ := hex.DecodeString(test[2])
		tx := Transaction{}
		tx.DeserializeTransaction(tb)

		prevTXs := map[int]*Transaction{}
		tb, _ = hex.DecodeString(test[3])
		decoder := gob.NewDecoder(bytes.NewReader(tb))
		decoder.Decode(&prevTXs)

		signData, err := tx.PrepareSignData(prevTXs)

		if err != nil {
			t.Fatalf("Getting sign data Error: %s", err.Error())
		}

		err = tx.SignData(w.GetPrivateKey(), signData)

		if err != nil {
			t.Fatalf("Signing Error: %s", err.Error())
		}

		err = tx.Verify(prevTXs)

		if err != nil {
			t.Fatalf("Verify Error: %s", err.Error())
		}

		fmt.Printf("Wallet %d %x\n", ind, w.PublicKey)
		fmt.Println(w.PrivateKey)
	}
}
