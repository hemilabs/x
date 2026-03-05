package keygen

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss-lib/v2/common"
)

// TestContextIEncodingMatchesRound2 replicates the exact ContextI construction
// from round_2.go line 50 and verifies it uses length-prefixed encoding via
// AppendBigIntToBytesSlice, not bare append.
func TestContextIEncodingMatchesRound2(t *testing.T) {
	ssid := []byte("test-ssid-for-eddsa-keygen-round2")

	for _, partyIndex := range []int{0, 1, 2, 255} {
		i := partyIndex
		// This is the exact pattern from round_2.go:50
		contextI := common.AppendBigIntToBytesSlice(ssid, new(big.Int).SetUint64(uint64(i)))

		// Bare append (the OLD broken pattern) for comparison
		bareAppend := append([]byte{}, ssid...)
		bareAppend = append(bareAppend, new(big.Int).SetUint64(uint64(i)).Bytes()...)

		if i == 0 {
			// Critical: for party 0, big.Int(0).Bytes() = [] (empty),
			// so bare append produces just ssid. Length-prefixed adds [00 00 00 00].
			if hex.EncodeToString(contextI) == hex.EncodeToString(bareAppend) {
				t.Fatal("ContextI for party 0 must differ from bare append (SSID alone)")
			}
			if len(contextI) != len(ssid)+4 {
				t.Fatalf("ContextI for party 0: expected len %d, got %d", len(ssid)+4, len(contextI))
			}
		}

		// Verify length-prefix structure: [ssid][4-byte len][bigint bytes]
		if len(contextI) < len(ssid)+4 {
			t.Fatalf("ContextI for party %d too short: %d", i, len(contextI))
		}
	}
}

// TestContextIGoldenVectorsEdDSAKeygen freezes the exact byte output of ContextI
// for known inputs, so any regression in AppendBigIntToBytesSlice is caught.
func TestContextIGoldenVectorsEdDSAKeygen(t *testing.T) {
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
		// Exact pattern from round_2.go:50
		contextI := common.AppendBigIntToBytesSlice(ssid, new(big.Int).SetUint64(tc.index))
		got := hex.EncodeToString(contextI)
		if got != tc.expected {
			t.Errorf("ContextI(ssid, %d) = %s, want %s", tc.index, got, tc.expected)
		}
	}
}

// TestContextIDistinguishesParties verifies that all party indices in a
// typical keygen produce distinct ContextI values.
func TestContextIDistinguishesParties(t *testing.T) {
	ssid := []byte("keygen-ssid-32-bytes-exactly!!!!")

	seen := make(map[string]int)
	for i := 0; i < 20; i++ {
		contextI := common.AppendBigIntToBytesSlice(ssid, new(big.Int).SetUint64(uint64(i)))
		h := hex.EncodeToString(contextI)
		if prev, ok := seen[h]; ok {
			t.Fatalf("ContextI collision: party %d and party %d produce identical bytes", prev, i)
		}
		seen[h] = i
	}
}
