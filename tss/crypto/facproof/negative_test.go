// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package facproof

import (
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss/v3/common"
	"github.com/hemilabs/x/tss/v3/crypto"
	"github.com/hemilabs/x/tss/v3/tss"
)

var negSession = []byte("facproof-neg-test")

// --- Verify with wrong N0 ---

func TestVerifyWrongN0(t *testing.T) {
	ec := tss.EC()
	N0p := common.GetRandomPrimeInt(rand.Reader, 512)
	N0q := common.GetRandomPrimeInt(rand.Reader, 512)
	N0 := new(big.Int).Mul(N0p, N0q)

	primes := [2]*big.Int{
		common.GetRandomPrimeInt(rand.Reader, 512),
		common.GetRandomPrimeInt(rand.Reader, 512),
	}
	NCap, s, tt, err := crypto.GenerateNTildei(rand.Reader, primes)
	if err != nil {
		t.Fatalf("GenerateNTildei: %v", err)
	}

	proof, err := NewProof(negSession, ec, N0, NCap, s, tt, N0p, N0q, rand.Reader)
	if err != nil {
		t.Fatalf("NewProof: %v", err)
	}

	wrongN0 := new(big.Int).Add(N0, big.NewInt(2))
	if proof.Verify(negSession, ec, wrongN0, NCap, s, tt) {
		t.Fatal("proof should fail with wrong N0")
	}
}

// --- NewProofFromBytes truncated ---

func TestNewProofFromBytesTruncated(t *testing.T) {
	_, err := NewProofFromBytes([][]byte{{1}, {2}})
	if err == nil {
		t.Fatal("expected error for truncated bytes")
	}
}

// --- ValidateBasic for RangeProofAlice/ProofBob ---
// These are tested via the mta tests already, but let's test NewProof error (nil N0)

func TestNewProofNilN0(t *testing.T) {
	ec := tss.EC()
	primes := [2]*big.Int{
		common.GetRandomPrimeInt(rand.Reader, 512),
		common.GetRandomPrimeInt(rand.Reader, 512),
	}
	NCap, s, tt, _ := crypto.GenerateNTildei(rand.Reader, primes)
	_, err := NewProof(negSession, ec, nil, NCap, s, tt, big.NewInt(3), big.NewInt(5), rand.Reader)
	if err == nil {
		t.Fatal("expected error for nil N0")
	}
}

// TestVerifyRejectsBadBases verifies that Verify returns false when t
// is 0 or has no inverse mod NCap.
func TestVerifyRejectsBadBases(t *testing.T) {
	pf := &ProofFac{}
	for _, f := range []**big.Int{&pf.P, &pf.Q, &pf.A, &pf.B, &pf.T, &pf.Sigma, &pf.Z1, &pf.Z2, &pf.W1, &pf.W2, &pf.V} {
		*f = big.NewInt(1)
	}
	pf.W1 = big.NewInt(-1)
	N0 := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 2047), big.NewInt(1))
	NCap := new(big.Int).Mul(big.NewInt(3), new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 2046), big.NewInt(1)))
	for _, tv := range []*big.Int{big.NewInt(0), big.NewInt(3)} {
		if pf.Verify([]byte("session"), tss.EC(), N0, NCap, big.NewInt(2), tv) {
			t.Fatalf("Verify accepted t = %v", tv)
		}
	}
}
