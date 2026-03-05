package signing

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss-lib/v2/common"
)

// TestContextJEncodingMatchesRound3 replicates the exact ContextJ construction
// from round_3.go line 41 and verifies it uses length-prefixed encoding via
// AppendBigIntToBytesSlice, not bare append.
func TestContextJEncodingMatchesRound3(t *testing.T) {
	ssid := []byte("test-ssid-for-ecdsa-signing-round3")

	for _, partyIndex := range []int{0, 1, 2, 255} {
		j := partyIndex
		// This is the exact pattern from round_3.go:41
		contextJ := common.AppendBigIntToBytesSlice(ssid, new(big.Int).SetUint64(uint64(j)))

		// Bare append (the OLD broken pattern) for comparison
		bareAppend := append([]byte{}, ssid...)
		bareAppend = append(bareAppend, new(big.Int).SetUint64(uint64(j)).Bytes()...)

		if j == 0 {
			// Critical: for party 0, big.Int(0).Bytes() = [] (empty),
			// so bare append produces just ssid. Length-prefixed adds [00 00 00 00].
			if hex.EncodeToString(contextJ) == hex.EncodeToString(bareAppend) {
				t.Fatal("ContextJ for party 0 must differ from bare append (SSID alone)")
			}
			if len(contextJ) != len(ssid)+4 {
				t.Fatalf("ContextJ for party 0: expected len %d, got %d", len(ssid)+4, len(contextJ))
			}
		}

		// Verify length-prefix structure: [ssid][4-byte len][bigint bytes]
		if len(contextJ) < len(ssid)+4 {
			t.Fatalf("ContextJ for party %d too short: %d", j, len(contextJ))
		}
	}
}

// TestContextJGoldenVectorsECDSASigning freezes the exact byte output of ContextJ
// for known inputs, so any regression in AppendBigIntToBytesSlice is caught.
func TestContextJGoldenVectorsECDSASigning(t *testing.T) {
	ssid := []byte("test-ssid")

	tests := []struct {
		index    uint64
		expected string
	}{
		// party 0: ssid + [00 00 00 00] (length=0, no value bytes)
		{0, "746573742d7373696400000000"},
		// party 1: ssid + [00 00 00 01] (length=1) + [01]
		{1, "746573742d737369640000000101"},
		// party 2: ssid + [00 00 00 01] (length=1) + [02]
		{2, "746573742d737369640000000102"},
		// party 256: ssid + [00 00 00 02] (length=2) + [01 00]
		{256, "746573742d73736964000000020100"},
	}

	for _, tc := range tests {
		// Exact pattern from round_3.go:41
		contextJ := common.AppendBigIntToBytesSlice(ssid, new(big.Int).SetUint64(tc.index))
		got := hex.EncodeToString(contextJ)
		if got != tc.expected {
			t.Errorf("ContextJ(ssid, %d) = %s, want %s", tc.index, got, tc.expected)
		}
	}
}
