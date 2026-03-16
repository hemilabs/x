// Copyright (c) 2026 Hemi Labs, Inc.
// Tests for FacProof fork changes: SSID session domain separation,
// V sign-magnitude encoding, N0/NCap BitLen < 2048 rejection.

package facproof

import (
	"context"
	"crypto/rand"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/hemilabs/x/tss-lib/v3/common"
	"github.com/hemilabs/x/tss-lib/v3/crypto"
	"github.com/hemilabs/x/tss-lib/v3/crypto/paillier"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

var forkSession = []byte("facproof-fork-test")

// generateFacProofFixture generates a complete facproof fixture with real
// Paillier keys and Pedersen parameters. Returns (proof, N0, NCap, s, t).
func generateFacProofFixture(t *testing.T) (*ProofFac, *big.Int, *big.Int, *big.Int, *big.Int) {
	t.Helper()
	ec := tss.EC()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Generate Paillier keypair for N0.
	sk, _, err := paillier.GenerateKeyPair(ctx, rand.Reader, 2048)
	assert.NoError(t, err)
	N0 := sk.N
	N0p := sk.P
	N0q := sk.Q

	// Generate Pedersen parameters (NCap, s, t).
	primes := [2]*big.Int{
		common.GetRandomPrimeInt(rand.Reader, 1024),
		common.GetRandomPrimeInt(rand.Reader, 1024),
	}
	NCap, s, tt, err := crypto.GenerateNTildei(rand.Reader, primes)
	assert.NoError(t, err)

	proof, err := NewProof(forkSession, ec, N0, NCap, s, tt, N0p, N0q, rand.Reader)
	assert.NoError(t, err)

	return proof, N0, NCap, s, tt
}

func TestFacProofForkVerifyHappyPath(t *testing.T) {
	proof, N0, NCap, s, tt := generateFacProofFixture(t)
	assert.True(t, proof.Verify(forkSession, tss.EC(), N0, NCap, s, tt))
}

func TestFacProofForkRejectsWrongSession(t *testing.T) {
	proof, N0, NCap, s, tt := generateFacProofFixture(t)
	assert.True(t, proof.Verify(forkSession, tss.EC(), N0, NCap, s, tt))
	assert.False(t, proof.Verify([]byte("wrong-session"), tss.EC(), N0, NCap, s, tt))
}

func TestFacProofForkVSignMagnitudeRoundTrip(t *testing.T) {
	// [FORK] V can be negative. Test Bytes()/NewProofFromBytes() round-trip.
	proof, N0, NCap, s, tt := generateFacProofFixture(t)

	// Serialize and deserialize.
	bzs := proof.Bytes()
	recovered, err := NewProofFromBytes(bzs[:])
	assert.NoError(t, err)

	// All fields should match.
	assert.Equal(t, 0, proof.P.Cmp(recovered.P), "P mismatch")
	assert.Equal(t, 0, proof.Q.Cmp(recovered.Q), "Q mismatch")
	assert.Equal(t, 0, proof.V.Cmp(recovered.V), "V mismatch (sign-magnitude)")
	assert.Equal(t, proof.V.Sign(), recovered.V.Sign(), "V sign mismatch")

	// Recovered proof should still verify.
	assert.True(t, recovered.Verify(forkSession, tss.EC(), N0, NCap, s, tt))
}

func TestFacProofForkRejectsSmallN0(t *testing.T) {
	// [FORK] N0.BitLen() < 2048 should be rejected.
	proof, _, NCap, s, tt := generateFacProofFixture(t)
	smallN0 := common.GetRandomPrimeInt(rand.Reader, 512)
	assert.False(t, proof.Verify(forkSession, tss.EC(), smallN0, NCap, s, tt))
}

func TestFacProofForkRejectsSmallNCap(t *testing.T) {
	// [FORK] NCap.BitLen() < 2048 should be rejected.
	proof, N0, _, s, tt := generateFacProofFixture(t)
	smallNCap := common.GetRandomPrimeInt(rand.Reader, 512)
	assert.False(t, proof.Verify(forkSession, tss.EC(), N0, smallNCap, s, tt))
}

func TestFacProofForkFromBytesRejectsInvalidVSign(t *testing.T) {
	proof, _, _, _, _ := generateFacProofFixture(t)
	bzs := proof.Bytes()

	// Tamper the V sign byte to an invalid value.
	original := bzs[10]
	tampered := make([]byte, len(original))
	copy(tampered, original)
	tampered[0] = 0x02 // invalid sign byte (must be 0x00 or 0x01)
	bzs[10] = tampered

	_, err := NewProofFromBytes(bzs[:])
	assert.Error(t, err, "invalid V sign byte should error")
	assert.Contains(t, err.Error(), "sign byte")
}

func TestFacProofForkFromBytesRejectsNegativeZero(t *testing.T) {
	proof, _, _, _, _ := generateFacProofFixture(t)
	bzs := proof.Bytes()

	// Craft a V field that encodes "negative zero" (sign=0x01, no magnitude bytes = zero).
	bzs[10] = []byte{0x01}

	_, err := NewProofFromBytes(bzs[:])
	assert.Error(t, err, "negative zero V should error")
}

func TestFacProofForkRejectsNilProof(t *testing.T) {
	var proof *ProofFac
	assert.False(t, proof.Verify(forkSession, tss.EC(), big.NewInt(1), big.NewInt(1), big.NewInt(1), big.NewInt(1)))
}

func TestFacProofForkRejectsNilN0(t *testing.T) {
	proof, _, NCap, s, tt := generateFacProofFixture(t)
	assert.False(t, proof.Verify(forkSession, tss.EC(), nil, NCap, s, tt), "nil N0 should be rejected")
}

func TestFacProofForkRejectsNilNCap(t *testing.T) {
	proof, N0, _, s, tt := generateFacProofFixture(t)
	assert.False(t, proof.Verify(forkSession, tss.EC(), N0, nil, s, tt), "nil NCap should be rejected")
}

func TestFacProofForkRejectsNilEC(t *testing.T) {
	proof, N0, NCap, s, tt := generateFacProofFixture(t)
	assert.False(t, proof.Verify(forkSession, nil, N0, NCap, s, tt), "nil EC should be rejected")
}

func TestFacProofForkNilSession(t *testing.T) {
	ec := tss.EC()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	sk, _, err := paillier.GenerateKeyPair(ctx, rand.Reader, 2048)
	assert.NoError(t, err)
	N0 := sk.N

	primes := [2]*big.Int{
		common.GetRandomPrimeInt(rand.Reader, 1024),
		common.GetRandomPrimeInt(rand.Reader, 1024),
	}
	NCap, s, tt, err := crypto.GenerateNTildei(rand.Reader, primes)
	assert.NoError(t, err)

	// A nil session should produce a valid proof that verifies with nil session
	// but fails with a different session.
	proof, err := NewProof(nil, ec, N0, NCap, s, tt, sk.P, sk.Q, rand.Reader)
	assert.NoError(t, err)
	assert.True(t, proof.Verify(nil, ec, N0, NCap, s, tt), "nil session proof should verify with nil session")
	assert.False(t, proof.Verify(forkSession, ec, N0, NCap, s, tt), "nil session proof should not verify with non-nil session")
}

func TestFacProofForkRejectsZeroN0(t *testing.T) {
	proof, _, NCap, s, tt := generateFacProofFixture(t)
	assert.False(t, proof.Verify(forkSession, tss.EC(), big.NewInt(0), NCap, s, tt), "zero N0 should be rejected")
}

func TestFacProofForkRejectsNegativeN0(t *testing.T) {
	proof, N0, NCap, s, tt := generateFacProofFixture(t)
	negN0 := new(big.Int).Neg(N0)
	assert.False(t, proof.Verify(forkSession, tss.EC(), negN0, NCap, s, tt), "negative N0 should be rejected")
}
