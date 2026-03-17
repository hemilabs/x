// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package paillier

import (
	"math/big"
	"testing"
)

func TestPublicKeyAsInts(t *testing.T) {
	pk := &PublicKey{N: big.NewInt(100)}
	ints := pk.AsInts()
	if len(ints) != 2 {
		t.Fatalf("expected 2 ints, got %d", len(ints))
	}
	if ints[0].Cmp(big.NewInt(100)) != 0 {
		t.Fatalf("N mismatch: got %v", ints[0])
	}
	// Gamma = N+1 = 101
	if ints[1].Cmp(big.NewInt(101)) != 0 {
		t.Fatalf("Gamma mismatch: got %v", ints[1])
	}
}
