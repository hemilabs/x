// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package common

import (
	"context"
	"crypto/rand"
	"math/big"
	"runtime"
	"strconv"
	"testing"
	"time"
)

func Test_getSafePrime(t *testing.T) {
	prime := new(big.Int).SetInt64(5)
	sPrime := getSafePrime(prime)
	if !sPrime.ProbablyPrime(50) {
		t.Fatal("expected true")
	}
}

func Test_getSafePrime_Bad(t *testing.T) {
	prime := new(big.Int).SetInt64(12)
	sPrime := getSafePrime(prime)
	if sPrime.ProbablyPrime(50) {
		t.Fatal("expected false")
	}
}

func Test_Validate(t *testing.T) {
	prime := new(big.Int).SetInt64(5)
	sPrime := getSafePrime(prime)
	sgp := &GermainSafePrime{prime, sPrime}
	if !sgp.Validate() {
		t.Fatal("expected true")
	}
}

func Test_Validate_Bad(t *testing.T) {
	prime := new(big.Int).SetInt64(12)
	sPrime := getSafePrime(prime)
	sgp := &GermainSafePrime{prime, sPrime}
	if sgp.Validate() {
		t.Fatal("expected false")
	}
}

func TestGetRandomGermainPrimeConcurrent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	sgps, err := GetRandomSafePrimesConcurrent(ctx, 1024, 2, runtime.NumCPU(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if len(sgps) != 2 {
		t.Fatalf("got %v, want %v", len(sgps), 2)
	}
	for _, sgp := range sgps {
		if sgp == nil {
			t.Fatal("expected non-nil")
		}
		if !sgp.Validate() {
			t.Fatal("expected true")
		}
	}
}

// TestGetRandomSafePrimesConcurrentReturns verifies that the search
// returns when its workers find more primes than it needs.  A worker that
// blocks on its send makes the deferred Wait hang, so the loop runs under
// a deadline.  With 8 workers and 32-bit primes, the extra workers find
// primes within the 200 runs.
func TestGetRandomSafePrimesConcurrentReturns(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	errc := make(chan error, 1)
	go func() {
		for i := 0; i < 200; i++ {
			if _, err := GetRandomSafePrimesConcurrent(ctx, 32, 1, 8, rand.Reader); err != nil {
				errc <- err
				return
			}
		}
		errc <- nil
	}()
	select {
	case err := <-errc:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("safe-prime search did not return")
	}
}

// TestGetRandomSafePrimesConcurrentBadConcurrency verifies that a
// concurrency below 1 gives an error.
func TestGetRandomSafePrimesConcurrentBadConcurrency(t *testing.T) {
	for _, c := range []int{0, -1} {
		t.Run(strconv.Itoa(c), func(t *testing.T) {
			if _, err := GetRandomSafePrimesConcurrent(t.Context(), 32, 1, c, rand.Reader); err == nil {
				t.Fatalf("concurrency %d: expected an error", c)
			}
		})
	}
}
