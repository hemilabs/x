// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.
package crypto

import (
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hemilabs/x/tss/v3/common"
)

func TestGenerateNTildeiRejectsEqualPrimes(t *testing.T) {
	// [FORK] Equal primes make NTilde = p^2, trivially factorable
	// Use a small safe prime for speed
	p := common.GetRandomPrimeInt(rand.Reader, 512)
	primes := [2]*big.Int{p, p} // same prime twice

	_, _, _, err := GenerateNTildei(rand.Reader, primes)
	assert.Error(t, err, "equal primes should be rejected")
	assert.Contains(t, err.Error(), "distinct")
}

func TestGenerateNTildeiRejectsNilPrimes(t *testing.T) {
	_, _, _, err := GenerateNTildei(rand.Reader, [2]*big.Int{nil, big.NewInt(7)})
	assert.Error(t, err, "nil prime should be rejected")

	_, _, _, err = GenerateNTildei(rand.Reader, [2]*big.Int{big.NewInt(7), nil})
	assert.Error(t, err, "nil prime should be rejected")
}

func TestGenerateNTildeiRejectsNonPrime(t *testing.T) {
	_, _, _, err := GenerateNTildei(rand.Reader, [2]*big.Int{big.NewInt(15), big.NewInt(7)})
	assert.Error(t, err, "composite number should be rejected")
}

func TestGenerateNTildeiHappyPath(t *testing.T) {
	p := common.GetRandomPrimeInt(rand.Reader, 512)
	q := common.GetRandomPrimeInt(rand.Reader, 512)
	// Ensure they're different (astronomically unlikely to be same, but be safe)
	for p.Cmp(q) == 0 {
		q = common.GetRandomPrimeInt(rand.Reader, 512)
	}

	NTilde, h1, h2, err := GenerateNTildei(rand.Reader, [2]*big.Int{p, q})
	assert.NoError(t, err)
	assert.NotNil(t, NTilde)
	assert.NotNil(t, h1)
	assert.NotNil(t, h2)

	// NTilde should equal p * q
	expected := new(big.Int).Mul(p, q)
	assert.Equal(t, 0, NTilde.Cmp(expected), "NTilde should be p * q")
}
