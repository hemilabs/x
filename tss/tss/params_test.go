// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package tss

import (
	"bytes"
	"crypto/rand"
	"math/big"
	"testing"
	"time"
)

func TestParametersGetters(t *testing.T) {
	pIDs := GenerateTestPartyIDs(3)
	ctx := NewPeerContext(pIDs)
	params := NewParameters(S256(), ctx, pIDs[0], 3, 1)

	if params.EC() != S256() {
		t.Fatal("EC mismatch")
	}
	if params.PartyID() != pIDs[0] {
		t.Fatal("PartyID mismatch")
	}
	if params.PartyCount() != 3 {
		t.Fatalf("PartyCount: want 3, got %d", params.PartyCount())
	}
	if params.Threshold() != 1 {
		t.Fatalf("Threshold: want 1, got %d", params.Threshold())
	}
	if params.Concurrency() < 1 {
		t.Fatal("Concurrency should be >= 1")
	}
	if params.SafePrimeGenTimeout() == 0 {
		t.Fatal("SafePrimeGenTimeout should have a default")
	}
	if params.PartialKeyRand() == nil {
		t.Fatal("PartialKeyRand should default to non-nil")
	}
	if params.Rand() == nil {
		t.Fatal("Rand should default to non-nil")
	}
}

func TestParametersSetters(t *testing.T) {
	pIDs := GenerateTestPartyIDs(3)
	ctx := NewPeerContext(pIDs)
	params := NewParameters(S256(), ctx, pIDs[0], 3, 1)

	params.SetConcurrency(4)
	if params.Concurrency() != 4 {
		t.Fatalf("SetConcurrency: want 4, got %d", params.Concurrency())
	}

	params.SetSafePrimeGenTimeout(10 * time.Second)
	if params.SafePrimeGenTimeout() != 10*time.Second {
		t.Fatal("SetSafePrimeGenTimeout mismatch")
	}

	customRand := bytes.NewReader(nil)
	params.SetRand(customRand)
	if params.Rand() != customRand {
		t.Fatal("SetRand mismatch")
	}

	params.SetPartialKeyRand(rand.Reader)
	if params.PartialKeyRand() != rand.Reader {
		t.Fatal("SetPartialKeyRand mismatch")
	}
}

func TestParametersProofFlags(t *testing.T) {
	pIDs := GenerateTestPartyIDs(3)
	ctx := NewPeerContext(pIDs)
	params := NewParameters(S256(), ctx, pIDs[0], 3, 1)

	if params.NoProofMod() {
		t.Fatal("NoProofMod should default to false")
	}
	if params.NoProofFac() {
		t.Fatal("NoProofFac should default to false")
	}

	params.SetNoProofMod()
	if !params.NoProofMod() {
		t.Fatal("SetNoProofMod did not set flag")
	}

	params.SetNoProofFac()
	if !params.NoProofFac() {
		t.Fatal("SetNoProofFac did not set flag")
	}
}

func TestParametersCeremonyID(t *testing.T) {
	pIDs := GenerateTestPartyIDs(3)
	ctx := NewPeerContext(pIDs)
	params := NewParameters(S256(), ctx, pIDs[0], 3, 1)

	if params.CeremonyID() != nil {
		t.Fatal("CeremonyID should default to nil")
	}

	cid := []byte("test-ceremony-42")
	params.SetCeremonyID(cid)
	if !bytes.Equal(params.CeremonyID(), cid) {
		t.Fatal("SetCeremonyID mismatch")
	}
}

func TestReSharingParametersGetters(t *testing.T) {
	oldPIDs := GenerateTestPartyIDs(3)
	newPIDs := GenerateTestPartyIDs(4)
	oldCtx := NewPeerContext(oldPIDs)
	newCtx := NewPeerContext(newPIDs)

	params := NewReSharingParameters(S256(), oldCtx, newCtx, oldPIDs[0], 3, 1, 4, 2)

	if params.OldPartyCount() != 3 {
		t.Fatalf("OldPartyCount: want 3, got %d", params.OldPartyCount())
	}
	if params.NewPartyCount() != 4 {
		t.Fatalf("NewPartyCount: want 4, got %d", params.NewPartyCount())
	}
	if params.NewThreshold() != 2 {
		t.Fatalf("NewThreshold: want 2, got %d", params.NewThreshold())
	}
	if params.OldAndNewPartyCount() != 7 {
		t.Fatalf("OldAndNewPartyCount: want 7, got %d", params.OldAndNewPartyCount())
	}
}

func TestReSharingParametersCommitteeMembership(t *testing.T) {
	oldPIDs := GenerateTestPartyIDs(3)
	newPIDs := GenerateTestPartyIDs(3)
	oldCtx := NewPeerContext(oldPIDs)
	newCtx := NewPeerContext(newPIDs)

	// Old-only party
	paramsOld := NewReSharingParameters(S256(), oldCtx, newCtx, oldPIDs[0], 3, 1, 3, 1)
	if !paramsOld.IsOldCommittee() {
		t.Fatal("oldPIDs[0] should be in old committee")
	}
	if paramsOld.IsNewCommittee() {
		t.Fatal("oldPIDs[0] should NOT be in new committee")
	}

	// New-only party
	paramsNew := NewReSharingParameters(S256(), oldCtx, newCtx, newPIDs[0], 3, 1, 3, 1)
	if paramsNew.IsOldCommittee() {
		t.Fatal("newPIDs[0] should NOT be in old committee")
	}
	if !paramsNew.IsNewCommittee() {
		t.Fatal("newPIDs[0] should be in new committee")
	}
}

func TestReSharingParametersOverlap(t *testing.T) {
	allPIDs := GenerateTestPartyIDs(4)
	copyPID := func(src *PartyID) *PartyID {
		return NewPartyID(src.Id, src.Moniker, new(big.Int).SetBytes(src.Key))
	}
	oldPIDs := SortPartyIDs(UnSortedPartyIDs{copyPID(allPIDs[0]), copyPID(allPIDs[1]), copyPID(allPIDs[2])})
	newPIDs := SortPartyIDs(UnSortedPartyIDs{copyPID(allPIDs[1]), copyPID(allPIDs[2]), copyPID(allPIDs[3])})
	oldCtx := NewPeerContext(oldPIDs)
	newCtx := NewPeerContext(newPIDs)

	// P1 (allPIDs[1]) is in both committees
	paramsDual := NewReSharingParameters(S256(), oldCtx, newCtx, allPIDs[1], 3, 1, 3, 1)
	if !paramsDual.IsOldCommittee() {
		t.Fatal("dual party should be in old committee")
	}
	if !paramsDual.IsNewCommittee() {
		t.Fatal("dual party should be in new committee")
	}
}

func TestOldAndNewPartiesLength(t *testing.T) {
	oldPIDs := GenerateTestPartyIDs(3)
	newPIDs := GenerateTestPartyIDs(2)
	oldCtx := NewPeerContext(oldPIDs)
	newCtx := NewPeerContext(newPIDs)

	params := NewReSharingParameters(S256(), oldCtx, newCtx, oldPIDs[0], 3, 1, 2, 1)
	all := params.OldAndNewParties()
	if len(all) != 5 {
		t.Fatalf("OldAndNewParties: want 5, got %d", len(all))
	}
	// Verify no aliasing with OldParties
	if len(params.OldParties().IDs()) != 3 {
		t.Fatal("OldAndNewParties corrupted OldParties")
	}
}

// TestCheckSelfAndConcurrency verifies that a bad own PartyID or party
// count gives an error, and that a low concurrency is stored as 1.
func TestCheckSelfAndConcurrency(t *testing.T) {
	pIDs := GenerateTestPartyIDs(3)
	ctx := NewPeerContext(pIDs)
	if err := NewParameters(S256(), ctx, pIDs[1], 3, 1).CheckSelf(); err != nil {
		t.Fatalf("valid parameters: %v", err)
	}
	stray := NewPartyID("x", "x", big.NewInt(99))
	stray.Index = 1
	for name, p := range map[string]*Parameters{
		"index -1":    NewParameters(S256(), ctx, NewPartyID("y", "y", big.NewInt(98)), 3, 1),
		"wrong count": NewParameters(S256(), ctx, pIDs[1], 4, 1),
		"not in list": NewParameters(S256(), ctx, stray, 3, 1),
	} {
		if err := p.CheckSelf(); err == nil {
			t.Fatalf("%s: expected an error", name)
		}
	}
	p := NewParameters(S256(), ctx, pIDs[0], 3, 1)
	p.SetConcurrency(0)
	if p.Concurrency() != 1 {
		t.Fatalf("SetConcurrency(0): got %d, want 1", p.Concurrency())
	}
}

// TestCheckCommittees verifies the reshare committee checks: a count
// that does not match the list, a party in neither committee, and a
// party in both committees.
func TestCheckCommittees(t *testing.T) {
	oldIDs, newIDs := GenerateTestPartyIDs(3), GenerateTestPartyIDs(3)
	oldCtx, newCtx := NewPeerContext(oldIDs), NewPeerContext(newIDs)
	if err := NewReSharingParameters(S256(), oldCtx, newCtx, oldIDs[0], 3, 1, 3, 1).CheckCommittees(); err != nil {
		t.Fatalf("old party: %v", err)
	}
	if err := NewReSharingParameters(S256(), oldCtx, newCtx, oldIDs[0], 3, 1, 2, 1).CheckCommittees(); err == nil {
		t.Fatal("wrong new count: expected an error")
	}
	stranger := NewPartyID("s", "s", big.NewInt(12345))
	if err := NewReSharingParameters(S256(), oldCtx, newCtx, stranger, 3, 1, 3, 1).CheckCommittees(); err == nil {
		t.Fatal("party in neither committee: expected an error")
	}
	// A party in both committees has a separate PartyID copy for each.
	both := NewPartyID(oldIDs[1].Id, oldIDs[1].Moniker, oldIDs[1].KeyInt())
	dualNew := SortPartyIDs(UnSortedPartyIDs{both, newIDs[0], newIDs[1]})
	if err := NewReSharingParameters(S256(), oldCtx, NewPeerContext(dualNew), oldIDs[1], 3, 1, 3, 1).CheckCommittees(); err != nil {
		t.Fatalf("party in both committees: %v", err)
	}
}
