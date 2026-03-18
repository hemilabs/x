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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGeneratePreParamsMultipleConcurrencyArgsPanics verifies that passing more
// than one optionalConcurrency argument triggers a panic, as documented by the
// function contract.
func TestGeneratePreParamsMultipleConcurrencyArgsPanics(t *testing.T) {
	assert.Panics(t, func() {
		ctx := context.Background()
		_, _ = GeneratePreParamsWithContextAndRandom(ctx, rand.Reader, 2, 4)
	}, "expected panic when multiple concurrency args are provided")
}

// TestGeneratePreParamsContextAlreadyCancelled verifies that passing an
// already-cancelled context returns an error immediately without blocking.
func TestGeneratePreParamsContextAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately before calling

	start := time.Now()
	preParams, err := GeneratePreParamsWithContext(ctx, 1)
	elapsed := time.Since(start)

	assert.Nil(t, preParams, "preParams should be nil with cancelled context")
	assert.Error(t, err, "should return an error with cancelled context")
	assert.Less(t, elapsed, 2*time.Second, "should return quickly, not block on prime generation")
}

// TestGeneratePreParamsContextAlreadyCancelledAndRandom exercises the
// WithContextAndRandom variant with an already-cancelled context.
func TestGeneratePreParamsContextAlreadyCancelledAndRandom(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	preParams, err := GeneratePreParamsWithContextAndRandom(ctx, rand.Reader, 1)
	elapsed := time.Since(start)

	assert.Nil(t, preParams)
	assert.Error(t, err)
	assert.Less(t, elapsed, 2*time.Second)
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
	assert.Nil(t, preParams)
	assert.Error(t, err)
}

// TestGeneratePreParamsNegativeConcurrency verifies that a negative
// concurrency value is handled gracefully (clamped to 1 after division).
func TestGeneratePreParamsNegativeConcurrency(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	preParams, err := GeneratePreParamsWithContextAndRandom(ctx, rand.Reader, -3)
	assert.Nil(t, preParams)
	assert.Error(t, err)
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
	assert.Nil(t, preParams, "preParams should be nil when rand reader fails")
	assert.Error(t, err, "should return an error when rand reader fails")
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

	assert.Greater(t, cr.bytesRead.Load(), int64(0), "custom rand reader should have been read from")
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
	require.NoError(t, err, "GeneratePreParams should succeed with sufficient timeout")
	require.NotNil(t, preParams)

	// Structural validation (nil checks)
	assert.True(t, preParams.Validate(), "Validate() should return true")

	// Full algebraic validation (NTilde = (2P+1)(2Q+1), H2 = H1^Alpha mod NTilde)
	assert.True(t, preParams.ValidateWithProof(), "ValidateWithProof() should return true")

	// Verify all fields are populated
	assert.NotNil(t, preParams.PaillierSK)
	assert.NotNil(t, preParams.PaillierSK.PublicKey.N)
	assert.NotNil(t, preParams.PaillierSK.LambdaN)
	assert.NotNil(t, preParams.NTildei)
	assert.NotNil(t, preParams.H1i)
	assert.NotNil(t, preParams.H2i)
	assert.NotNil(t, preParams.Alpha)
	assert.NotNil(t, preParams.Beta)
	assert.NotNil(t, preParams.P)
	assert.NotNil(t, preParams.Q)

	// P != Q (defense-in-depth — astronomically unlikely but tested)
	assert.NotEqual(t, 0, preParams.P.Cmp(preParams.Q), "P and Q should be distinct")

	// Bit lengths: P, Q should be ~1024-bit safe primes (their "prime" halves)
	assert.GreaterOrEqual(t, preParams.P.BitLen(), 1000, "P bit length should be ~1024")
	assert.GreaterOrEqual(t, preParams.Q.BitLen(), 1000, "Q bit length should be ~1024")

	// NTilde bit length should be ~2048
	assert.GreaterOrEqual(t, preParams.NTildei.BitLen(), 2000, "NTilde should be ~2048 bits")

	// Paillier modulus should be 2048 bits
	assert.Equal(t, paillierModulusLen, preParams.PaillierSK.PublicKey.N.BitLen(),
		"Paillier modulus should be exactly 2048 bits")
}

// TestGeneratePreParamsTimeoutWrapper verifies that the timeout-based wrapper
// GeneratePreParams correctly propagates context cancellation.
func TestGeneratePreParamsTimeoutWrapper(t *testing.T) {
	// 1ms timeout — will definitely fail, but should return an error not panic
	start := time.Now()
	preParams, err := GeneratePreParams(1*time.Millisecond, 1)
	elapsed := time.Since(start)

	assert.Nil(t, preParams)
	assert.Error(t, err)
	assert.Less(t, elapsed, 5*time.Second, "should respect the timeout")
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

	assert.Nil(t, pp1)
	assert.Nil(t, pp2)
	assert.Error(t, err1)
	assert.Error(t, err2)
}
