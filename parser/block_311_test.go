package parser

import (
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

func TestParseBlock311(t *testing.T) {
	hexBytes, err := os.ReadFile("/Users/hanh/projects/zkool2/rust/tests/block_311.hex")
	if err != nil {
		t.Fatalf("Cannot read block_311.hex: %v", err)
	}
	blockHex := strings.TrimSpace(string(hexBytes))
	blockData, err := hex.DecodeString(blockHex)
	if err != nil {
		t.Fatalf("hex decode: %v", err)
	}
	t.Logf("Block size: %d bytes", len(blockData))
	
	block := NewBlock()
	rest, err := block.ParseFromSlice(blockData)
	if err != nil {
		t.Fatalf("ParseFromSlice error: %v", err)
	}
	t.Logf("Rest: %d bytes", len(rest))
	t.Logf("TX count: %d", block.GetTxCount())
	for i, tx := range block.Transactions() {
		t.Logf("TX %d: txid=%s", i, tx.txID)
	}
}
