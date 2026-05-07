// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"sync/atomic"
	"reflect"
	"testing"
	"time"

)

// TestGeneratePreParamsMultipleConcurrencyArgsPanics verifies that passing more
// than one optionalConcurrency argument triggers a panic, as documented by the
// function contract.
func TestGeneratePreParamsMultipleConcurrencyArgsPanics(t *testing.T) {
	func() {
		defer func() {
			if r := recover(); r == nil {
			t.Fatal("expected panic when multiple concurrency args are provided")
		}
		}()
		ctx := context.Background()
		_, _ = GeneratePreParamsWithContextAndRandom(ctx, rand.Reader, 2, 4)
	}()
}

// TestGeneratePreParamsContextAlreadyCancelled verifies that passing an
// already-cancelled context returns an error immediately without blocking.
func TestGeneratePreParamsContextAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately before calling

	start := time.Now()
	preParams, err := GeneratePreParamsWithContext(ctx, 1)
	elapsed := time.Since(start)

	if preParams != nil {
		t.Fatalf("expected nil, got %v", preParams)
	}
	if err == nil {
		t.Fatal("should return an error with cancelled context")
	}
	if elapsed >= 2*time.Second {
		t.Fatal("should return quickly, not block on prime generation")
	}
}

// TestGeneratePreParamsContextAlreadyCancelledAndRandom exercises the
// WithContextAndRandom variant with an already-cancelled context.
func TestGeneratePreParamsContextAlreadyCancelledAndRandom(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	preParams, err := GeneratePreParamsWithContextAndRandom(ctx, rand.Reader, 1)
	elapsed := time.Since(start)

	if preParams != nil {
		t.Fatalf("expected nil, got %v", preParams)
	}
	if err == nil {
		t.Fatal("expected error")
	}
	if elapsed >= 2*time.Second {
		t.Fatalf("expected %v < %v", elapsed, 2*time.Second)
	}
}

// TestGeneratePreParamsZeroConcurrency verifies that passing concurrency=0
// does not panic and is handled gracefully (the code divides by 3 and clamps
// to minimum 1).
func TestGeneratePreParamsZeroConcurrency(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	// Should not panic even with concurrency=0; the function clamps to 1.
	preParams, err := GeneratePreParamsWithContextAndRandom(ctx, rand.Reader, 0)
	// Will timeout (5ms is too short for real primes), but should not panic.
	if preParams != nil {
		t.Fatalf("expected nil, got %v", preParams)
	}
	if err == nil {
		t.Fatal("expected error")
	}
}

// TestGeneratePreParamsNegativeConcurrency verifies that a negative
// concurrency value is handled gracefully (clamped to 1 after division).
func TestGeneratePreParamsNegativeConcurrency(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	preParams, err := GeneratePreParamsWithContextAndRandom(ctx, rand.Reader, -3)
	if preParams != nil {
		t.Fatalf("expected nil, got %v", preParams)
	}
	if err == nil {
		t.Fatal("expected error")
	}
}

// failingReader is an io.Reader that always returns an error.
// It exercises the error path when the rand reader fails during prime generation.
type failingReader struct{}

func (f *failingReader) Read([]byte) (int, error) {
	return 0, errors.New("simulated RNG failure")
}

// TestGeneratePreParamsFailingRandReader verifies that a broken rand reader
// causes an error (not a panic) and returns promptly.
func TestGeneratePreParamsFailingRandReader(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	preParams, err := GeneratePreParamsWithContextAndRandom(ctx, &failingReader{}, 1)
	if preParams != nil {
		t.Fatalf("expected nil, got %v", preParams)
	}
	if err == nil {
		t.Fatal("should return an error when rand reader fails")
	}
}

// countingReader wraps an io.Reader and counts how many bytes are read.
// Used to verify that a custom rand reader is actually being consumed.
// bytesRead uses atomic access because the production code passes the
// reader to multiple concurrent goroutines.
type countingReader struct {
	inner     io.Reader
	bytesRead atomic.Int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.inner.Read(p)
	c.bytesRead.Add(int64(n))
	return n, err
}

// TestGeneratePreParamsCustomRandIsUsed verifies that the custom rand.Reader
// parameter is actually consumed during prime generation. We use a short
// timeout so we don't wait for full prime generation, but even the initial
// attempts should read from our custom reader.
func TestGeneratePreParamsCustomRandIsUsed(t *testing.T) {
	cr := &countingReader{inner: rand.Reader}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Will almost certainly timeout, but that's fine — we just want to confirm
	// the custom reader was used.
	_, _ = GeneratePreParamsWithContextAndRandom(ctx, cr, 1)

	if cr.bytesRead.Load() <= int64(0) {
		t.Fatal("custom rand reader should have been read from")
	}
}

// TestGeneratePreParamsResultValidates generates real pre-params (slow!) and
// verifies that both Validate() and ValidateWithProof() return true, and that
// all fields are non-nil with correct algebraic relationships.
func TestGeneratePreParamsResultValidates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow safe prime generation in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	preParams, err := GeneratePreParamsWithContextAndRandom(ctx, rand.Reader, 1)
	if err != nil {
		t.Fatalf("GeneratePreParams should succeed with sufficient timeout"+": %v", err)
	}
	if preParams == nil {
		t.Fatal("expected non-nil")
	}

	// Structural validation (nil checks)
	if !preParams.Validate() {
		t.Fatal("Validate() should return true")
	}

	// Full algebraic validation (NTilde = (2P+1)(2Q+1), H2 = H1^Alpha mod NTilde)
	if !preParams.ValidateWithProof() {
		t.Fatal("ValidateWithProof() should return true")
	}

	// Verify all fields are populated
	if preParams.PaillierSK == nil {
		t.Fatal("expected non-nil")
	}
	if preParams.PaillierSK.N == nil {
		t.Fatal("expected non-nil")
	}
	if preParams.PaillierSK.LambdaN == nil {
		t.Fatal("expected non-nil")
	}
	if preParams.NTildei == nil {
		t.Fatal("expected non-nil")
	}
	if preParams.H1i == nil {
		t.Fatal("expected non-nil")
	}
	if preParams.H2i == nil {
		t.Fatal("expected non-nil")
	}
	if preParams.Alpha == nil {
		t.Fatal("expected non-nil")
	}
	if preParams.Beta == nil {
		t.Fatal("expected non-nil")
	}
	if preParams.P == nil {
		t.Fatal("expected non-nil")
	}
	if preParams.Q == nil {
		t.Fatal("expected non-nil")
	}

	// P != Q (defense-in-depth — astronomically unlikely but tested)
	if preParams.P.Cmp(preParams.Q) == 0 {
		t.Fatalf("P and Q should be distinct")
	}

	// Bit lengths: P, Q should be ~1024-bit safe primes (their "prime" halves)
	if preParams.P.BitLen() < 1000 {
		t.Fatal("P bit length should be ~1024")
	}
	if preParams.Q.BitLen() < 1000 {
		t.Fatal("Q bit length should be ~1024")
	}

	// NTilde bit length should be ~2048
	if preParams.NTildei.BitLen() < 2000 {
		t.Fatal("NTilde should be ~2048 bits")
	}

	// Paillier modulus should be 2048 bits
	if !reflect.DeepEqual(paillierModulusLen, preParams.PaillierSK.N.BitLen()) {
		t.Fatalf("Paillier modulus should be exactly 2048 bits")
	}
}

// TestGeneratePreParamsTimeoutWrapper verifies that the timeout-based wrapper
// GeneratePreParams correctly propagates context cancellation.
func TestGeneratePreParamsTimeoutWrapper(t *testing.T) {
	// 1ms timeout — will definitely fail, but should return an error not panic
	start := time.Now()
	preParams, err := GeneratePreParams(1*time.Millisecond, 1)
	elapsed := time.Since(start)

	if preParams != nil {
		t.Fatalf("expected nil, got %v", preParams)
	}
	if err == nil {
		t.Fatal("expected error")
	}
	if elapsed >= 5*time.Second {
		t.Fatal("should respect the timeout")
	}
}

// TestGeneratePreParamsWithContextCallsWithContextAndRandom verifies that
// GeneratePreParamsWithContext delegates to GeneratePreParamsWithContextAndRandom.
// We do this by passing a cancelled context — both functions should behave
// identically.
func TestGeneratePreParamsWithContextCallsWithContextAndRandom(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	pp1, err1 := GeneratePreParamsWithContext(ctx, 1)
	pp2, err2 := GeneratePreParamsWithContextAndRandom(ctx, rand.Reader, 1)

	if pp1 != nil {
		t.Fatalf("expected nil, got %v", pp1)
	}
	if pp2 != nil {
		t.Fatalf("expected nil, got %v", pp2)
	}
	if err1 == nil {
		t.Fatal("expected error")
	}
	if err2 == nil {
		t.Fatal("expected error")
	}
}
