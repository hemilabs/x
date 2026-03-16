// Copyright (c) 2026 Hemi Labs, Inc.
// Tests for DLN proof fork changes: SSID session domain separation,
// N.BitLen() < 2048 rejection, and consolidated pre-validation.

package dlnproof

import (
	"context"
	"crypto/rand"
	"math/big"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/hemilabs/x/tss-lib/v3/common"
)

var dlnSession = []byte("dln-fork-test")

// generateDLNParams generates proper DLN proof parameters at runtime using
// safe primes. Returns h1, h2, x (discrete log), p, q (Sophie Germain primes),
// N = (2p+1)(2q+1).
func generateDLNParams(t *testing.T) (h1, h2, x, p, q, N *big.Int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	concurrency := runtime.NumCPU()
	if concurrency < 1 {
		concurrency = 1
	}

	// Generate two 1024-bit safe primes: safeP = 2p+1, safeQ = 2q+1
	sgps, err := common.GetRandomSafePrimesConcurrent(ctx, 1024, 2, concurrency, rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate safe primes: %v", err)
	}

	p = sgps[0].Prime() // Sophie Germain prime
	q = sgps[1].Prime() // Sophie Germain prime
	safeP := sgps[0].SafePrime()
	safeQ := sgps[1].SafePrime()
	N = new(big.Int).Mul(safeP, safeQ)

	modN := common.ModInt(N)
	pMulQ := new(big.Int).Mul(p, q)

	// h1 = f^2 mod N (a quadratic residue)
	f := common.GetRandomPositiveRelativelyPrimeInt(rand.Reader, N)
	h1 = modN.Mul(f, f)

	// x = alpha, random coprime to N
	alpha := common.GetRandomPositiveRelativelyPrimeInt(rand.Reader, N)
	x = new(big.Int).Mod(alpha, pMulQ)

	// h2 = h1^x mod N
	h2 = modN.Exp(h1, alpha)

	return
}

func TestDLNProofVerifyHappyPath(t *testing.T) {
	h1, h2, x, p, q, N := generateDLNParams(t)
	proof := NewDLNProof(dlnSession, h1, h2, x, p, q, N, rand.Reader)
	assert.True(t, proof.Verify(dlnSession, h1, h2, N))
}

func TestDLNProofRejectsWrongSession(t *testing.T) {
	h1, h2, x, p, q, N := generateDLNParams(t)
	proof := NewDLNProof(dlnSession, h1, h2, x, p, q, N, rand.Reader)

	assert.True(t, proof.Verify(dlnSession, h1, h2, N))
	assert.False(t, proof.Verify([]byte("wrong-session"), h1, h2, N))
}

func TestDLNProofRejectsSmallN(t *testing.T) {
	h1, h2, x, p, q, N := generateDLNParams(t)
	proof := NewDLNProof(dlnSession, h1, h2, x, p, q, N, rand.Reader)

	// 1024-bit N is below the 2048-bit threshold.
	smallN := new(big.Int).SetBit(new(big.Int), 1023, 1)
	smallN.Add(smallN, big.NewInt(1))
	assert.False(t, proof.Verify(dlnSession, h1, h2, smallN), "small N should be rejected")
}

func TestDLNProofRejectsNilN(t *testing.T) {
	h1, h2, x, p, q, N := generateDLNParams(t)
	proof := NewDLNProof(dlnSession, h1, h2, x, p, q, N, rand.Reader)
	assert.False(t, proof.Verify(dlnSession, h1, h2, nil), "nil N should be rejected")
}

func TestDLNProofRejectsNilProof(t *testing.T) {
	var proof *Proof
	assert.False(t, proof.Verify(dlnSession, big.NewInt(3), big.NewInt(5), big.NewInt(15)))
}

func TestDLNProofRejectsTamperedAlpha(t *testing.T) {
	h1, h2, x, p, q, N := generateDLNParams(t)
	proof := NewDLNProof(dlnSession, h1, h2, x, p, q, N, rand.Reader)

	assert.True(t, proof.Verify(dlnSession, h1, h2, N))

	proof.Alpha[0] = new(big.Int).Add(proof.Alpha[0], big.NewInt(1))
	assert.False(t, proof.Verify(dlnSession, h1, h2, N), "tampered Alpha should fail")
}

func TestDLNProofRejectsEqualH1H2(t *testing.T) {
	h1, _, _, _, _, N := generateDLNParams(t)
	proof := &Proof{}
	assert.False(t, proof.Verify(dlnSession, h1, h1, N))
}

func TestDLNProofRejectsNilAlphaElement(t *testing.T) {
	h1, h2, x, p, q, N := generateDLNParams(t)
	proof := NewDLNProof(dlnSession, h1, h2, x, p, q, N, rand.Reader)

	proof.Alpha[0] = nil
	assert.False(t, proof.Verify(dlnSession, h1, h2, N), "nil Alpha element should be rejected")
}

func TestDLNProofRejectsNilTElement(t *testing.T) {
	h1, h2, x, p, q, N := generateDLNParams(t)
	proof := NewDLNProof(dlnSession, h1, h2, x, p, q, N, rand.Reader)

	proof.T[0] = nil
	assert.False(t, proof.Verify(dlnSession, h1, h2, N), "nil T element should be rejected")
}

func TestDLNProofRejectsZeroN(t *testing.T) {
	h1, h2, x, p, q, N := generateDLNParams(t)
	proof := NewDLNProof(dlnSession, h1, h2, x, p, q, N, rand.Reader)
	assert.False(t, proof.Verify(dlnSession, h1, h2, big.NewInt(0)), "zero N should be rejected")
}

func TestDLNProofRejectsNegativeN(t *testing.T) {
	h1, h2, x, p, q, N := generateDLNParams(t)
	proof := NewDLNProof(dlnSession, h1, h2, x, p, q, N, rand.Reader)
	negN := new(big.Int).Neg(N)
	assert.False(t, proof.Verify(dlnSession, h1, h2, negN), "negative N should be rejected")
}

func TestDLNProofRejectsH1EqualsOne(t *testing.T) {
	_, h2, x, p, q, N := generateDLNParams(t)
	proof := NewDLNProof(dlnSession, big.NewInt(1), h2, x, p, q, N, rand.Reader)
	assert.False(t, proof.Verify(dlnSession, big.NewInt(1), h2, N), "h1 == 1 should be rejected")
}

func TestDLNProofNilSession(t *testing.T) {
	h1, h2, x, p, q, N := generateDLNParams(t)
	proof := NewDLNProof(nil, h1, h2, x, p, q, N, rand.Reader)
	assert.True(t, proof.Verify(nil, h1, h2, N), "nil session proof should verify with nil session")
	assert.False(t, proof.Verify(dlnSession, h1, h2, N), "nil session proof should not verify with non-nil session")
}

func TestDLNProofSerializeRoundTrip(t *testing.T) {
	h1, h2, x, p, q, N := generateDLNParams(t)
	proof := NewDLNProof(dlnSession, h1, h2, x, p, q, N, rand.Reader)

	bzs, err := proof.Serialize()
	assert.NoError(t, err)

	recovered, err := UnmarshalDLNProof(bzs)
	assert.NoError(t, err)

	assert.True(t, recovered.Verify(dlnSession, h1, h2, N), "deserialized proof should verify")
}
