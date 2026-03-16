// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package common_test

import (
	"crypto/rand"
	"math/big"
	"reflect"
	"testing"

	"github.com/hemilabs/x/tss-lib/v3/common"
)

// TestRejectionSampleMutatesInput verifies that RejectionSample does NOT
// mutate its eHash argument (the in-place mutation bug has been fixed).
func TestRejectionSampleMutatesInput(t *testing.T) {
	q := big.NewInt(100)
	original := big.NewInt(257)
	originalCopy := new(big.Int).Set(original)

	result := common.RejectionSample(q, original)

	// Result is 257 mod 100 = 57.
	if result.Cmp(big.NewInt(57)) != 0 {
		t.Fatalf("expected 57, got %s", result)
	}

	// Verify the input was NOT mutated (fixed from the original in-place mutation bug).
	if original.Cmp(originalCopy) != 0 {
		t.Fatalf("RejectionSample mutated input: was %s, now %s", originalCopy, original)
	}
}

func TestRejectionSample(t *testing.T) {
	curveQ := common.GetRandomPrimeInt(rand.Reader, 256)
	randomQ := common.MustGetRandomInt(rand.Reader, 64)
	hash := common.SHA512_256iOne(big.NewInt(123))
	rs1 := common.RejectionSample(curveQ, hash)
	rs2 := common.RejectionSample(randomQ, hash)
	rs3 := common.RejectionSample(common.MustGetRandomInt(rand.Reader, 64), hash)
	type args struct {
		q     *big.Int
		eHash *big.Int
	}
	tests := []struct {
		name       string
		args       args
		want       *big.Int
		wantBitLen int
		notEqual   bool
	}{{
		name:       "happy path with curve order",
		args:       args{curveQ, hash},
		want:       rs1,
		wantBitLen: 256,
	}, {
		name:       "happy path with random 64-bit int",
		args:       args{randomQ, hash},
		want:       rs2,
		wantBitLen: 64,
	}, {
		name:       "inequality with different input",
		args:       args{randomQ, hash},
		want:       rs3,
		wantBitLen: 64,
		notEqual:   true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := common.RejectionSample(tt.args.q, tt.args.eHash)
			if !tt.notEqual && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RejectionSample() = %v, want %v", got, tt.want)
			}
			if tt.wantBitLen < got.BitLen() { // leading zeros not counted
				t.Errorf("RejectionSample() = bitlen %d, want %d", got.BitLen(), tt.wantBitLen)
			}
		})
	}
}

// TestRejectionSampleDoesNotMutateQ verifies that RejectionSample does not
// mutate the q argument.
func TestRejectionSampleDoesNotMutateQ(t *testing.T) {
	q := big.NewInt(100)
	qCopy := new(big.Int).Set(q)

	common.RejectionSample(q, big.NewInt(257))

	if q.Cmp(qCopy) != 0 {
		t.Fatalf("RejectionSample mutated q: was %s, now %s", qCopy, q)
	}
}

// TestRejectionSampleQEqualsOne verifies that when q=1, the result is always
// 0 because any integer mod 1 = 0.
func TestRejectionSampleQEqualsOne(t *testing.T) {
	hashes := []*big.Int{
		big.NewInt(0),
		big.NewInt(1),
		big.NewInt(12345),
		big.NewInt(999999999),
	}
	for _, hash := range hashes {
		result := common.RejectionSample(big.NewInt(1), hash)
		if result.Cmp(big.NewInt(0)) != 0 {
			t.Fatalf("RejectionSample(1, %s) = %s, want 0", hash, result)
		}
	}
}

// TestRejectionSampleResultInRange verifies that for 100 random q values and
// hashes, the result is always in the range [0, q).
func TestRejectionSampleResultInRange(t *testing.T) {
	for i := 0; i < 100; i++ {
		q := common.MustGetRandomInt(rand.Reader, 128)
		// Ensure q > 0 (MustGetRandomInt may return 0 in degenerate cases).
		q.Add(q, big.NewInt(1))
		hash := common.MustGetRandomInt(rand.Reader, 256)

		result := common.RejectionSample(q, hash)

		if result.Sign() < 0 {
			t.Fatalf("iteration %d: result %s is negative", i, result)
		}
		if result.Cmp(q) >= 0 {
			t.Fatalf("iteration %d: result %s >= q %s", i, result, q)
		}
	}
}

// TestRejectionSampleGoldenVector freezes a golden vector for cross-language
// verification. RejectionSample(100, 257) = 257 mod 100 = 57.
func TestRejectionSampleGoldenVector(t *testing.T) {
	result := common.RejectionSample(big.NewInt(100), big.NewInt(257))
	if result.Cmp(big.NewInt(57)) != 0 {
		t.Fatalf("RejectionSample(100, 257) = %s, want 57", result)
	}
}

// TestRejectionSampleNegativeHash verifies behavior with a negative hash.
// In Go, big.Int.Mod(-257, 100) returns 43 (Go's Mod follows the sign of
// the divisor, returning a non-negative result). Rust implementations
// using unsigned types will need to handle this differently.
func TestRejectionSampleNegativeHash(t *testing.T) {
	result := common.RejectionSample(big.NewInt(100), big.NewInt(-257))
	// Go's Mod: -257 mod 100 = 43 (non-negative, follows divisor sign).
	if result.Cmp(big.NewInt(43)) != 0 {
		t.Fatalf("RejectionSample(100, -257) = %s, want 43", result)
	}
}
