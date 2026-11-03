// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package common_test

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/hemilabs/x/tss/v3/common"
)

const (
	randomIntBitLen = 1024
)

func TestGetRandomInt(t *testing.T) {
	rnd := common.MustGetRandomInt(rand.Reader, randomIntBitLen)
	if rnd == nil {
		t.Fatal("rand int should not be zero")
	}
}

func TestGetRandomPositiveInt(t *testing.T) {
	rnd := common.MustGetRandomInt(rand.Reader, randomIntBitLen)
	rndPos := common.GetRandomPositiveInt(rand.Reader, rnd)
	if rndPos == nil {
		t.Fatal("rand int should not be zero")
	}
	if rndPos.Cmp(big.NewInt(0)) != 1 {
		t.Fatal("rand int should be positive")
	}
}

func TestGetRandomPositiveRelativelyPrimeInt(t *testing.T) {
	rnd := common.MustGetRandomInt(rand.Reader, randomIntBitLen)
	rndPosRP, err := common.GetRandomPositiveRelativelyPrimeInt(rand.Reader, rnd)
	if err != nil {
		t.Fatal(err)
	}
	if !common.IsNumberInMultiplicativeGroup(rnd, rndPosRP) {
		t.Fatal("expected true")
	}
	if rndPosRP.Cmp(big.NewInt(0)) != 1 {
		t.Fatal("rand int should be positive")
	}
	// TODO test for relative primeness
}

func TestGetRandomPrimeInt(t *testing.T) {
	prime := common.GetRandomPrimeInt(rand.Reader, randomIntBitLen)
	if prime == nil {
		t.Fatal("rand prime should not be zero")
	}
	if !prime.ProbablyPrime(50) {
		t.Fatal("rand prime should be prime")
	}
}

// failReader is a reader that always fails.
type failReader struct{}

func (failReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

// within runs f and fails the test if f does not return before a
// deadline.  A helper without its bound check loops forever.
func within(t *testing.T, f func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("call did not return")
	}
}

// TestRandomHelpersDegenerateBounds verifies the results of the random
// helpers at the bounds of their input.
func TestRandomHelpersDegenerateBounds(t *testing.T) {
	for _, tc := range []struct {
		bits int
		rand interface{ Read([]byte) (int, error) }
		ok   bool
	}{
		{-1, rand.Reader, false},
		{0, rand.Reader, false},
		{1, rand.Reader, false},
		{2, rand.Reader, true},
		{64, failReader{}, false},
	} {
		t.Run(fmt.Sprintf("prime bits %d %T", tc.bits, tc.rand), func(t *testing.T) {
			var p *big.Int
			within(t, func() { p = common.GetRandomPrimeInt(tc.rand, tc.bits) })
			if (p != nil) != tc.ok {
				t.Fatalf("GetRandomPrimeInt(%d) = %v, want ok %v", tc.bits, p, tc.ok)
			}
		})
	}
	for _, tc := range []struct {
		n  *big.Int
		ok bool
	}{
		{nil, false},
		{big.NewInt(0), false},
		{big.NewInt(1), false},
		{big.NewInt(2), true},
	} {
		t.Run(fmt.Sprintf("modulus %v", tc.n), func(t *testing.T) {
			var v, g *big.Int
			var err, gerr error
			within(t, func() {
				v, err = common.GetRandomPositiveRelativelyPrimeInt(rand.Reader, tc.n)
				g, gerr = common.GetRandomGeneratorOfTheQuadraticResidue(rand.Reader, tc.n)
			})
			if (err == nil) != tc.ok || (gerr == nil) != tc.ok {
				t.Fatalf("errors %v and %v, want ok %v", err, gerr, tc.ok)
			}
			if tc.ok && (v.Cmp(big.NewInt(1)) != 0 || g.Cmp(big.NewInt(1)) != 0) {
				t.Fatalf("modulus 2: got %v and %v, want 1", v, g)
			}
		})
	}
}
