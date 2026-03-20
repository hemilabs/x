// Copyright (c) 2025 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package tss

import (
	"fmt"
	"math/big"
	"testing"
)

// ---------------------------------------------------------------------------
// Parameters accessor tests
// ---------------------------------------------------------------------------

// TestParametersSSIDNonceAccessors verifies that SetSSIDNonce/SSIDNonce work
// correctly and that the default nonce is 0.
func TestParametersSSIDNonceAccessors(t *testing.T) {
	pIDs := GenerateTestPartyIDs(2)
	p2pCtx := NewPeerContext(pIDs)
	params := NewParameters(S256(), p2pCtx, pIDs[0], 2, 1)

	// Default nonce should be 0.
	if params.SSIDNonce() != 0 {
		t.Fatalf("default SSIDNonce should be 0, got %d", params.SSIDNonce())
	}

	// Set and get.
	params.SetSSIDNonce(42)
	if params.SSIDNonce() != 42 {
		t.Fatalf("SSIDNonce should be 42, got %d", params.SSIDNonce())
	}

	// Set to a different value.
	params.SetSSIDNonce(999)
	if params.SSIDNonce() != 999 {
		t.Fatalf("SSIDNonce should be 999, got %d", params.SSIDNonce())
	}

	// Set back to 0.
	params.SetSSIDNonce(0)
	if params.SSIDNonce() != 0 {
		t.Fatalf("SSIDNonce should be 0, got %d", params.SSIDNonce())
	}
}

// TestParametersNoProofDLNAccessors verifies that SetNoProofDLN/NoProofDLN work
// correctly and that the default is false.
func TestParametersNoProofDLNAccessors(t *testing.T) {
	pIDs := GenerateTestPartyIDs(2)
	p2pCtx := NewPeerContext(pIDs)
	params := NewParameters(S256(), p2pCtx, pIDs[0], 2, 1)

	// Default should be false.
	if params.NoProofDLN() {
		t.Fatal("default NoProofDLN should be false")
	}

	// Set and get.
	params.SetNoProofDLN()
	if !params.NoProofDLN() {
		t.Fatal("NoProofDLN should be true after SetNoProofDLN()")
	}
}

// ---------------------------------------------------------------------------
// PartyID tests
// ---------------------------------------------------------------------------

// TestSortedPartyIDsLessEqualKeys verifies that Less() uses strict < (not <=),
// complying with the sort.Interface contract: Less(a,a) must be false.
func TestSortedPartyIDsLessEqualKeys(t *testing.T) {
	sameKey := big.NewInt(42)
	p1 := NewPartyID("p1", "Party1", sameKey)
	p2 := NewPartyID("p2", "Party2", sameKey)

	spids := SortedPartyIDs{p1, p2}

	less01 := spids.Less(0, 1)
	less10 := spids.Less(1, 0)

	// With strict <, neither direction should be true for equal keys.
	if less01 {
		t.Fatal("Less(0,1) should be false for equal keys")
	}
	if less10 {
		t.Fatal("Less(1,0) should be false for equal keys")
	}
	// Also verify reflexivity: Less(a,a) must be false.
	if spids.Less(0, 0) {
		t.Fatal("Less(0,0) should be false (irreflexivity)")
	}
}

// TestSortedPartyIDsLessDistinctKeys verifies correct ordering for distinct keys.
func TestSortedPartyIDsLessDistinctKeys(t *testing.T) {
	p1 := NewPartyID("p1", "Party1", big.NewInt(1))
	p2 := NewPartyID("p2", "Party2", big.NewInt(2))

	spids := SortedPartyIDs{p1, p2}

	if !spids.Less(0, 1) {
		t.Fatal("expected Less(0,1) = true for key 1 < key 2")
	}
	if spids.Less(1, 0) {
		t.Fatal("expected Less(1,0) = false for key 2 > key 1")
	}
}

// TestSortPartyIDsDuplicateKeyPanics verifies that SortPartyIDs panics when
// two parties have the same key.
func TestSortPartyIDsDuplicateKeyPanics(t *testing.T) {
	sameKey := big.NewInt(42)
	p1 := NewPartyID("p1", "Party1", sameKey)
	p2 := NewPartyID("p2", "Party2", sameKey)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("SortPartyIDs should panic on duplicate keys")
		}
	}()
	SortPartyIDs(UnSortedPartyIDs{p1, p2})
}

// TestSortPartyIDsUniqueKeys verifies that SortPartyIDs succeeds with unique keys.
func TestSortPartyIDsUniqueKeys(t *testing.T) {
	p1 := NewPartyID("p1", "Party1", big.NewInt(3))
	p2 := NewPartyID("p2", "Party2", big.NewInt(1))
	p3 := NewPartyID("p3", "Party3", big.NewInt(2))

	sorted := SortPartyIDs(UnSortedPartyIDs{p1, p2, p3})

	// Should be sorted by key: 1, 2, 3.
	if sorted[0].KeyInt().Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("expected key 1 at index 0, got %s", sorted[0].KeyInt())
	}
	if sorted[1].KeyInt().Cmp(big.NewInt(2)) != 0 {
		t.Fatalf("expected key 2 at index 1, got %s", sorted[1].KeyInt())
	}
	if sorted[2].KeyInt().Cmp(big.NewInt(3)) != 0 {
		t.Fatalf("expected key 3 at index 2, got %s", sorted[2].KeyInt())
	}
}

// TestSortPartyIDsEmptyInput verifies that SortPartyIDs handles empty and nil
// input without panicking and returns an empty slice.
func TestSortPartyIDsEmptyInput(t *testing.T) {
	sorted := SortPartyIDs(UnSortedPartyIDs{})
	if len(sorted) != 0 {
		t.Fatalf("expected empty slice, got length %d", len(sorted))
	}

	sorted = SortPartyIDs(nil)
	if len(sorted) != 0 {
		t.Fatalf("expected empty slice for nil input, got length %d", len(sorted))
	}
}

// TestSortPartyIDsSingleParty verifies that a single party gets Index 0 after sorting.
func TestSortPartyIDsSingleParty(t *testing.T) {
	p := NewPartyID("p1", "Party1", big.NewInt(99))
	sorted := SortPartyIDs(UnSortedPartyIDs{p})

	if len(sorted) != 1 {
		t.Fatalf("expected length 1, got %d", len(sorted))
	}
	if sorted[0].Index != 0 {
		t.Fatalf("expected Index 0, got %d", sorted[0].Index)
	}
}

// TestSortPartyIDsStartAt verifies that SortPartyIDs with a startAt parameter
// assigns indices starting at the given offset.
func TestSortPartyIDsStartAt(t *testing.T) {
	p1 := NewPartyID("p1", "Party1", big.NewInt(10))
	p2 := NewPartyID("p2", "Party2", big.NewInt(20))
	p3 := NewPartyID("p3", "Party3", big.NewInt(30))

	sorted := SortPartyIDs(UnSortedPartyIDs{p3, p1, p2}, 5)

	for i, pid := range sorted {
		expected := i + 5
		if pid.Index != expected {
			t.Fatalf("sorted[%d].Index = %d, want %d", i, pid.Index, expected)
		}
	}
}

// TestSortPartyIDsIndexAssignment verifies that after sorting 5 parties,
// sorted[i].Index == i for all i.
func TestSortPartyIDsIndexAssignment(t *testing.T) {
	ids := make(UnSortedPartyIDs, 5)
	for i := 0; i < 5; i++ {
		ids[i] = NewPartyID(
			fmt.Sprintf("p%d", i),
			fmt.Sprintf("Party%d", i),
			big.NewInt(int64((i+1)*100)),
		)
	}

	sorted := SortPartyIDs(ids)

	if len(sorted) != 5 {
		t.Fatalf("expected length 5, got %d", len(sorted))
	}
	for i, pid := range sorted {
		if pid.Index != i {
			t.Fatalf("sorted[%d].Index = %d, want %d", i, pid.Index, i)
		}
	}
}

// TestSortPartyIDsAlreadySorted verifies that input already in ascending key
// order produces the same key order after sorting.
func TestSortPartyIDsAlreadySorted(t *testing.T) {
	p1 := NewPartyID("p1", "Party1", big.NewInt(1))
	p2 := NewPartyID("p2", "Party2", big.NewInt(2))
	p3 := NewPartyID("p3", "Party3", big.NewInt(3))

	sorted := SortPartyIDs(UnSortedPartyIDs{p1, p2, p3})

	expectedKeys := []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)}
	for i, pid := range sorted {
		if pid.KeyInt().Cmp(expectedKeys[i]) != 0 {
			t.Fatalf("sorted[%d] key = %s, want %s", i, pid.KeyInt(), expectedKeys[i])
		}
	}
}

// TestSortPartyIDsReverseSorted verifies that input in descending key order
// gets reversed after sorting.
func TestSortPartyIDsReverseSorted(t *testing.T) {
	p1 := NewPartyID("p1", "Party1", big.NewInt(30))
	p2 := NewPartyID("p2", "Party2", big.NewInt(20))
	p3 := NewPartyID("p3", "Party3", big.NewInt(10))

	sorted := SortPartyIDs(UnSortedPartyIDs{p1, p2, p3})

	expectedKeys := []*big.Int{big.NewInt(10), big.NewInt(20), big.NewInt(30)}
	for i, pid := range sorted {
		if pid.KeyInt().Cmp(expectedKeys[i]) != 0 {
			t.Fatalf("sorted[%d] key = %s, want %s", i, pid.KeyInt(), expectedKeys[i])
		}
	}
}

// TestPartyIDValidateBasicEmptyKey verifies ValidateBasic rejects empty keys.
// An empty key would produce KeyInt() == 0, breaking protocol invariants
// (e.g., duplicate detection, Lagrange interpolation).
func TestPartyIDValidateBasicEmptyKey(t *testing.T) {
	pid := &PartyID{
		PartyIDData: &PartyIDData{Key: []byte{}},
		Index:       0,
	}
	result := pid.ValidateBasic()
	if result {
		t.Fatal("expected ValidateBasic() = false for empty Key")
	}
}

// TestPartyIDValidateBasicNilPid verifies that calling ValidateBasic on a nil
// *PartyID returns false.
func TestPartyIDValidateBasicNilPid(t *testing.T) {
	var pid *PartyID
	if pid.ValidateBasic() {
		t.Fatal("expected ValidateBasic() = false for nil *PartyID")
	}
}

// TestPartyIDValidateBasicNegativeIndex verifies that a PartyID created via
// NewPartyID (which sets Index to -1) fails ValidateBasic.
func TestPartyIDValidateBasicNegativeIndex(t *testing.T) {
	pid := NewPartyID("p1", "Party1", big.NewInt(42))
	if pid.Index != -1 {
		t.Fatalf("expected NewPartyID to set Index = -1, got %d", pid.Index)
	}
	if pid.ValidateBasic() {
		t.Fatal("expected ValidateBasic() = false for unsorted party with Index -1")
	}
}

// TestSortedPartyIDsFindByKey verifies that FindByKey returns the correct party
// for an existing key and nil for a missing key.
func TestSortedPartyIDsFindByKey(t *testing.T) {
	p1 := NewPartyID("p1", "Party1", big.NewInt(10))
	p2 := NewPartyID("p2", "Party2", big.NewInt(20))
	p3 := NewPartyID("p3", "Party3", big.NewInt(30))

	sorted := SortPartyIDs(UnSortedPartyIDs{p3, p1, p2})

	// Find an existing key.
	found := sorted.FindByKey(big.NewInt(20))
	if found == nil {
		t.Fatal("expected to find party with key 20, got nil")
	}
	if found.KeyInt().Cmp(big.NewInt(20)) != 0 {
		t.Fatalf("found party has key %s, want 20", found.KeyInt())
	}

	// Find a missing key.
	missing := sorted.FindByKey(big.NewInt(999))
	if missing != nil {
		t.Fatalf("expected nil for missing key, got %v", missing)
	}
}

// TestSortedPartyIDsExclude verifies that Exclude removes the specified party
// and the remaining count is correct.
func TestSortedPartyIDsExclude(t *testing.T) {
	p1 := NewPartyID("p1", "Party1", big.NewInt(10))
	p2 := NewPartyID("p2", "Party2", big.NewInt(20))
	p3 := NewPartyID("p3", "Party3", big.NewInt(30))

	sorted := SortPartyIDs(UnSortedPartyIDs{p1, p2, p3})

	// Exclude the middle party (key 20).
	remaining := sorted.Exclude(p2)
	if len(remaining) != 2 {
		t.Fatalf("expected 2 remaining parties, got %d", len(remaining))
	}

	// Verify that the excluded party is not present.
	for _, pid := range remaining {
		if pid.KeyInt().Cmp(big.NewInt(20)) == 0 {
			t.Fatal("excluded party (key 20) should not be in remaining set")
		}
	}

	// Verify the remaining keys are correct.
	expectedKeys := []*big.Int{big.NewInt(10), big.NewInt(30)}
	for i, pid := range remaining {
		if pid.KeyInt().Cmp(expectedKeys[i]) != 0 {
			t.Fatalf("remaining[%d] key = %s, want %s", i, pid.KeyInt(), expectedKeys[i])
		}
	}
}

// TestSortedPartyIDsKeys verifies that Keys() returns the correct slice of
// *big.Int values in sorted order.
func TestSortedPartyIDsKeys(t *testing.T) {
	p1 := NewPartyID("p1", "Party1", big.NewInt(50))
	p2 := NewPartyID("p2", "Party2", big.NewInt(10))
	p3 := NewPartyID("p3", "Party3", big.NewInt(30))

	sorted := SortPartyIDs(UnSortedPartyIDs{p1, p2, p3})

	keys := sorted.Keys()
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}

	expectedKeys := []*big.Int{big.NewInt(10), big.NewInt(30), big.NewInt(50)}
	for i, k := range keys {
		if k.Cmp(expectedKeys[i]) != 0 {
			t.Fatalf("keys[%d] = %s, want %s", i, k, expectedKeys[i])
		}
	}
}

// TestGenerateTestPartyIDsStartAt verifies that GenerateTestPartyIDs with a
// startAt parameter assigns indices starting at the given offset.
func TestGenerateTestPartyIDsStartAt(t *testing.T) {
	sorted := GenerateTestPartyIDs(3, 5)
	if len(sorted) != 3 {
		t.Fatalf("expected 3 parties, got %d", len(sorted))
	}
	for i, pid := range sorted {
		expected := i + 5
		if pid.Index != expected {
			t.Fatalf("sorted[%d].Index = %d, want %d", i, pid.Index, expected)
		}
	}
}

// TestGenerateTestPartyIDsDefaultStartAt verifies that GenerateTestPartyIDs
// without a startAt parameter assigns indices starting at 0.
func TestGenerateTestPartyIDsDefaultStartAt(t *testing.T) {
	sorted := GenerateTestPartyIDs(4)
	if len(sorted) != 4 {
		t.Fatalf("expected 4 parties, got %d", len(sorted))
	}
	for i, pid := range sorted {
		if pid.Index != i {
			t.Fatalf("sorted[%d].Index = %d, want %d", i, pid.Index, i)
		}
	}
}

// TestGenerateTestPartyIDsUniqueKeys verifies that all generated parties have
// distinct keys.
func TestGenerateTestPartyIDsUniqueKeys(t *testing.T) {
	sorted := GenerateTestPartyIDs(5)
	seen := make(map[string]bool)
	for _, pid := range sorted {
		keyHex := pid.KeyInt().Text(16)
		if seen[keyHex] {
			t.Fatalf("duplicate key found: %s", keyHex)
		}
		seen[keyHex] = true
	}
}

// TestSortedPartyIDsToUnSorted verifies that ToUnSorted returns the same
// underlying parties (same keys and indices) as UnSortedPartyIDs.
func TestSortedPartyIDsToUnSorted(t *testing.T) {
	p1 := NewPartyID("p1", "Party1", big.NewInt(10))
	p2 := NewPartyID("p2", "Party2", big.NewInt(20))
	p3 := NewPartyID("p3", "Party3", big.NewInt(30))

	sorted := SortPartyIDs(UnSortedPartyIDs{p3, p1, p2})
	unsorted := sorted.ToUnSorted()

	if len(unsorted) != len(sorted) {
		t.Fatalf("expected %d parties, got %d", len(sorted), len(unsorted))
	}

	// Same pointers, same order.
	for i := range sorted {
		if sorted[i] != unsorted[i] {
			t.Fatalf("sorted[%d] and unsorted[%d] point to different PartyIDs", i, i)
		}
	}
}

// TestNewPartyIDZeroKey verifies that NewPartyID with big.NewInt(0) sets
// Key to []byte{} (empty, since big.Int(0).Bytes() returns empty).
// This is important for cross-language compatibility.
func TestNewPartyIDZeroKey(t *testing.T) {
	pid := NewPartyID("p0", "Party0", big.NewInt(0))

	// big.NewInt(0).Bytes() = []byte{} (empty).
	if len(pid.Key) != 0 {
		t.Fatalf("expected empty Key for big.NewInt(0), got %v", pid.Key)
	}

	// KeyInt() should still reconstruct to 0.
	if pid.KeyInt().Cmp(big.NewInt(0)) != 0 {
		t.Fatalf("KeyInt() = %s, want 0", pid.KeyInt())
	}

	// Index should be -1 (unsorted).
	if pid.Index != -1 {
		t.Fatalf("expected Index -1, got %d", pid.Index)
	}
}

// TestPartyIDStringFormat verifies the String() format: "{Index,Moniker}".
func TestPartyIDStringFormat(t *testing.T) {
	pid := NewPartyID("p1", "Alice", big.NewInt(42))
	// Before sorting, Index is -1.
	expected := "{-1,Alice}"
	got := pid.String()
	if got != expected {
		t.Fatalf("String() = %q, want %q", got, expected)
	}

	// After sorting, Index becomes 0.
	sorted := SortPartyIDs(UnSortedPartyIDs{pid})
	expected = fmt.Sprintf("{0,%s}", "Alice")
	got = sorted[0].String()
	if got != expected {
		t.Fatalf("String() after sort = %q, want %q", got, expected)
	}
}

// TestSSIDNonceUintType verifies that SSIDNonce is uint, preventing the
// sign collision where big.Int.Bytes() drops the sign (so -N and N would
// produce identical SSID inputs if int were allowed).
func TestSSIDNonceUintType(t *testing.T) {
	// Verify that distinct nonces produce different byte representations.
	zero := new(big.Int).SetUint64(0).Bytes()
	one := new(big.Int).SetUint64(1).Bytes()
	if string(zero) == string(one) {
		t.Fatal("nonce 0 and 1 should produce different bytes")
	}

	// SetSSIDNonce now takes uint, preventing negative values at compile time.
	pIDs := GenerateTestPartyIDs(2)
	p2pCtx := NewPeerContext(pIDs)
	params := NewParameters(S256(), p2pCtx, pIDs[0], 2, 1)
	params.SetSSIDNonce(1)
	if params.SSIDNonce() != 1 {
		t.Fatalf("SSIDNonce should be 1, got %d", params.SSIDNonce())
	}

	// Verify large uint values work correctly.
	params.SetSSIDNonce(^uint(0)) // max uint
	if params.SSIDNonce() != ^uint(0) {
		t.Fatalf("SSIDNonce should be max uint, got %d", params.SSIDNonce())
	}
}

// TestPartyIDStringFormatMultiParty verifies String() format for multiple parties.
func TestPartyIDStringFormatMultiParty(t *testing.T) {
	p1 := NewPartyID("p1", "Alice", big.NewInt(10))
	p2 := NewPartyID("p2", "Bob", big.NewInt(20))
	p3 := NewPartyID("p3", "Charlie", big.NewInt(30))

	sorted := SortPartyIDs(UnSortedPartyIDs{p3, p1, p2})

	// After sorting: Alice(10) at 0, Bob(20) at 1, Charlie(30) at 2.
	tests := []struct {
		index    int
		expected string
	}{
		{0, "{0,Alice}"},
		{1, "{1,Bob}"},
		{2, "{2,Charlie}"},
	}
	for _, tc := range tests {
		got := sorted[tc.index].String()
		if got != tc.expected {
			t.Fatalf("sorted[%d].String() = %q, want %q", tc.index, got, tc.expected)
		}
	}
}
