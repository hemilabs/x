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

// TestGetRandomSafePrimesConcurrentReturns verifies that a search for
// small safe primes with many workers always returns.  Workers find
// primes faster than the caller reads them, so a full buffer must not
// block them after the caller returns.  A concurrency below 1 gives an
// error and does not wait forever.
func TestGetRandomSafePrimesConcurrentReturns(t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			if _, err := GetRandomSafePrimesConcurrent(context.Background(), 32, 1, 8, rand.Reader); err != nil {
				t.Error(err)
				return
			}
		}
		if _, err := GetRandomSafePrimesConcurrent(context.Background(), 32, 1, 0, rand.Reader); err == nil {
			t.Error("concurrency 0: expected an error")
		}
	}()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("GetRandomSafePrimesConcurrent did not return")
	}
}
