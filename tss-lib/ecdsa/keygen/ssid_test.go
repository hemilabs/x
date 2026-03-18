// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// makeSSIDTestParams builds a *tss.Parameters and *localTempData suitable for
// calling getSSID.  ssidNonce is set from the params' SSIDNonce (default 0).
func makeSSIDTestParams(t *testing.T, n, threshold int) (*tss.Parameters, *localTempData) {
	t.Helper()
	pIDs := tss.GenerateTestPartyIDs(n)
	peerCtx := tss.NewPeerContext(pIDs)
	params := tss.NewParameters(tss.S256(), peerCtx, pIDs[0], n, threshold)
	temp := &localTempData{
		ssidNonce: new(big.Int).SetUint64(uint64(params.SSIDNonce())),
	}
	return params, temp
}

// TestGetSSIDDeterministic verifies that calling getSSID twice with
// identical inputs produces the same output.
func TestGetSSIDDeterministic(t *testing.T) {
	params, temp := makeSSIDTestParams(t, 3, 1)

	ssid1, err := getSSID(params, temp, 1)
	if err != nil {
		t.Fatalf("getSSID call 1: %v", err)
	}
	ssid2, err := getSSID(params, temp, 1)
	if err != nil {
		t.Fatalf("getSSID call 2: %v", err)
	}
	if !bytes.Equal(ssid1, ssid2) {
		t.Fatalf("getSSID not deterministic: %x != %x", ssid1, ssid2)
	}
	if len(ssid1) == 0 {
		t.Fatal("getSSID returned empty slice")
	}
}

// TestGetSSIDWithCeremonyID verifies that setting a CeremonyID changes
// the SSID output relative to no CeremonyID.
func TestGetSSIDWithCeremonyID(t *testing.T) {
	params, temp := makeSSIDTestParams(t, 3, 1)

	ssidWithout, err := getSSID(params, temp, 1)
	if err != nil {
		t.Fatalf("getSSID without CeremonyID: %v", err)
	}

	params.SetCeremonyID([]byte("test-ceremony-42"))
	ssidWith, err := getSSID(params, temp, 1)
	if err != nil {
		t.Fatalf("getSSID with CeremonyID: %v", err)
	}

	if bytes.Equal(ssidWithout, ssidWith) {
		t.Fatal("CeremonyID did not change the SSID")
	}
}

// TestGetSSIDDifferentCeremonyIDs verifies that two distinct CeremonyIDs
// produce two distinct SSIDs.
func TestGetSSIDDifferentCeremonyIDs(t *testing.T) {
	params, temp := makeSSIDTestParams(t, 3, 1)

	params.SetCeremonyID([]byte("ceremony-A"))
	ssidA, err := getSSID(params, temp, 1)
	if err != nil {
		t.Fatalf("getSSID ceremony-A: %v", err)
	}

	params.SetCeremonyID([]byte("ceremony-B"))
	ssidB, err := getSSID(params, temp, 1)
	if err != nil {
		t.Fatalf("getSSID ceremony-B: %v", err)
	}

	if bytes.Equal(ssidA, ssidB) {
		t.Fatal("different CeremonyIDs produced the same SSID")
	}
}

// TestGetSSIDDifferentRoundNumbers verifies that same params but
// different round numbers produce different SSIDs.
func TestGetSSIDDifferentRoundNumbers(t *testing.T) {
	params, temp := makeSSIDTestParams(t, 3, 1)

	ssid1, err := getSSID(params, temp, 1)
	if err != nil {
		t.Fatalf("getSSID round 1: %v", err)
	}
	ssid2, err := getSSID(params, temp, 2)
	if err != nil {
		t.Fatalf("getSSID round 2: %v", err)
	}
	ssid3, err := getSSID(params, temp, 3)
	if err != nil {
		t.Fatalf("getSSID round 3: %v", err)
	}

	if bytes.Equal(ssid1, ssid2) {
		t.Fatal("round 1 and 2 produced the same SSID")
	}
	if bytes.Equal(ssid2, ssid3) {
		t.Fatal("round 2 and 3 produced the same SSID")
	}
	if bytes.Equal(ssid1, ssid3) {
		t.Fatal("round 1 and 3 produced the same SSID")
	}
}

// TestGetSSIDIncludesAllPartyKeys verifies that changing ANY party's
// key changes the SSID — not just the last party.
func TestGetSSIDIncludesAllPartyKeys(t *testing.T) {
	baseKeys := []*big.Int{big.NewInt(100), big.NewInt(200), big.NewInt(300)}

	makeParams := func(k1, k2, k3 *big.Int) (*tss.Parameters, *localTempData) {
		ids := tss.UnSortedPartyIDs{
			tss.NewPartyID("1", "P[1]", k1),
			tss.NewPartyID("2", "P[2]", k2),
			tss.NewPartyID("3", "P[3]", k3),
		}
		sorted := tss.SortPartyIDs(ids)
		peerCtx := tss.NewPeerContext(sorted)
		params := tss.NewParameters(tss.S256(), peerCtx, sorted[0], 3, 1)
		temp := &localTempData{
			ssidNonce: new(big.Int).SetUint64(0),
		}
		return params, temp
	}

	paramsOriginal, tempOriginal := makeParams(baseKeys[0], baseKeys[1], baseKeys[2])
	ssidOriginal, err := getSSID(paramsOriginal, tempOriginal, 1)
	if err != nil {
		t.Fatalf("getSSID original: %v", err)
	}

	// Change each party's key individually and verify the SSID changes.
	for i := 0; i < 3; i++ {
		t.Run("change_key_"+big.NewInt(int64(i)).String(), func(t *testing.T) {
			keys := []*big.Int{
				new(big.Int).Set(baseKeys[0]),
				new(big.Int).Set(baseKeys[1]),
				new(big.Int).Set(baseKeys[2]),
			}
			keys[i] = big.NewInt(999) // mutate party i's key
			p, tmp := makeParams(keys[0], keys[1], keys[2])
			ssid, err := getSSID(p, tmp, 1)
			if err != nil {
				t.Fatalf("getSSID with changed key %d: %v", i, err)
			}
			if bytes.Equal(ssidOriginal, ssid) {
				t.Fatalf("changing party %d's key did not change the SSID", i)
			}
		})
	}
}

// TestGetSSIDIncludesNonce verifies that a different ssidNonce
// produces a different SSID.
func TestGetSSIDIncludesNonce(t *testing.T) {
	params, temp0 := makeSSIDTestParams(t, 3, 1)

	ssid0, err := getSSID(params, temp0, 1)
	if err != nil {
		t.Fatalf("getSSID nonce=0: %v", err)
	}

	temp1 := &localTempData{
		ssidNonce: big.NewInt(42),
	}
	ssid1, err := getSSID(params, temp1, 1)
	if err != nil {
		t.Fatalf("getSSID nonce=42: %v", err)
	}

	if bytes.Equal(ssid0, ssid1) {
		t.Fatal("different ssidNonce values produced the same SSID")
	}
}

// TestGetSSIDOutputLength verifies that getSSID produces a
// SHA-512/256 output (32 bytes).
func TestGetSSIDOutputLength(t *testing.T) {
	params, temp := makeSSIDTestParams(t, 3, 1)

	ssid, err := getSSID(params, temp, 1)
	if err != nil {
		t.Fatalf("getSSID: %v", err)
	}
	// SHA-512/256 produces 32 bytes, but big.Int.Bytes() strips
	// leading zeros.  Verify the output is at most 32 bytes and
	// non-empty.
	if len(ssid) == 0 {
		t.Fatal("getSSID returned empty output")
	}
	if len(ssid) > 32 {
		t.Fatalf("getSSID output too long: %d bytes (expected <= 32)", len(ssid))
	}
}
