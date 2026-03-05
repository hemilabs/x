package mta

import (
	"context"
	"crypto/rand"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/crypto"
	"github.com/hemilabs/x/tss-lib/v2/crypto/paillier"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

// generateAliceProofFixture creates an honest RangeProofAlice with fresh Paillier keys
// and Pedersen parameters. Returns the proof and all public verification inputs.
func generateAliceProofFixture(t *testing.T) (*RangeProofAlice, *paillier.PublicKey, *big.Int, *big.Int, *big.Int, *big.Int) {
	q := tss.EC().Params().N
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	sk, pk, err := paillier.GenerateKeyPair(ctx, rand.Reader, testPaillierKeyLength)
	assert.NoError(t, err)
	m := common.GetRandomPositiveInt(rand.Reader, q)
	c, r, err := sk.EncryptAndReturnRandomness(rand.Reader, m)
	assert.NoError(t, err)
	primes := [2]*big.Int{common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits), common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits)}
	NTilde, h1, h2, err := crypto.GenerateNTildei(rand.Reader, primes)
	assert.NoError(t, err)
	proof, err := ProveRangeAlice(Session, tss.EC(), pk, c, NTilde, h1, h2, m, r, rand.Reader)
	assert.NoError(t, err)
	return proof, pk, NTilde, h1, h2, c
}

// generateBobProofFixture creates an honest ProofBob with fresh Paillier keys
// and Pedersen parameters. It follows the MtA protocol: Alice encrypts a, Bob
// computes c2 = c1^b * Enc(betaPrm, cRand) mod N^2, then proves knowledge of
// b and betaPrm. Returns the proof and all public verification inputs.
func generateBobProofFixture(t *testing.T) (*ProofBob, *paillier.PublicKey, *big.Int, *big.Int, *big.Int, *big.Int, *big.Int) {
	q := tss.EC().Params().N
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	_, pk, err := paillier.GenerateKeyPair(ctx, rand.Reader, testPaillierKeyLength)
	assert.NoError(t, err)

	// Alice's ciphertext c1 = Enc(a).
	a := common.GetRandomPositiveInt(rand.Reader, q)
	c1, _, err := pk.EncryptAndReturnRandomness(rand.Reader, a)
	assert.NoError(t, err)

	// Bob's secrets: b (multiplier), betaPrm (additive share).
	b := common.GetRandomPositiveInt(rand.Reader, q)
	betaPrm := common.GetRandomPositiveInt(rand.Reader, q)

	// Bob computes c2 = c1^b * Enc(betaPrm, cRand) mod N^2.
	cBTimesA, err := pk.HomoMult(b, c1)
	assert.NoError(t, err)
	cBetaPrm, cRand, err := pk.EncryptAndReturnRandomness(rand.Reader, betaPrm)
	assert.NoError(t, err)
	c2, err := pk.HomoAdd(cBTimesA, cBetaPrm)
	assert.NoError(t, err)

	primes := [2]*big.Int{common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits), common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits)}
	NTilde, h1, h2, err := crypto.GenerateNTildei(rand.Reader, primes)
	assert.NoError(t, err)

	proof, err := ProveBob(Session, tss.EC(), pk, NTilde, h1, h2, c1, c2, b, betaPrm, cRand, rand.Reader)
	assert.NoError(t, err)

	return proof, pk, NTilde, h1, h2, c1, c2
}

// TestRangeProofAliceRejectsOversizedS2 verifies that the fork's s2 upper bound
// check (s2 <= 2*q^3*NTilde) rejects a tampered proof with s2 just above the bound.
func TestRangeProofAliceRejectsOversizedS2(t *testing.T) {
	pf, pk, NTilde, h1, h2, c := generateAliceProofFixture(t)

	// Sanity: honest proof verifies.
	assert.True(t, pf.Verify(Session, tss.EC(), pk, NTilde, h1, h2, c), "honest proof must verify")

	// Compute the s2 upper bound: 2 * q^3 * NTilde.
	q := tss.EC().Params().N
	q3 := new(big.Int).Mul(q, new(big.Int).Mul(q, q))
	s2Bound := new(big.Int).Lsh(new(big.Int).Mul(q3, NTilde), 1)

	// Tamper: set S2 = s2Bound + 1 (just over the limit).
	pf.S2 = new(big.Int).Add(s2Bound, big.NewInt(1))
	assert.False(t, pf.Verify(Session, tss.EC(), pk, NTilde, h1, h2, c), "proof with oversized S2 must be rejected")
}

// TestRangeProofAliceRejectsWrongSession verifies that a RangeProofAlice
// generated with one session is rejected when verified with a different session.
func TestRangeProofAliceRejectsWrongSession(t *testing.T) {
	pf, pk, NTilde, h1, h2, c := generateAliceProofFixture(t)

	// Sanity: honest proof verifies with the original session.
	assert.True(t, pf.Verify(Session, tss.EC(), pk, NTilde, h1, h2, c), "honest proof must verify")

	// Cross-session: must be rejected.
	wrongSession := []byte("wrong")
	assert.False(t, pf.Verify(wrongSession, tss.EC(), pk, NTilde, h1, h2, c), "proof must be rejected with wrong session")
}

// TestRangeProofAliceRejectsDegeneratePedersen verifies that the fork rejects
// proofs verified against degenerate Pedersen parameters (h1=1 or h2=1).
func TestRangeProofAliceRejectsDegeneratePedersen(t *testing.T) {
	pf, pk, NTilde, h1, h2, c := generateAliceProofFixture(t)

	// Sanity: honest proof verifies.
	assert.True(t, pf.Verify(Session, tss.EC(), pk, NTilde, h1, h2, c), "honest proof must verify")

	// Degenerate h1=1: eliminates binding, proof is unsound.
	assert.False(t, pf.Verify(Session, tss.EC(), pk, NTilde, big.NewInt(1), h2, c), "proof must be rejected with h1=1")

	// Degenerate h2=1: eliminates hiding, proof is unsound.
	assert.False(t, pf.Verify(Session, tss.EC(), pk, NTilde, h1, big.NewInt(1), c), "proof must be rejected with h2=1")
}

// TestRangeProofAliceRejectsSmallNTilde verifies that the fork rejects proofs
// verified with an NTilde smaller than 2048 bits.
func TestRangeProofAliceRejectsSmallNTilde(t *testing.T) {
	pf, pk, NTilde, h1, h2, c := generateAliceProofFixture(t)

	// Sanity: honest proof verifies with a properly-sized NTilde.
	assert.True(t, pf.Verify(Session, tss.EC(), pk, NTilde, h1, h2, c), "honest proof must verify")

	// Generate a small NTilde (~512 bits, well under 2048).
	smallNTilde := common.GetRandomPrimeInt(rand.Reader, 512)
	assert.False(t, pf.Verify(Session, tss.EC(), pk, smallNTilde, h1, h2, c), "proof must be rejected with small NTilde")
}

// TestProofBobRejectsOversizedS2 verifies that the fork's s2 upper bound check
// rejects a tampered ProofBob with s2 just above the bound.
func TestProofBobRejectsOversizedS2(t *testing.T) {
	pf, pk, NTilde, h1, h2, c1, c2 := generateBobProofFixture(t)

	// Sanity: honest proof verifies.
	assert.True(t, pf.Verify(Session, tss.EC(), pk, NTilde, h1, h2, c1, c2), "honest proof must verify")

	// Compute the s2/t2 upper bound: 2 * q^3 * NTilde.
	q := tss.EC().Params().N
	q3 := new(big.Int).Mul(q, new(big.Int).Mul(q, q))
	s2t2Bound := new(big.Int).Lsh(new(big.Int).Mul(q3, NTilde), 1)

	// Tamper: set S2 = s2t2Bound + 1.
	pf.S2 = new(big.Int).Add(s2t2Bound, big.NewInt(1))
	assert.False(t, pf.Verify(Session, tss.EC(), pk, NTilde, h1, h2, c1, c2), "proof with oversized S2 must be rejected")
}

// TestProofBobRejectsOversizedT2 verifies that the fork's t2 upper bound check
// rejects a tampered ProofBob with t2 just above the bound.
func TestProofBobRejectsOversizedT2(t *testing.T) {
	pf, pk, NTilde, h1, h2, c1, c2 := generateBobProofFixture(t)

	// Sanity: honest proof verifies.
	assert.True(t, pf.Verify(Session, tss.EC(), pk, NTilde, h1, h2, c1, c2), "honest proof must verify")

	// Compute the s2/t2 upper bound: 2 * q^3 * NTilde.
	q := tss.EC().Params().N
	q3 := new(big.Int).Mul(q, new(big.Int).Mul(q, q))
	s2t2Bound := new(big.Int).Lsh(new(big.Int).Mul(q3, NTilde), 1)

	// Tamper: set T2 = s2t2Bound + 1.
	pf.T2 = new(big.Int).Add(s2t2Bound, big.NewInt(1))
	assert.False(t, pf.Verify(Session, tss.EC(), pk, NTilde, h1, h2, c1, c2), "proof with oversized T2 must be rejected")
}

// TestProofBobRejectsWrongSession verifies that a ProofBob generated with one
// session tag is rejected when verified with a different session tag.
func TestProofBobRejectsWrongSession(t *testing.T) {
	pf, pk, NTilde, h1, h2, c1, c2 := generateBobProofFixture(t)

	// Sanity: honest proof verifies.
	assert.True(t, pf.Verify(Session, tss.EC(), pk, NTilde, h1, h2, c1, c2), "honest proof must verify")

	// Cross-session: must be rejected.
	wrongSession := []byte("wrong-session")
	assert.False(t, pf.Verify(wrongSession, tss.EC(), pk, NTilde, h1, h2, c1, c2), "proof must be rejected with wrong session")
}

// TestProofBobRejectsDegeneratePedersen verifies that the fork rejects ProofBob
// proofs verified against degenerate Pedersen parameters (h1=1 or h2=1).
func TestProofBobRejectsDegeneratePedersen(t *testing.T) {
	pf, pk, NTilde, h1, h2, c1, c2 := generateBobProofFixture(t)

	// Sanity: honest proof verifies.
	assert.True(t, pf.Verify(Session, tss.EC(), pk, NTilde, h1, h2, c1, c2), "honest proof must verify")

	// Degenerate h1=1: eliminates binding.
	assert.False(t, pf.Verify(Session, tss.EC(), pk, NTilde, big.NewInt(1), h2, c1, c2), "proof must be rejected with h1=1")

	// Degenerate h2=1: eliminates hiding.
	assert.False(t, pf.Verify(Session, tss.EC(), pk, NTilde, h1, big.NewInt(1), c1, c2), "proof must be rejected with h2=1")
}

// TestRangeProofAliceRejectsNonCoprimeC verifies that the fork rejects a
// RangeProofAlice when the ciphertext c shares a factor with N^2 (i.e., c = N).
func TestRangeProofAliceRejectsNonCoprimeC(t *testing.T) {
	pf, pk, NTilde, h1, h2, _ := generateAliceProofFixture(t)

	// Tamper: set c = pk.N (shares a factor with N^2, so GCD(c, N^2) != 1).
	badC := new(big.Int).Set(pk.N)
	assert.False(t, pf.Verify(Session, tss.EC(), pk, NTilde, h1, h2, badC), "proof must be rejected when c shares factor with N")
}

// TestProofBobRejectsNonCoprimeC verifies that the fork rejects a ProofBob
// when c1 shares a factor with N (revealing N's factorization).
func TestProofBobRejectsNonCoprimeC(t *testing.T) {
	pf, pk, NTilde, h1, h2, _, c2 := generateBobProofFixture(t)

	// Tamper: set c1 = pk.N (shares a factor with N, so GCD(c1, N) != 1).
	badC1 := new(big.Int).Set(pk.N)
	assert.False(t, pf.Verify(Session, tss.EC(), pk, NTilde, h1, h2, badC1, c2), "proof must be rejected when c1 shares factor with N")
}
