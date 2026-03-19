// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package resharing

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss-lib/v3/crypto"
	"github.com/hemilabs/x/tss-lib/v3/ecdsa/keygen"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// buildTestResharingFixture creates deterministic ReSharingParameters and
// matching keygen save data for SSID and index tests.  The returned
// objects are minimal but structurally valid for getReshareSSID.
func buildTestResharingFixture(t *testing.T) (
	params *tss.ReSharingParameters,
	input *keygen.LocalPartySaveData,
	temp *localTempData,
	oldPIDs tss.SortedPartyIDs,
	newPIDs tss.SortedPartyIDs,
) {
	t.Helper()
	ec := tss.S256()

	// Build deterministic party IDs (3 old, 3 new).
	oldPIDs = makeDeterministicPartyIDs(3, 100)
	newPIDs = makeDeterministicPartyIDs(3, 200)
	oldCtx := tss.NewPeerContext(oldPIDs)
	newCtx := tss.NewPeerContext(newPIDs)

	// Parameters from the perspective of old party 0.
	params = tss.NewReSharingParameters(ec, oldCtx, newCtx, oldPIDs[0], 3, 1, 3, 1)

	// Build minimal LocalPartySaveData with BigXj, NTildej, H1j, H2j.
	n := 3
	save := keygen.NewLocalPartySaveData(n)
	for i := 0; i < n; i++ {
		// Use scalar base mult to produce valid on-curve points.
		scalar := big.NewInt(int64(i + 7))
		pt := crypto.ScalarBaseMult(ec, scalar)
		save.BigXj[i] = pt
		save.NTildej[i] = big.NewInt(int64(1000 + i))
		save.H1j[i] = big.NewInt(int64(2000 + i))
		save.H2j[i] = big.NewInt(int64(3000 + i))
	}

	input = &save
	temp = &localTempData{
		ssidNonce: big.NewInt(42),
	}
	return
}

// makeDeterministicPartyIDs creates sorted party IDs with keys derived from
// a base offset so that tests are reproducible.
func makeDeterministicPartyIDs(count int, base int64) tss.SortedPartyIDs {
	ids := make(tss.UnSortedPartyIDs, count)
	for i := 0; i < count; i++ {
		key := big.NewInt(base + int64(i) + 1) // +1 to avoid zero key
		ids[i] = tss.NewPartyID(
			big.NewInt(base+int64(i)).String(),
			"P",
			key,
		)
	}
	return tss.SortPartyIDs(ids)
}

// ---------- getReshareSSID tests ----------

func TestGetReshareSSIDDeterministic(t *testing.T) {
	params, input, temp, _, _ := buildTestResharingFixture(t)

	ssid1, err := getReshareSSID(params, input, temp, 1)
	if err != nil {
		t.Fatalf("getReshareSSID call 1: %v", err)
	}
	ssid2, err := getReshareSSID(params, input, temp, 1)
	if err != nil {
		t.Fatalf("getReshareSSID call 2: %v", err)
	}

	if !bytes.Equal(ssid1, ssid2) {
		t.Fatalf("same inputs must produce identical SSIDs:\n  ssid1=%x\n  ssid2=%x", ssid1, ssid2)
	}
	if len(ssid1) == 0 {
		t.Fatal("SSID must be non-empty")
	}
}

func TestGetReshareSSIDWithCeremonyID(t *testing.T) {
	params, input, temp, _, _ := buildTestResharingFixture(t)

	// Baseline: no ceremony ID.
	ssidNoCID, err := getReshareSSID(params, input, temp, 1)
	if err != nil {
		t.Fatalf("getReshareSSID (no CID): %v", err)
	}

	// Set a ceremony ID and recompute.
	params.SetCeremonyID([]byte("ceremony-alpha"))
	ssidWithCID, err := getReshareSSID(params, input, temp, 1)
	if err != nil {
		t.Fatalf("getReshareSSID (with CID): %v", err)
	}

	if bytes.Equal(ssidNoCID, ssidWithCID) {
		t.Fatal("CeremonyID must change the SSID, but got identical values")
	}

	// Different ceremony ID produces a different SSID again.
	params.SetCeremonyID([]byte("ceremony-beta"))
	ssidWithCID2, err := getReshareSSID(params, input, temp, 1)
	if err != nil {
		t.Fatalf("getReshareSSID (with CID2): %v", err)
	}
	if bytes.Equal(ssidWithCID, ssidWithCID2) {
		t.Fatal("different CeremonyIDs must produce different SSIDs")
	}

	// Determinism: same ceremony ID twice.
	ssidWithCID2Again, err := getReshareSSID(params, input, temp, 1)
	if err != nil {
		t.Fatalf("getReshareSSID (with CID2 again): %v", err)
	}
	if !bytes.Equal(ssidWithCID2, ssidWithCID2Again) {
		t.Fatal("same CeremonyID must produce identical SSIDs on repeated calls")
	}
}

func TestGetReshareSSIDDifferentRoundNumbers(t *testing.T) {
	params, input, temp, _, _ := buildTestResharingFixture(t)

	ssids := make(map[string]int)
	for round := 1; round <= 5; round++ {
		ssid, err := getReshareSSID(params, input, temp, round)
		if err != nil {
			t.Fatalf("getReshareSSID(round=%d): %v", round, err)
		}
		key := string(ssid)
		if prev, exists := ssids[key]; exists {
			t.Fatalf("round %d produced the same SSID as round %d", round, prev)
		}
		ssids[key] = round
	}
}

func TestGetReshareSSIDDifferentNonce(t *testing.T) {
	params, input, temp, _, _ := buildTestResharingFixture(t)

	ssid1, err := getReshareSSID(params, input, temp, 1)
	if err != nil {
		t.Fatalf("getReshareSSID(nonce=42): %v", err)
	}

	temp2 := &localTempData{ssidNonce: big.NewInt(99)}
	ssid2, err := getReshareSSID(params, input, temp2, 1)
	if err != nil {
		t.Fatalf("getReshareSSID(nonce=99): %v", err)
	}

	if bytes.Equal(ssid1, ssid2) {
		t.Fatal("different ssidNonce values must produce different SSIDs")
	}
}

func TestGetReshareSSIDDifferentThresholds(t *testing.T) {
	ec := tss.S256()
	oldPIDs := makeDeterministicPartyIDs(4, 100)
	newPIDs := makeDeterministicPartyIDs(4, 200)
	oldCtx := tss.NewPeerContext(oldPIDs)
	newCtx := tss.NewPeerContext(newPIDs)

	n := 4
	save := keygen.NewLocalPartySaveData(n)
	for i := 0; i < n; i++ {
		pt := crypto.ScalarBaseMult(ec, big.NewInt(int64(i+7)))
		save.BigXj[i] = pt
		save.NTildej[i] = big.NewInt(int64(1000 + i))
		save.H1j[i] = big.NewInt(int64(2000 + i))
		save.H2j[i] = big.NewInt(int64(3000 + i))
	}
	temp := &localTempData{ssidNonce: big.NewInt(1)}

	params1 := tss.NewReSharingParameters(ec, oldCtx, newCtx, oldPIDs[0], 4, 1, 4, 1)
	ssid1, err := getReshareSSID(params1, &save, temp, 1)
	if err != nil {
		t.Fatalf("getReshareSSID(threshold=1): %v", err)
	}

	params2 := tss.NewReSharingParameters(ec, oldCtx, newCtx, oldPIDs[0], 4, 2, 4, 2)
	ssid2, err := getReshareSSID(params2, &save, temp, 1)
	if err != nil {
		t.Fatalf("getReshareSSID(threshold=2): %v", err)
	}

	if bytes.Equal(ssid1, ssid2) {
		t.Fatal("different thresholds must produce different SSIDs")
	}
}

// ---------- oldIndex / newIndex tests ----------

func TestOldIndexFindsCorrectIndex(t *testing.T) {
	ec := tss.S256()
	oldPIDs := makeDeterministicPartyIDs(3, 100)
	newPIDs := makeDeterministicPartyIDs(3, 200)
	oldCtx := tss.NewPeerContext(oldPIDs)
	newCtx := tss.NewPeerContext(newPIDs)

	for i, pid := range oldPIDs {
		params := tss.NewReSharingParameters(ec, oldCtx, newCtx, pid, 3, 1, 3, 1)
		got := oldIndex(params)
		if got != i {
			t.Errorf("oldIndex for party %d: expected %d, got %d", i, i, got)
		}
	}
}

func TestOldIndexReturnsNegOneForNonMember(t *testing.T) {
	ec := tss.S256()
	oldPIDs := makeDeterministicPartyIDs(3, 100)
	newPIDs := makeDeterministicPartyIDs(3, 200)
	oldCtx := tss.NewPeerContext(oldPIDs)
	newCtx := tss.NewPeerContext(newPIDs)

	// A party in the new committee should not be found in the old committee.
	for _, pid := range newPIDs {
		params := tss.NewReSharingParameters(ec, oldCtx, newCtx, pid, 3, 1, 3, 1)
		got := oldIndex(params)
		if got != -1 {
			t.Errorf("oldIndex for new-committee party %v: expected -1, got %d", pid, got)
		}
	}

	// A completely unrelated party.
	stranger := tss.NewPartyID("stranger", "stranger", big.NewInt(999999))
	paramsStranger := tss.NewReSharingParameters(ec, oldCtx, newCtx, stranger, 3, 1, 3, 1)
	if got := oldIndex(paramsStranger); got != -1 {
		t.Errorf("oldIndex for stranger party: expected -1, got %d", got)
	}
}

func TestNewIndexFindsCorrectIndex(t *testing.T) {
	ec := tss.S256()
	oldPIDs := makeDeterministicPartyIDs(3, 100)
	newPIDs := makeDeterministicPartyIDs(3, 200)
	oldCtx := tss.NewPeerContext(oldPIDs)
	newCtx := tss.NewPeerContext(newPIDs)

	for i, pid := range newPIDs {
		params := tss.NewReSharingParameters(ec, oldCtx, newCtx, pid, 3, 1, 3, 1)
		got := newIndex(params)
		if got != i {
			t.Errorf("newIndex for party %d: expected %d, got %d", i, i, got)
		}
	}
}

func TestNewIndexReturnsNegOneForNonMember(t *testing.T) {
	ec := tss.S256()
	oldPIDs := makeDeterministicPartyIDs(3, 100)
	newPIDs := makeDeterministicPartyIDs(3, 200)
	oldCtx := tss.NewPeerContext(oldPIDs)
	newCtx := tss.NewPeerContext(newPIDs)

	// Old-committee parties should not be found in new committee.
	for _, pid := range oldPIDs {
		params := tss.NewReSharingParameters(ec, oldCtx, newCtx, pid, 3, 1, 3, 1)
		got := newIndex(params)
		if got != -1 {
			t.Errorf("newIndex for old-committee party %v: expected -1, got %d", pid, got)
		}
	}
}

// TestOldAndNewIndexOverlappingParty verifies that when a party is a member
// of both the old and new committees, both oldIndex and newIndex return
// valid (non-negative) indices.
func TestOldAndNewIndexOverlappingParty(t *testing.T) {
	ec := tss.S256()

	// Create party IDs where party 0 is in both committees.
	sharedPID := tss.NewPartyID("shared", "shared", big.NewInt(50))
	oldOnly1 := tss.NewPartyID("old1", "old1", big.NewInt(51))
	oldOnly2 := tss.NewPartyID("old2", "old2", big.NewInt(52))
	newOnly1 := tss.NewPartyID("new1", "new1", big.NewInt(53))
	newOnly2 := tss.NewPartyID("new2", "new2", big.NewInt(54))

	oldPIDs := tss.SortPartyIDs(tss.UnSortedPartyIDs{sharedPID, oldOnly1, oldOnly2})
	newPIDs := tss.SortPartyIDs(tss.UnSortedPartyIDs{sharedPID, newOnly1, newOnly2})
	oldCtx := tss.NewPeerContext(oldPIDs)
	newCtx := tss.NewPeerContext(newPIDs)

	params := tss.NewReSharingParameters(ec, oldCtx, newCtx, sharedPID, 3, 1, 3, 1)

	oi := oldIndex(params)
	ni := newIndex(params)
	if oi < 0 {
		t.Fatalf("overlapping party should be in old committee, got oldIndex=%d", oi)
	}
	if ni < 0 {
		t.Fatalf("overlapping party should be in new committee, got newIndex=%d", ni)
	}
	// Verify the indices actually point to our shared party.
	if oldPIDs[oi].KeyInt().Cmp(sharedPID.KeyInt()) != 0 {
		t.Errorf("oldIndex %d does not point to shared party", oi)
	}
	if newPIDs[ni].KeyInt().Cmp(sharedPID.KeyInt()) != 0 {
		t.Errorf("newIndex %d does not point to shared party", ni)
	}
}

// ---------- SSID length / format sanity ----------

func TestGetReshareSSIDLength(t *testing.T) {
	params, input, temp, _, _ := buildTestResharingFixture(t)
	ssid, err := getReshareSSID(params, input, temp, 1)
	if err != nil {
		t.Fatalf("getReshareSSID: %v", err)
	}
	// SHA-512/256 produces 32 bytes. The SSID is SHA512_256i(...).Bytes()
	// which strips leading zeros, so it is at most 32 bytes.
	if len(ssid) == 0 || len(ssid) > 32 {
		t.Fatalf("SSID length should be in (0, 32], got %d", len(ssid))
	}
}
