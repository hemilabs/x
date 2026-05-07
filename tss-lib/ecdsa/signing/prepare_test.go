// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package signing

import (
	"math/big"
	"testing"


	"github.com/hemilabs/x/tss-lib/v3/crypto"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

func TestPrepareForSigningNoXiMutation(t *testing.T) {
	// Setup: 3 parties, threshold=1
	ec := tss.S256()
	q := ec.Params().N

	// Create distinct party keys
	ks := []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)}
	xi := new(big.Int).SetInt64(42) // Some secret share
	xiCopy := new(big.Int).Set(xi)  // Save original value

	// Create dummy public keys (just generator multiples)
	bigXs := make([]*crypto.ECPoint, 3)
	for j := 0; j < 3; j++ {
		bigXs[j] = crypto.ScalarBaseMult(ec, big.NewInt(int64(j+10)))
	}

	// Call PrepareForSigning for party 0
	_, _ = PrepareForSigning(ec, 0, 3, xi, ks, bigXs)

	// xi should NOT have been mutated
	if xi.Cmp(xiCopy) != 0 {
		t.Fatalf("xi must not be mutated by PrepareForSigning")
	}
	_ = q // suppress unused warning if needed
}

func TestPrepareForSigningCollidingKeysPanics(t *testing.T) {
	ec := tss.S256()

	ks := []*big.Int{big.NewInt(1), big.NewInt(1), big.NewInt(3)} // ks[0] == ks[1]
	xi := big.NewInt(42)

	bigXs := make([]*crypto.ECPoint, 3)
	for j := 0; j < 3; j++ {
		bigXs[j] = crypto.ScalarBaseMult(ec, big.NewInt(int64(j+10)))
	}

	func() {
		defer func() {
			if r := recover(); r == nil {
			t.Fatal("colliding keys should panic")
		}
		}()
		PrepareForSigning(ec, 0, 3, xi, ks, bigXs)
	}()
}
