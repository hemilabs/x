// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package dlnproof

import (
	"math/big"
	"testing"
)

func TestUnmarshalDLNProofTooFewParts(t *testing.T) {
	_, err := UnmarshalDLNProof([][]byte{big.NewInt(1).Bytes(), big.NewInt(2).Bytes()})
	if err == nil {
		t.Fatal("expected error for too few parts")
	}
}

func TestUnmarshalDLNProofEmpty(t *testing.T) {
	_, err := UnmarshalDLNProof(nil)
	if err == nil {
		t.Fatal("expected error for nil input")
	}
}

// TestVerifyBadInputsNoPanic verifies that Verify returns false for a
// nil h1, and for a negative T when h1 has no inverse mod N.
func TestVerifyBadInputsNoPanic(t *testing.T) {
	// N = 3 * (2^2046 + 1) is odd and 2048 bits; h1 = 3 shares a factor.
	N := new(big.Int).Mul(big.NewInt(3), new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 2046), big.NewInt(1)))
	p := &Proof{}
	for i := range p.T {
		p.Alpha[i], p.T[i] = big.NewInt(2), big.NewInt(2)
	}
	if p.Verify([]byte("session"), nil, big.NewInt(5), N) {
		t.Fatal("Verify accepted a nil h1")
	}
	p.T[0] = big.NewInt(-2)
	if p.Verify([]byte("session"), big.NewInt(3), big.NewInt(5), N) {
		t.Fatal("Verify accepted a negative T")
	}
}
