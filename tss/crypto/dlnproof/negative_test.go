// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package dlnproof

import (
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss/v3/common"
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

// TestVerifyRejectsNonUnitBase verifies that Verify returns false when
// h1 shares a factor with N.
func TestVerifyRejectsNonUnitBase(t *testing.T) {
	// h1 = 3 shares a factor with N.  h2 = h1^2, so a prover that
	// knows x = 2 makes a proof that passes the equation.
	N := new(big.Int).Mul(big.NewInt(3), new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 2046), big.NewInt(1)))
	h1 := big.NewInt(3)
	h2 := new(big.Int).Exp(h1, big.NewInt(2), N)
	session := []byte("session")
	p := &Proof{}
	r := big.NewInt(5)
	for i := range p.Alpha {
		p.Alpha[i] = new(big.Int).Exp(h1, r, N)
	}
	c := common.SHA512_256i_TAGGED(session, append([]*big.Int{h1, h2, N}, p.Alpha[:]...)...)
	for i := range p.T {
		p.T[i] = new(big.Int).Add(r, big.NewInt(2*int64(c.Bit(i))))
	}
	if p.Verify(session, h1, h2, N) {
		t.Fatal("Verify accepted an h1 that shares a factor with N")
	}
}
