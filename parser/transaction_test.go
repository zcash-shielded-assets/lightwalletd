// Copyright (c) 2019-2020 The Zcash developers
// Distributed under the MIT software license, see the accompanying
// file COPYING or https://www.opensource.org/licenses/mit-license.php .
package parser

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

// Some of these values may be "null" (which translates to nil in Go) in
// the test data, so we have *_set variables to indicate if the corresponding
// variable is non-null. (There is an "optional" package we could use for
// these but it doesn't seem worth pulling it in.)
type TxTestData struct {
	Tx                 string
	Txid               string
	Version            int
	NVersionGroupId    int
	NConsensusBranchId int
	Tx_in_count        int
	Tx_out_count       int
	NSpendsSapling     int
	NoutputsSapling    int
	NActionsOrchard    int
}

// https://jhall.io/posts/go-json-tricks-array-as-structs/
func (r *TxTestData) UnmarshalJSON(p []byte) error {
	var t []any
	if err := json.Unmarshal(p, &t); err != nil {
		return err
	}
	// Skip comment rows (they have fewer elements than expected)
	if len(t) < 15 {
		return nil
	}
	// Handle null values for optional fields
	if t[0] != nil {
		r.Tx = t[0].(string)
	}
	if t[1] != nil {
		r.Txid = t[1].(string)
	}
	r.Version = int(toFloat64(t[2]))
	r.NVersionGroupId = int(toFloat64(t[3]))
	r.NConsensusBranchId = int(toFloat64(t[4]))
	r.Tx_in_count = int(toFloat64(t[7]))
	r.Tx_out_count = int(toFloat64(t[8]))
	r.NSpendsSapling = int(toFloat64(t[9]))
	r.NoutputsSapling = int(toFloat64(t[10]))
	r.NActionsOrchard = int(toFloat64(t[14]))
	return nil
}

func toFloat64(v any) float64 {
	if v == nil {
		return 0
	}
	return v.(float64)
}

func TestV5TransactionParser(t *testing.T) {
	// The raw data are stored in a separate file because they're large enough
	// to make the test table difficult to scroll through. They are in the same
	// order as the test table above. If you update the test table without
	// adding a line to the raw file, this test will panic due to index
	// misalignment.
	s, err := os.ReadFile("../testdata/tx_v5.json")
	if err != nil {
		t.Fatal(err)
	}

	var testdata []json.RawMessage
	err = json.Unmarshal(s, &testdata)
	if err != nil {
		t.Fatal(err)
	}
	if len(testdata) < 3 {
		t.Fatal("tx_vt.json has too few lines")
	}
	testdata = testdata[2:]
	for _, onetx := range testdata {
		var txtestdata TxTestData

		err = json.Unmarshal(onetx, &txtestdata)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("txid %s", txtestdata.Txid)
		rawTxData, _ := hex.DecodeString(txtestdata.Tx)

		tx := NewTransaction()
		rest, err := tx.ParseFromSlice(rawTxData)
		if err != nil {
			t.Fatalf("%v", err)
		}
		if len(rest) != 0 {
			t.Fatalf("Test did not consume entire buffer, %d remaining", len(rest))
		}
		// Currently, we can't check the txid because we get that from
		// zcashd (getblock rpc) rather than computing it ourselves.
		// https://github.com/zcash/lightwalletd/issues/392
		if tx.version != uint32(txtestdata.Version) {
			t.Fatal("version miscompare")
		}
		if tx.nVersionGroupID != uint32(txtestdata.NVersionGroupId) {
			t.Fatal("nVersionGroupId miscompare")
		}
		if tx.consensusBranchID != uint32(txtestdata.NConsensusBranchId) {
			t.Fatal("consensusBranchID miscompare")
		}
		if len(tx.transparentInputs) != int(txtestdata.Tx_in_count) {
			t.Fatal("tx_in_count miscompare")
		}
		if len(tx.transparentOutputs) != int(txtestdata.Tx_out_count) {
			t.Fatal("tx_out_count miscompare")
		}
		if len(tx.shieldedSpends) != int(txtestdata.NSpendsSapling) {
			t.Fatal("NSpendsSapling miscompare")
		}
		if len(tx.shieldedOutputs) != int(txtestdata.NoutputsSapling) {
			t.Fatal("NOutputsSapling miscompare")
		}
		if len(tx.orchardActions) != int(txtestdata.NActionsOrchard) {
			t.Fatal("NActionsOrchard miscompare")
		}
		// Verify compact ciphertext size for v5 (should be 52 bytes)
		for _, action := range tx.orchardActions {
			c := action.ToCompact()
			if len(c.Ciphertext) != 52 {
				t.Fatalf("v5 compact ciphertext should be 52 bytes, got %d", len(c.Ciphertext))
			}
		}
	}
}

func TestV6TransactionParser(t *testing.T) {
	s, err := os.ReadFile("../testdata/tx_v6.json")
	if err != nil {
		t.Fatal(err)
	}

	var testdata []json.RawMessage
	err = json.Unmarshal(s, &testdata)
	if err != nil {
		t.Fatal(err)
	}
	if len(testdata) < 3 {
		t.Skip("no v6 test vectors available yet")
	}
	testdata = testdata[2:]
	if len(testdata) == 0 {
		t.Skip("no v6 test vectors available yet")
	}
	for _, onetx := range testdata {
		var txtestdata TxTestData

		err = json.Unmarshal(onetx, &txtestdata)
		if err != nil {
			// Skip comment rows (they're JSON strings, not arrays)
			continue
		}
		t.Logf("txid %s", txtestdata.Txid)
		rawTxData, _ := hex.DecodeString(txtestdata.Tx)

		tx := NewTransaction()
		rest, err := tx.ParseFromSlice(rawTxData)
		if err != nil {
			t.Fatalf("%v", err)
		}
		if len(rest) != 0 {
			t.Fatalf("Test did not consume entire buffer, %d remaining", len(rest))
		}
		if tx.version != uint32(txtestdata.Version) {
			t.Fatal("version miscompare")
		}
		if tx.nVersionGroupID != uint32(txtestdata.NVersionGroupId) {
			t.Fatal("nVersionGroupId miscompare")
		}
		if tx.consensusBranchID != uint32(txtestdata.NConsensusBranchId) {
			t.Fatal("consensusBranchID miscompare")
		}
		if len(tx.transparentInputs) != int(txtestdata.Tx_in_count) {
			t.Fatal("tx_in_count miscompare")
		}
		if len(tx.transparentOutputs) != int(txtestdata.Tx_out_count) {
			t.Fatal("tx_out_count miscompare")
		}
		if len(tx.shieldedSpends) != int(txtestdata.NSpendsSapling) {
			t.Fatal("NSpendsSapling miscompare")
		}
		if len(tx.shieldedOutputs) != int(txtestdata.NoutputsSapling) {
			t.Fatal("NOutputsSapling miscompare")
		}
		if len(tx.orchardActions) != int(txtestdata.NActionsOrchard) {
			t.Fatal("NActionsOrchard miscompare")
		}
		// Verify compact ciphertext size for v6/ZSA (should be 84 bytes)
		for _, action := range tx.orchardActions {
			c := action.ToCompact()
			if len(c.Ciphertext) != 84 {
				t.Fatalf("v6 compact ciphertext should be 84 bytes, got %d", len(c.Ciphertext))
			}
		}
	}
}
