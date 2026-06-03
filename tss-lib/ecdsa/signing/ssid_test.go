// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package signing

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss-lib/v3/crypto"
	"github.com/hemilabs/x/tss-lib/v3/ecdsa/keygen"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// buildSigningSSIDFixture creates minimal but structurally valid inputs
// for getSigningSSID.  Party keys are deterministic so tests are
// reproducible.
func buildSigningSSIDFixture(t *testing.T, n, threshold int) (params *tss.Parameters, key *keygen.LocalPartySaveData, temp *localTempData) {
	t.Helper()
	ec := tss.S256()

	ids := make(tss.UnSortedPartyIDs, n)
	for i := 0; i < n; i++ {
		ids[i] = tss.NewPartyID(
			big.NewInt(int64(i)).String(),
			"P",
			big.NewInt(int64(100+i+1)),
		)
	}
	sorted := tss.SortPartyIDs(ids)
	peerCtx := tss.NewPeerContext(sorted)
	params = tss.NewParameters(ec, peerCtx, sorted[0], n, threshold)

	save := keygen.NewLocalPartySaveData(n)
	for i := 0; i < n; i++ {
		save.BigXj[i] = crypto.ScalarBaseMult(ec, big.NewInt(int64(i+7)))
		save.NTildej[i] = big.NewInt(int64(1000 + i))
		save.H1j[i] = big.NewInt(int64(2000 + i))
		save.H2j[i] = big.NewInt(int64(3000 + i))
	}

	key = &save
	temp = &localTempData{
		ssidNonce: big.NewInt(42),
		m:         new(big.Int).SetBytes([]byte("test-message-hash")),
	}
	return
}

// ---------- Determinism ----------

func TestGetSigningSSIDDeterministic(t *testing.T) {
	params, key, temp := buildSigningSSIDFixture(t, 3, 1)

	ssid1, err := getSigningSSID(params, key, temp, 1)
	if err != nil {
		t.Fatalf("call 1: %v", err)
	}
	ssid2, err := getSigningSSID(params, key, temp, 1)
	if err != nil {
		t.Fatalf("call 2: %v", err)
	}
	if !bytes.Equal(ssid1, ssid2) {
		t.Fatalf("not deterministic: %x != %x", ssid1, ssid2)
	}
	if len(ssid1) == 0 {
		t.Fatal("returned empty SSID")
	}
}

// ---------- CeremonyID ----------

func TestGetSigningSSIDWithCeremonyID(t *testing.T) {
	params, key, temp := buildSigningSSIDFixture(t, 3, 1)

	ssidNone, err := getSigningSSID(params, key, temp, 1)
	if err != nil {
		t.Fatalf("no CID: %v", err)
	}

	params.SetCeremonyID([]byte("ceremony-A"))
	ssidA, err := getSigningSSID(params, key, temp, 1)
	if err != nil {
		t.Fatalf("CID-A: %v", err)
	}
	if bytes.Equal(ssidNone, ssidA) {
		t.Fatal("CeremonyID did not change SSID")
	}

	params.SetCeremonyID([]byte("ceremony-B"))
	ssidB, err := getSigningSSID(params, key, temp, 1)
	if err != nil {
		t.Fatalf("CID-B: %v", err)
	}
	if bytes.Equal(ssidA, ssidB) {
		t.Fatal("different CeremonyIDs produced same SSID")
	}
}

// ---------- Round number ----------

func TestGetSigningSSIDDifferentRoundNumbers(t *testing.T) {
	params, key, temp := buildSigningSSIDFixture(t, 3, 1)

	seen := make(map[string]int)
	for round := 1; round <= 5; round++ {
		ssid, err := getSigningSSID(params, key, temp, round)
		if err != nil {
			t.Fatalf("round %d: %v", round, err)
		}
		k := string(ssid)
		if prev, ok := seen[k]; ok {
			t.Fatalf("round %d collides with round %d", round, prev)
		}
		seen[k] = round
	}
}

// ---------- Nonce ----------

func TestGetSigningSSIDDifferentNonce(t *testing.T) {
	params, key, temp := buildSigningSSIDFixture(t, 3, 1)

	ssid1, err := getSigningSSID(params, key, temp, 1)
	if err != nil {
		t.Fatalf("nonce=42: %v", err)
	}

	temp2 := &localTempData{ssidNonce: big.NewInt(99), m: temp.m}
	ssid2, err := getSigningSSID(params, key, temp2, 1)
	if err != nil {
		t.Fatalf("nonce=99: %v", err)
	}
	if bytes.Equal(ssid1, ssid2) {
		t.Fatal("different nonces produced same SSID")
	}
}

// ---------- Message hash sensitivity (signing-specific) ----------

func TestGetSigningSSIDDifferentMessage(t *testing.T) {
	params, key, temp := buildSigningSSIDFixture(t, 3, 1)

	ssid1, err := getSigningSSID(params, key, temp, 1)
	if err != nil {
		t.Fatalf("msg-A: %v", err)
	}

	temp2 := &localTempData{
		ssidNonce: temp.ssidNonce,
		m:         new(big.Int).SetBytes([]byte("different-message-hash")),
	}
	ssid2, err := getSigningSSID(params, key, temp2, 1)
	if err != nil {
		t.Fatalf("msg-B: %v", err)
	}
	if bytes.Equal(ssid1, ssid2) {
		t.Fatal("different message hashes must produce different SSIDs")
	}
}

func TestGetSigningSSIDNilMessageVsNonNil(t *testing.T) {
	params, key, temp := buildSigningSSIDFixture(t, 3, 1)

	ssidWithMsg, err := getSigningSSID(params, key, temp, 1)
	if err != nil {
		t.Fatalf("with m: %v", err)
	}

	tempNil := &localTempData{ssidNonce: temp.ssidNonce, m: nil}
	ssidNoMsg, err := getSigningSSID(params, key, tempNil, 1)
	if err != nil {
		t.Fatalf("nil m: %v", err)
	}
	if bytes.Equal(ssidWithMsg, ssidNoMsg) {
		t.Fatal("nil m and non-nil m must produce different SSIDs")
	}
}

// ---------- BigXj sensitivity ----------

func TestGetSigningSSIDDifferentBigXj(t *testing.T) {
	params, key, temp := buildSigningSSIDFixture(t, 3, 1)

	ssidOrig, err := getSigningSSID(params, key, temp, 1)
	if err != nil {
		t.Fatalf("original: %v", err)
	}

	for i := 0; i < 3; i++ {
		t.Run(big.NewInt(int64(i)).String(), func(t *testing.T) {
			clone := cloneLocalPartySaveData(key, 3)
			clone.BigXj[i] = crypto.ScalarBaseMult(tss.S256(), big.NewInt(999))
			ssid, err := getSigningSSID(params, clone, temp, 1)
			if err != nil {
				t.Fatalf("mutated BigXj[%d]: %v", i, err)
			}
			if bytes.Equal(ssidOrig, ssid) {
				t.Fatalf("changing BigXj[%d] did not change SSID", i)
			}
		})
	}
}

// ---------- NTildej / H1j / H2j sensitivity ----------

func TestGetSigningSSIDDifferentNTildej(t *testing.T) {
	params, key, temp := buildSigningSSIDFixture(t, 3, 1)
	ssidOrig, _ := getSigningSSID(params, key, temp, 1)

	clone := cloneLocalPartySaveData(key, 3)
	clone.NTildej[1] = big.NewInt(99999)
	ssid, err := getSigningSSID(params, clone, temp, 1)
	if err != nil {
		t.Fatalf("mutated NTildej: %v", err)
	}
	if bytes.Equal(ssidOrig, ssid) {
		t.Fatal("changing NTildej did not change SSID")
	}
}

func TestGetSigningSSIDDifferentH1j(t *testing.T) {
	params, key, temp := buildSigningSSIDFixture(t, 3, 1)
	ssidOrig, _ := getSigningSSID(params, key, temp, 1)

	clone := cloneLocalPartySaveData(key, 3)
	clone.H1j[1] = big.NewInt(99999)
	ssid, err := getSigningSSID(params, clone, temp, 1)
	if err != nil {
		t.Fatalf("mutated H1j: %v", err)
	}
	if bytes.Equal(ssidOrig, ssid) {
		t.Fatal("changing H1j did not change SSID")
	}
}

func TestGetSigningSSIDDifferentH2j(t *testing.T) {
	params, key, temp := buildSigningSSIDFixture(t, 3, 1)
	ssidOrig, _ := getSigningSSID(params, key, temp, 1)

	clone := cloneLocalPartySaveData(key, 3)
	clone.H2j[1] = big.NewInt(99999)
	ssid, err := getSigningSSID(params, clone, temp, 1)
	if err != nil {
		t.Fatalf("mutated H2j: %v", err)
	}
	if bytes.Equal(ssidOrig, ssid) {
		t.Fatal("changing H2j did not change SSID")
	}
}

// ---------- Party keys, threshold, party count ----------

func TestGetSigningSSIDDifferentPartyKeys(t *testing.T) {
	params, key, temp := buildSigningSSIDFixture(t, 3, 1)
	ssidOrig, err := getSigningSSID(params, key, temp, 1)
	if err != nil {
		t.Fatalf("original: %v", err)
	}

	// Rebuild params with a different key for party 1.
	ids := tss.UnSortedPartyIDs{
		tss.NewPartyID("0", "P", big.NewInt(101)),
		tss.NewPartyID("1", "P", big.NewInt(999)), // changed from 102
		tss.NewPartyID("2", "P", big.NewInt(103)),
	}
	sorted := tss.SortPartyIDs(ids)
	peerCtx := tss.NewPeerContext(sorted)
	params2 := tss.NewParameters(tss.S256(), peerCtx, sorted[0], 3, 1)

	ssid2, err := getSigningSSID(params2, key, temp, 1)
	if err != nil {
		t.Fatalf("changed key: %v", err)
	}
	if bytes.Equal(ssidOrig, ssid2) {
		t.Fatal("different party keys must produce different SSIDs")
	}
}

func TestGetSigningSSIDDifferentThreshold(t *testing.T) {
	ec := tss.S256()
	ids := tss.UnSortedPartyIDs{
		tss.NewPartyID("0", "P", big.NewInt(101)),
		tss.NewPartyID("1", "P", big.NewInt(102)),
		tss.NewPartyID("2", "P", big.NewInt(103)),
		tss.NewPartyID("3", "P", big.NewInt(104)),
	}
	sorted := tss.SortPartyIDs(ids)
	peerCtx := tss.NewPeerContext(sorted)

	save := keygen.NewLocalPartySaveData(4)
	for i := 0; i < 4; i++ {
		save.BigXj[i] = crypto.ScalarBaseMult(ec, big.NewInt(int64(i+7)))
		save.NTildej[i] = big.NewInt(int64(1000 + i))
		save.H1j[i] = big.NewInt(int64(2000 + i))
		save.H2j[i] = big.NewInt(int64(3000 + i))
	}
	temp := &localTempData{ssidNonce: big.NewInt(0), m: big.NewInt(12345)}

	params1 := tss.NewParameters(ec, peerCtx, sorted[0], 4, 1)
	ssid1, err := getSigningSSID(params1, &save, temp, 1)
	if err != nil {
		t.Fatalf("threshold=1: %v", err)
	}

	params2 := tss.NewParameters(ec, peerCtx, sorted[0], 4, 2)
	ssid2, err := getSigningSSID(params2, &save, temp, 1)
	if err != nil {
		t.Fatalf("threshold=2: %v", err)
	}

	if bytes.Equal(ssid1, ssid2) {
		t.Fatal("different thresholds must produce different SSIDs")
	}
}

func TestGetSigningSSIDDifferentPartyCount(t *testing.T) {
	ec := tss.S256()
	temp := &localTempData{ssidNonce: big.NewInt(0), m: big.NewInt(12345)}

	makeSave := func(n int) *keygen.LocalPartySaveData {
		s := keygen.NewLocalPartySaveData(n)
		for i := 0; i < n; i++ {
			s.BigXj[i] = crypto.ScalarBaseMult(ec, big.NewInt(int64(i+7)))
			s.NTildej[i] = big.NewInt(int64(1000 + i))
			s.H1j[i] = big.NewInt(int64(2000 + i))
			s.H2j[i] = big.NewInt(int64(3000 + i))
		}
		return &s
	}

	ids3 := tss.SortPartyIDs(tss.UnSortedPartyIDs{
		tss.NewPartyID("0", "P", big.NewInt(101)),
		tss.NewPartyID("1", "P", big.NewInt(102)),
		tss.NewPartyID("2", "P", big.NewInt(103)),
	})
	params3 := tss.NewParameters(ec, tss.NewPeerContext(ids3), ids3[0], 3, 1)
	ssid3, err := getSigningSSID(params3, makeSave(3), temp, 1)
	if err != nil {
		t.Fatalf("n=3: %v", err)
	}

	ids4 := tss.SortPartyIDs(tss.UnSortedPartyIDs{
		tss.NewPartyID("0", "P", big.NewInt(101)),
		tss.NewPartyID("1", "P", big.NewInt(102)),
		tss.NewPartyID("2", "P", big.NewInt(103)),
		tss.NewPartyID("3", "P", big.NewInt(104)),
	})
	params4 := tss.NewParameters(ec, tss.NewPeerContext(ids4), ids4[0], 4, 1)
	ssid4, err := getSigningSSID(params4, makeSave(4), temp, 1)
	if err != nil {
		t.Fatalf("n=4: %v", err)
	}

	if bytes.Equal(ssid3, ssid4) {
		t.Fatal("different party counts must produce different SSIDs")
	}
}

// ---------- Output length ----------

func TestGetSigningSSIDOutputLength(t *testing.T) {
	params, key, temp := buildSigningSSIDFixture(t, 3, 1)
	ssid, err := getSigningSSID(params, key, temp, 1)
	if err != nil {
		t.Fatalf("getSigningSSID: %v", err)
	}
	if len(ssid) == 0 || len(ssid) > 32 {
		t.Fatalf("SSID length should be in (0, 32], got %d", len(ssid))
	}
}

// ---------- helpers ----------

// cloneLocalPartySaveData creates a shallow clone of the save data with
// deep-copied slices so mutations don't affect the original.
func cloneLocalPartySaveData(src *keygen.LocalPartySaveData, n int) *keygen.LocalPartySaveData {
	dst := keygen.NewLocalPartySaveData(n)
	for i := 0; i < n; i++ {
		dst.BigXj[i] = src.BigXj[i] // ECPoint is immutable
		dst.NTildej[i] = new(big.Int).Set(src.NTildej[i])
		dst.H1j[i] = new(big.Int).Set(src.H1j[i])
		dst.H2j[i] = new(big.Int).Set(src.H2j[i])
	}
	return &dst
}
