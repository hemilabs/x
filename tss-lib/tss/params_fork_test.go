// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package tss

import (
	"math/big"
	"reflect"
	"testing"
)

// --- NewParameters panic tests ---

func TestNewParametersPanicsInvalidThreshold(t *testing.T) {
	// threshold >= partyCount should panic
	pIDs := GenerateTestPartyIDs(3)
	ctx := NewPeerContext(pIDs)

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("threshold >= partyCount should panic")
			}
		}()
		NewParameters(S256(), ctx, pIDs[0], 3, 3) // threshold == partyCount
	}()
}

func TestNewParametersPanicsNegativeThreshold(t *testing.T) {
	pIDs := GenerateTestPartyIDs(3)
	ctx := NewPeerContext(pIDs)

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("negative threshold should panic")
			}
		}()
		NewParameters(S256(), ctx, pIDs[0], 3, -1)
	}()
}

func TestNewParametersPanicsZeroPartyCount(t *testing.T) {
	pIDs := GenerateTestPartyIDs(3)
	ctx := NewPeerContext(pIDs)

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("zero partyCount should panic")
			}
		}()
		NewParameters(S256(), ctx, pIDs[0], 0, 0) // partyCount < 1
	}()
}

func TestNewParametersAcceptsValid(t *testing.T) {
	pIDs := GenerateTestPartyIDs(3)
	ctx := NewPeerContext(pIDs)

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("unexpected panic: %v", r)
			}
		}()
		p := NewParameters(S256(), ctx, pIDs[0], 3, 1)
		if p.PartyCount() != 3 {
			t.Fatalf("got %v, want 3", p.PartyCount())
		}
		if p.Threshold() != 1 {
			t.Fatalf("got %v, want 1", p.Threshold())
		}
	}()
}

// --- NewReSharingParameters panic tests ---

func TestNewReSharingParametersPanicsZeroNewPartyCount(t *testing.T) {
	oldIDs := GenerateTestPartyIDs(3)
	newIDs := GenerateTestPartyIDs(3, 3)
	oldCtx := NewPeerContext(oldIDs)
	newCtx := NewPeerContext(newIDs)

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("zero newPartyCount should panic")
			}
		}()
		NewReSharingParameters(S256(), oldCtx, newCtx, oldIDs[0], 3, 1, 0, 1) // newPartyCount=0
	}()
}

func TestNewReSharingParametersPanicsInvalidNewThreshold(t *testing.T) {
	oldIDs := GenerateTestPartyIDs(3)
	newIDs := GenerateTestPartyIDs(3, 3)
	oldCtx := NewPeerContext(oldIDs)
	newCtx := NewPeerContext(newIDs)

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("newThreshold >= newPartyCount should panic")
			}
		}()
		NewReSharingParameters(S256(), oldCtx, newCtx, oldIDs[0], 3, 1, 3, 3) // newThreshold == newPartyCount
	}()
}

func TestNewReSharingParametersPanicsNegativeNewThreshold(t *testing.T) {
	oldIDs := GenerateTestPartyIDs(3)
	newIDs := GenerateTestPartyIDs(3, 3)
	oldCtx := NewPeerContext(oldIDs)
	newCtx := NewPeerContext(newIDs)

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("negative newThreshold should panic")
			}
		}()
		NewReSharingParameters(S256(), oldCtx, newCtx, oldIDs[0], 3, 1, 3, -1) // newThreshold=-1
	}()
}

// --- SortPartyIDs panic tests ---

func TestSortPartyIDsZeroKeyPanics(t *testing.T) {
	ids := UnSortedPartyIDs{
		NewPartyID("p1", "P1", big.NewInt(1)),
		NewPartyID("p2", "P2", big.NewInt(0)), // zero key
		NewPartyID("p3", "P3", big.NewInt(3)),
	}

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("zero key should panic")
			}
		}()
		SortPartyIDs(ids)
	}()
}

func TestSortPartyIDsDuplicateKeyPanics_Fork(t *testing.T) {
	ids := UnSortedPartyIDs{
		NewPartyID("p1", "P1", big.NewInt(5)),
		NewPartyID("p2", "P2", big.NewInt(5)), // duplicate
		NewPartyID("p3", "P3", big.NewInt(3)),
	}

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("duplicate key should panic")
			}
		}()
		SortPartyIDs(ids)
	}()
}

func TestSortPartyIDsAcceptsValid(t *testing.T) {
	ids := UnSortedPartyIDs{
		NewPartyID("p1", "P1", big.NewInt(3)),
		NewPartyID("p2", "P2", big.NewInt(1)),
		NewPartyID("p3", "P3", big.NewInt(2)),
	}

	sorted := SortPartyIDs(ids)
	if len(sorted) != 3 {
		t.Fatalf("got %v, want %v", len(sorted), 3)
	}
	// Should be sorted ascending by key
	if sorted[0].KeyInt().Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("got %v, want %v", sorted[0].KeyInt().Cmp(big.NewInt(1)), 0)
	}
	if sorted[1].KeyInt().Cmp(big.NewInt(2)) != 0 {
		t.Fatalf("got %v, want %v", sorted[1].KeyInt().Cmp(big.NewInt(2)), 0)
	}
	if sorted[2].KeyInt().Cmp(big.NewInt(3)) != 0 {
		t.Fatalf("got %v, want %v", sorted[2].KeyInt().Cmp(big.NewInt(3)), 0)
	}
}

// --- OldAndNewParties aliasing fix test ---

func TestOldAndNewPartiesNoAliasing(t *testing.T) {
	oldIDs := GenerateTestPartyIDs(3)
	newIDs := GenerateTestPartyIDs(3, 3) // start at index 3

	oldCtx := NewPeerContext(oldIDs)
	newCtx := NewPeerContext(newIDs)

	params := NewReSharingParameters(S256(), oldCtx, newCtx, oldIDs[0], 3, 1, 3, 1)

	// Save original old parties
	oldBefore := make([]*PartyID, len(oldIDs))
	copy(oldBefore, oldIDs)

	// Call OldAndNewParties
	combined := params.OldAndNewParties()

	// Verify combined has all 6 parties
	if len(combined) != 6 {
		t.Fatalf("got %v, want %v", len(combined), 6)
	}

	// Verify old parties were not corrupted
	for i, pid := range oldIDs {
		if pid.KeyInt().Cmp(oldBefore[i].KeyInt()) != 0 {
			t.Fatalf("old party %d key was corrupted by OldAndNewParties", i)
		}
	}
}

// --- SSIDNonce uint test ---

func TestSSIDNonceUint(t *testing.T) {
	pIDs := GenerateTestPartyIDs(3)
	ctx := NewPeerContext(pIDs)
	p := NewParameters(S256(), ctx, pIDs[0], 3, 1)

	// Default nonce should be 0
	if !reflect.DeepEqual(uint(0), p.SSIDNonce()) {
		t.Fatalf("got %v, want %v", p.SSIDNonce(), uint(0))
	}

	// Set and get
	p.SetSSIDNonce(42)
	if !reflect.DeepEqual(uint(42), p.SSIDNonce()) {
		t.Fatalf("got %v, want %v", p.SSIDNonce(), uint(42))
	}

	// Large value (won't overflow uint)
	p.SetSSIDNonce(^uint(0)) // max uint
	if !reflect.DeepEqual(^uint(0), p.SSIDNonce()) {
		t.Fatalf("got %v, want %v", p.SSIDNonce(), ^uint(0))
	}
}

// --- PartyID ValidateBasic tests ---

func TestPartyIDValidateBasicRejectsEmptyKey(t *testing.T) {
	// [FORK] Upstream checks Key != nil but not len(Key) > 0.
	// An empty byte slice passes nil check but KeyInt() returns 0.
	pid := &PartyID{
		PartyIDData: &PartyIDData{
			Id:      "test",
			Moniker: "Test",
			Key:     []byte{}, // empty, not nil
		},
		Index: 0,
	}
	if pid.ValidateBasic() {
		t.Fatal("empty key should fail ValidateBasic")
	}
}

func TestPartyIDValidateBasicRejectsNilKey(t *testing.T) {
	pid := &PartyID{
		PartyIDData: &PartyIDData{
			Id:      "test",
			Moniker: "Test",
			Key:     nil,
		},
		Index: 0,
	}
	if pid.ValidateBasic() {
		t.Fatal("nil key should fail ValidateBasic")
	}
}

func TestPartyIDValidateBasicAcceptsValid(t *testing.T) {
	pid := NewPartyID("test", "Test", big.NewInt(42))
	pid.Index = 0
	if !pid.ValidateBasic() {
		t.Fatal("valid party ID should pass")
	}
}

func TestPartyIDValidateBasicRejectsNegativeIndex(t *testing.T) {
	pid := NewPartyID("test", "Test", big.NewInt(42))
	pid.Index = -1 // not yet sorted
	if pid.ValidateBasic() {
		t.Fatal("negative index should fail ValidateBasic")
	}
}

// --- SortedPartyIDs Less strict ordering ---

func TestSortedPartyIDsLessIsStrict(t *testing.T) {
	// [FORK] Upstream uses <= (treats equal as less-than), fork uses < (strict)
	ids := SortedPartyIDs{
		NewPartyID("a", "A", big.NewInt(5)),
		NewPartyID("b", "B", big.NewInt(5)), // same key
	}
	// With strict Less, neither should be "less than" the other
	if ids.Less(0, 1) {
		t.Fatal("equal keys: Less(0,1) should be false (strict)")
	}
	if ids.Less(1, 0) {
		t.Fatal("equal keys: Less(1,0) should be false (strict)")
	}
}
