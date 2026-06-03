// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package crypto

import (
	"crypto/rand"
	"math/big"
	"strings"
	"testing"

	"github.com/hemilabs/x/tss-lib/v3/common"
)

func TestGenerateNTildeiRejectsEqualPrimes(t *testing.T) {
	// [FORK] Equal primes make NTilde = p^2, trivially factorable
	// Use a small safe prime for speed
	p := common.GetRandomPrimeInt(rand.Reader, 512)
	primes := [2]*big.Int{p, p} // same prime twice

	_, _, _, err := GenerateNTildei(rand.Reader, primes)
	if err == nil {
		t.Fatal("equal primes should be rejected")
	}
	if !strings.Contains(err.Error(), "distinct") {
		t.Fatalf("expected %q to contain %q", err.Error(), "distinct")
	}
}

func TestGenerateNTildeiRejectsNilPrimes(t *testing.T) {
	_, _, _, err := GenerateNTildei(rand.Reader, [2]*big.Int{nil, big.NewInt(7)})
	if err == nil {
		t.Fatal("nil prime should be rejected")
	}

	_, _, _, err = GenerateNTildei(rand.Reader, [2]*big.Int{big.NewInt(7), nil})
	if err == nil {
		t.Fatal("nil prime should be rejected")
	}
}

func TestGenerateNTildeiRejectsNonPrime(t *testing.T) {
	_, _, _, err := GenerateNTildei(rand.Reader, [2]*big.Int{big.NewInt(15), big.NewInt(7)})
	if err == nil {
		t.Fatal("composite number should be rejected")
	}
}

func TestGenerateNTildeiHappyPath(t *testing.T) {
	p := common.GetRandomPrimeInt(rand.Reader, 512)
	q := common.GetRandomPrimeInt(rand.Reader, 512)
	// Ensure they're different (astronomically unlikely to be same, but be safe)
	for p.Cmp(q) == 0 {
		q = common.GetRandomPrimeInt(rand.Reader, 512)
	}

	NTilde, h1, h2, err := GenerateNTildei(rand.Reader, [2]*big.Int{p, q})
	if err != nil {
		t.Fatal(err)
	}
	if NTilde == nil {
		t.Fatal("expected non-nil")
	}
	if h1 == nil {
		t.Fatal("expected non-nil")
	}
	if h2 == nil {
		t.Fatal("expected non-nil")
	}

	// NTilde should equal p * q
	expected := new(big.Int).Mul(p, q)
	if NTilde.Cmp(expected) != 0 {
		t.Fatalf("NTilde should be p * q")
	}
}
