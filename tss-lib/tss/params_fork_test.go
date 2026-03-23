// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package tss

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- NewParameters panic tests ---

func TestNewParametersPanicsInvalidThreshold(t *testing.T) {
	// threshold >= partyCount should panic
	pIDs := GenerateTestPartyIDs(3)
	ctx := NewPeerContext(pIDs)

	assert.Panics(t, func() {
		NewParameters(S256(), ctx, pIDs[0], 3, 3) // threshold == partyCount
	}, "threshold >= partyCount should panic")
}

func TestNewParametersPanicsNegativeThreshold(t *testing.T) {
	pIDs := GenerateTestPartyIDs(3)
	ctx := NewPeerContext(pIDs)

	assert.Panics(t, func() {
		NewParameters(S256(), ctx, pIDs[0], 3, -1)
	}, "negative threshold should panic")
}

func TestNewParametersPanicsZeroPartyCount(t *testing.T) {
	pIDs := GenerateTestPartyIDs(3)
	ctx := NewPeerContext(pIDs)

	assert.Panics(t, func() {
		NewParameters(S256(), ctx, pIDs[0], 0, 0) // partyCount < 1
	}, "zero partyCount should panic")
}

func TestNewParametersAcceptsValid(t *testing.T) {
	pIDs := GenerateTestPartyIDs(3)
	ctx := NewPeerContext(pIDs)

	assert.NotPanics(t, func() {
		p := NewParameters(S256(), ctx, pIDs[0], 3, 1)
		assert.Equal(t, 3, p.PartyCount())
		assert.Equal(t, 1, p.Threshold())
	})
}

// --- NewReSharingParameters panic tests ---

func TestNewReSharingParametersPanicsZeroNewPartyCount(t *testing.T) {
	oldIDs := GenerateTestPartyIDs(3)
	newIDs := GenerateTestPartyIDs(3, 3)
	oldCtx := NewPeerContext(oldIDs)
	newCtx := NewPeerContext(newIDs)

	assert.Panics(t, func() {
		NewReSharingParameters(S256(), oldCtx, newCtx, oldIDs[0], 3, 1, 0, 1) // newPartyCount=0
	}, "zero newPartyCount should panic")
}

func TestNewReSharingParametersPanicsInvalidNewThreshold(t *testing.T) {
	oldIDs := GenerateTestPartyIDs(3)
	newIDs := GenerateTestPartyIDs(3, 3)
	oldCtx := NewPeerContext(oldIDs)
	newCtx := NewPeerContext(newIDs)

	assert.Panics(t, func() {
		NewReSharingParameters(S256(), oldCtx, newCtx, oldIDs[0], 3, 1, 3, 3) // newThreshold == newPartyCount
	}, "newThreshold >= newPartyCount should panic")
}

func TestNewReSharingParametersPanicsNegativeNewThreshold(t *testing.T) {
	oldIDs := GenerateTestPartyIDs(3)
	newIDs := GenerateTestPartyIDs(3, 3)
	oldCtx := NewPeerContext(oldIDs)
	newCtx := NewPeerContext(newIDs)

	assert.Panics(t, func() {
		NewReSharingParameters(S256(), oldCtx, newCtx, oldIDs[0], 3, 1, 3, -1) // newThreshold=-1
	}, "negative newThreshold should panic")
}

// --- SortPartyIDs panic tests ---

func TestSortPartyIDsZeroKeyPanics(t *testing.T) {
	ids := UnSortedPartyIDs{
		NewPartyID("p1", "P1", big.NewInt(1)),
		NewPartyID("p2", "P2", big.NewInt(0)), // zero key
		NewPartyID("p3", "P3", big.NewInt(3)),
	}

	assert.Panics(t, func() {
		SortPartyIDs(ids)
	}, "zero key should panic")
}

func TestSortPartyIDsDuplicateKeyPanics_Fork(t *testing.T) {
	ids := UnSortedPartyIDs{
		NewPartyID("p1", "P1", big.NewInt(5)),
		NewPartyID("p2", "P2", big.NewInt(5)), // duplicate
		NewPartyID("p3", "P3", big.NewInt(3)),
	}

	assert.Panics(t, func() {
		SortPartyIDs(ids)
	}, "duplicate key should panic")
}

func TestSortPartyIDsAcceptsValid(t *testing.T) {
	ids := UnSortedPartyIDs{
		NewPartyID("p1", "P1", big.NewInt(3)),
		NewPartyID("p2", "P2", big.NewInt(1)),
		NewPartyID("p3", "P3", big.NewInt(2)),
	}

	sorted := SortPartyIDs(ids)
	assert.Equal(t, 3, len(sorted))
	// Should be sorted ascending by key
	assert.Equal(t, 0, sorted[0].KeyInt().Cmp(big.NewInt(1)))
	assert.Equal(t, 0, sorted[1].KeyInt().Cmp(big.NewInt(2)))
	assert.Equal(t, 0, sorted[2].KeyInt().Cmp(big.NewInt(3)))
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
	assert.Equal(t, 6, len(combined))

	// Verify old parties were not corrupted
	for i, pid := range oldIDs {
		assert.Equal(t, 0, pid.KeyInt().Cmp(oldBefore[i].KeyInt()),
			"old party %d key was corrupted by OldAndNewParties", i)
	}
}

// --- SSIDNonce uint test ---

func TestSSIDNonceUint(t *testing.T) {
	pIDs := GenerateTestPartyIDs(3)
	ctx := NewPeerContext(pIDs)
	p := NewParameters(S256(), ctx, pIDs[0], 3, 1)

	// Default nonce should be 0
	assert.Equal(t, uint(0), p.SSIDNonce())

	// Set and get
	p.SetSSIDNonce(42)
	assert.Equal(t, uint(42), p.SSIDNonce())

	// Large value (won't overflow uint)
	p.SetSSIDNonce(^uint(0)) // max uint
	assert.Equal(t, ^uint(0), p.SSIDNonce())
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
	assert.False(t, pid.ValidateBasic(), "empty key should fail ValidateBasic")
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
	assert.False(t, pid.ValidateBasic(), "nil key should fail ValidateBasic")
}

func TestPartyIDValidateBasicAcceptsValid(t *testing.T) {
	pid := NewPartyID("test", "Test", big.NewInt(42))
	pid.Index = 0
	assert.True(t, pid.ValidateBasic(), "valid party ID should pass")
}

func TestPartyIDValidateBasicRejectsNegativeIndex(t *testing.T) {
	pid := NewPartyID("test", "Test", big.NewInt(42))
	pid.Index = -1 // not yet sorted
	assert.False(t, pid.ValidateBasic(), "negative index should fail ValidateBasic")
}

// --- SortedPartyIDs Less strict ordering ---

func TestSortedPartyIDsLessIsStrict(t *testing.T) {
	// [FORK] Upstream uses <= (treats equal as less-than), fork uses < (strict)
	ids := SortedPartyIDs{
		NewPartyID("a", "A", big.NewInt(5)),
		NewPartyID("b", "B", big.NewInt(5)), // same key
	}
	// With strict Less, neither should be "less than" the other
	assert.False(t, ids.Less(0, 1), "equal keys: Less(0,1) should be false (strict)")
	assert.False(t, ids.Less(1, 0), "equal keys: Less(1,0) should be false (strict)")
}
