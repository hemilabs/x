// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"context"
	"crypto/rand"
	"math/big"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hemilabs/x/tss-lib/v3/common"
	"github.com/hemilabs/x/tss-lib/v3/crypto/dlnproof"
)

// dlnTestParams holds a set of DLN proof parameters generated from real
// safe primes.  Creating these is slow (~seconds) so tests that need them
// should call generateDLNTestParams once and reuse the result.
type dlnTestParams struct {
	H1, H2  *big.Int
	Alpha   *big.Int // discrete log: H2 = H1^Alpha mod N
	Beta    *big.Int // modular inverse of Alpha mod p*q
	P, Q    *big.Int // Sophie Germain primes
	N       *big.Int // N = (2P+1)(2Q+1)
	Session []byte
}

// generateDLNTestParams generates proper DLN proof parameters at runtime
// using safe primes, mirroring the logic in GeneratePreParamsWithContextAndRandom.
func generateDLNTestParams(t *testing.T) *dlnTestParams {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	concurrency := runtime.NumCPU()
	if concurrency < 1 {
		concurrency = 1
	}

	sgps, err := common.GetRandomSafePrimesConcurrent(ctx, 1024, 2, concurrency, rand.Reader)
	require.NoError(t, err, "safe prime generation failed")

	p := sgps[0].Prime()
	q := sgps[1].Prime()
	safeP := sgps[0].SafePrime()
	safeQ := sgps[1].SafePrime()
	N := new(big.Int).Mul(safeP, safeQ)

	modN := common.ModInt(N)
	pMulQ := new(big.Int).Mul(p, q)
	modPQ := common.ModInt(pMulQ)

	f := common.GetRandomPositiveRelativelyPrimeInt(rand.Reader, N)
	h1 := modN.Mul(f, f)

	alpha := common.GetRandomPositiveRelativelyPrimeInt(rand.Reader, N)
	alphaModPQ := new(big.Int).Mod(alpha, pMulQ)
	beta := modPQ.ModInverse(alphaModPQ)
	require.NotNil(t, beta, "alpha modular inverse failed")

	h2 := modN.Exp(h1, alpha)

	return &dlnTestParams{
		H1:      h1,
		H2:      h2,
		Alpha:   alphaModPQ,
		Beta:    beta,
		P:       p,
		Q:       q,
		N:       N,
		Session: []byte("dln-verifier-test"),
	}
}

// TestNewDlnProofVerifierZeroConcurrencyPanics verifies that constructing a
// DlnProofVerifier with concurrency=0 panics, as documented.
func TestNewDlnProofVerifierZeroConcurrencyPanics(t *testing.T) {
	assert.Panics(t, func() {
		NewDlnProofVerifier(0)
	}, "concurrency=0 must panic")
}

// TestNewDlnProofVerifierValidConcurrency verifies that concurrency values
// 1 and greater succeed without panic.
func TestNewDlnProofVerifierValidConcurrency(t *testing.T) {
	for _, c := range []int{1, 2, 4, 128} {
		dpv := NewDlnProofVerifier(c)
		assert.NotNil(t, dpv, "concurrency=%d should create valid verifier", c)
	}
}

// TestVerifyDLNProofSuccess creates a real DLN proof from safe-prime
// parameters and verifies that VerifyDLNProof calls onDone(true).
func TestVerifyDLNProofSuccess(t *testing.T) {
	params := generateDLNTestParams(t)
	proof := dlnproof.NewDLNProof(
		params.Session, params.H1, params.H2,
		params.Alpha, params.P, params.Q, params.N,
		rand.Reader,
	)
	require.NotNil(t, proof)

	dpv := NewDlnProofVerifier(1)
	var result atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)
	dpv.VerifyDLNProof(proof, params.Session, params.H1, params.H2, params.N, func(ok bool) {
		result.Store(ok)
		wg.Done()
	})
	wg.Wait()
	assert.True(t, result.Load(), "valid proof must pass verification")
}

// TestVerifyDLNProofIncorrectH1 creates a valid proof then verifies with a
// tampered H1 value, expecting onDone(false).
func TestVerifyDLNProofIncorrectH1(t *testing.T) {
	params := generateDLNTestParams(t)
	proof := dlnproof.NewDLNProof(
		params.Session, params.H1, params.H2,
		params.Alpha, params.P, params.Q, params.N,
		rand.Reader,
	)
	require.NotNil(t, proof)

	// Tamper: H1 + 2 (still odd, still in range, but wrong)
	badH1 := new(big.Int).Add(params.H1, big.NewInt(2))

	dpv := NewDlnProofVerifier(1)
	var result atomic.Bool
	result.Store(true) // pre-set to true to detect false negative
	var wg sync.WaitGroup
	wg.Add(1)
	dpv.VerifyDLNProof(proof, params.Session, badH1, params.H2, params.N, func(ok bool) {
		result.Store(ok)
		wg.Done()
	})
	wg.Wait()
	assert.False(t, result.Load(), "tampered H1 must cause verification failure")
}

// TestVerifyDLNProofIncorrectH2 creates a valid proof then verifies with a
// tampered H2 value, expecting onDone(false).
func TestVerifyDLNProofIncorrectH2(t *testing.T) {
	params := generateDLNTestParams(t)
	proof := dlnproof.NewDLNProof(
		params.Session, params.H1, params.H2,
		params.Alpha, params.P, params.Q, params.N,
		rand.Reader,
	)
	require.NotNil(t, proof)

	// Tamper: H2 + 2
	badH2 := new(big.Int).Add(params.H2, big.NewInt(2))

	dpv := NewDlnProofVerifier(1)
	var result atomic.Bool
	result.Store(true)
	var wg sync.WaitGroup
	wg.Add(1)
	dpv.VerifyDLNProof(proof, params.Session, params.H1, badH2, params.N, func(ok bool) {
		result.Store(ok)
		wg.Done()
	})
	wg.Wait()
	assert.False(t, result.Load(), "tampered H2 must cause verification failure")
}

// TestVerifyDLNProofWrongSession creates a proof with one session ID and
// verifies with a different one, expecting onDone(false).  This exercises
// the SSID domain-separation fork (SHA512_256i_TAGGED with Session).
func TestVerifyDLNProofWrongSession(t *testing.T) {
	params := generateDLNTestParams(t)
	proof := dlnproof.NewDLNProof(
		params.Session, params.H1, params.H2,
		params.Alpha, params.P, params.Q, params.N,
		rand.Reader,
	)
	require.NotNil(t, proof)

	wrongSession := []byte("wrong-session-id")

	dpv := NewDlnProofVerifier(1)
	var result atomic.Bool
	result.Store(true)
	var wg sync.WaitGroup
	wg.Add(1)
	dpv.VerifyDLNProof(proof, wrongSession, params.H1, params.H2, params.N, func(ok bool) {
		result.Store(ok)
		wg.Done()
	})
	wg.Wait()
	assert.False(t, result.Load(), "wrong session must cause verification failure")
}

// TestVerifyDLNProofNilProof passes a nil proof pointer and verifies that
// onDone(false) is called (SNARK mode path).
func TestVerifyDLNProofNilProof(t *testing.T) {
	dpv := NewDlnProofVerifier(1)
	var result atomic.Bool
	result.Store(true)
	var wg sync.WaitGroup
	wg.Add(1)
	dpv.VerifyDLNProof(nil, []byte("session"), big.NewInt(3), big.NewInt(5), big.NewInt(15), func(ok bool) {
		result.Store(ok)
		wg.Done()
	})
	wg.Wait()
	assert.False(t, result.Load(), "nil proof must call onDone(false)")
}

// TestVerifyDLNProofNilProofCallbackInvoked ensures that with a nil proof the
// callback is always invoked exactly once (no deadlock, no double-call).
func TestVerifyDLNProofNilProofCallbackInvoked(t *testing.T) {
	dpv := NewDlnProofVerifier(2)
	var count atomic.Int32
	var wg sync.WaitGroup

	const iterations = 10
	wg.Add(iterations)
	for i := 0; i < iterations; i++ {
		dpv.VerifyDLNProof(nil, []byte("s"), big.NewInt(3), big.NewInt(5), big.NewInt(15), func(ok bool) {
			assert.False(t, ok)
			count.Add(1)
			wg.Done()
		})
	}
	wg.Wait()
	assert.Equal(t, int32(iterations), count.Load(), "callback must be invoked exactly once per call")
}

// TestVerifyDLNProofConcurrencyBound launches more verifications than the
// concurrency limit and verifies that all complete successfully.  This
// exercises the semaphore: with concurrency=2 and 20 verifications, the
// goroutines must queue on the semaphore.
func TestVerifyDLNProofConcurrencyBound(t *testing.T) {
	params := generateDLNTestParams(t)
	proof := dlnproof.NewDLNProof(
		params.Session, params.H1, params.H2,
		params.Alpha, params.P, params.Q, params.N,
		rand.Reader,
	)
	require.NotNil(t, proof)

	const concurrency = 2
	const numVerifications = 20

	dpv := NewDlnProofVerifier(concurrency)
	var successCount atomic.Int32
	var failCount atomic.Int32
	var wg sync.WaitGroup
	wg.Add(numVerifications)

	for i := 0; i < numVerifications; i++ {
		dpv.VerifyDLNProof(proof, params.Session, params.H1, params.H2, params.N, func(ok bool) {
			if ok {
				successCount.Add(1)
			} else {
				failCount.Add(1)
			}
			wg.Done()
		})
	}

	wg.Wait()
	assert.Equal(t, int32(numVerifications), successCount.Load(),
		"all %d verifications must succeed", numVerifications)
	assert.Equal(t, int32(0), failCount.Load(),
		"no verifications should fail")
}

// TestVerifyDLNProofConcurrencyBoundMixed launches a mix of valid and nil
// proofs beyond the concurrency limit to verify that the semaphore properly
// serializes work and all callbacks fire correctly.
func TestVerifyDLNProofConcurrencyBoundMixed(t *testing.T) {
	params := generateDLNTestParams(t)
	proof := dlnproof.NewDLNProof(
		params.Session, params.H1, params.H2,
		params.Alpha, params.P, params.Q, params.N,
		rand.Reader,
	)
	require.NotNil(t, proof)

	const concurrency = 3
	const numValid = 10
	const numNil = 10
	const total = numValid + numNil

	dpv := NewDlnProofVerifier(concurrency)
	var successCount atomic.Int32
	var failCount atomic.Int32
	var wg sync.WaitGroup
	wg.Add(total)

	// Interleave valid and nil proofs.
	for i := 0; i < total; i++ {
		var p *dlnproof.Proof
		if i%2 == 0 {
			p = proof
		}
		dpv.VerifyDLNProof(p, params.Session, params.H1, params.H2, params.N, func(ok bool) {
			if ok {
				successCount.Add(1)
			} else {
				failCount.Add(1)
			}
			wg.Done()
		})
	}

	wg.Wait()
	assert.Equal(t, int32(numValid), successCount.Load(), "valid proofs must succeed")
	assert.Equal(t, int32(numNil), failCount.Load(), "nil proofs must fail")
}

// TestVerifyDLNProofSemaphoreReleasedOnNilProof verifies that the semaphore
// slot is released even when the proof is nil.  If it were not released,
// subsequent verifications would deadlock.
func TestVerifyDLNProofSemaphoreReleasedOnNilProof(t *testing.T) {
	// concurrency=1: if the semaphore is not released after nil proof,
	// the second call will block forever.
	dpv := NewDlnProofVerifier(1)

	done := make(chan struct{})
	go func() {
		var wg sync.WaitGroup
		// First: nil proof
		wg.Add(1)
		dpv.VerifyDLNProof(nil, nil, nil, nil, nil, func(bool) {
			wg.Done()
		})
		wg.Wait()

		// Second: also nil — should not deadlock
		wg.Add(1)
		dpv.VerifyDLNProof(nil, nil, nil, nil, nil, func(bool) {
			wg.Done()
		})
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock: semaphore not released after nil proof")
	}
}

// TestVerifyDLNProofSwappedH1H2 verifies that swapping H1 and H2 at
// verification time causes failure.  The proof is for (H1, H2) but we
// verify with (H2, H1).
func TestVerifyDLNProofSwappedH1H2(t *testing.T) {
	params := generateDLNTestParams(t)
	proof := dlnproof.NewDLNProof(
		params.Session, params.H1, params.H2,
		params.Alpha, params.P, params.Q, params.N,
		rand.Reader,
	)
	require.NotNil(t, proof)

	dpv := NewDlnProofVerifier(1)
	var result atomic.Bool
	result.Store(true)
	var wg sync.WaitGroup
	wg.Add(1)
	// Swap H1 and H2
	dpv.VerifyDLNProof(proof, params.Session, params.H2, params.H1, params.N, func(ok bool) {
		result.Store(ok)
		wg.Done()
	})
	wg.Wait()
	assert.False(t, result.Load(), "swapped H1/H2 must cause verification failure")
}

// TestVerifyDLNProofBothProofDirections mirrors the Round 1 pattern where
// two DLN proofs are created: one for (H1, H2, Alpha) and one for
// (H2, H1, Beta).  Both must verify with the correct parameters.
func TestVerifyDLNProofBothProofDirections(t *testing.T) {
	params := generateDLNTestParams(t)

	// DLNProof1: proves knowledge of Alpha such that H2 = H1^Alpha mod N
	proof1 := dlnproof.NewDLNProof(
		params.Session, params.H1, params.H2,
		params.Alpha, params.P, params.Q, params.N,
		rand.Reader,
	)
	// DLNProof2: proves knowledge of Beta such that H1 = H2^Beta mod N
	proof2 := dlnproof.NewDLNProof(
		params.Session, params.H2, params.H1,
		params.Beta, params.P, params.Q, params.N,
		rand.Reader,
	)
	require.NotNil(t, proof1)
	require.NotNil(t, proof2)

	dpv := NewDlnProofVerifier(2)

	var result1, result2 atomic.Bool
	var wg sync.WaitGroup
	wg.Add(2)
	dpv.VerifyDLNProof(proof1, params.Session, params.H1, params.H2, params.N, func(ok bool) {
		result1.Store(ok)
		wg.Done()
	})
	dpv.VerifyDLNProof(proof2, params.Session, params.H2, params.H1, params.N, func(ok bool) {
		result2.Store(ok)
		wg.Done()
	})
	wg.Wait()
	assert.True(t, result1.Load(), "proof1 (H1->H2, Alpha) must verify")
	assert.True(t, result2.Load(), "proof2 (H2->H1, Beta) must verify")
}
