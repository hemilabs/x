// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package mta_test

import (
	"context"
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss-lib/v3/common"
	"github.com/hemilabs/x/tss-lib/v3/crypto"
	"github.com/hemilabs/x/tss-lib/v3/crypto/mta"
	"github.com/hemilabs/x/tss-lib/v3/crypto/paillier"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

const testPaillierKeyLength = 2048

// testSafePrimeBits uses smaller primes for test speed.
const testSafePrimeBits = 1024

var testSession = []byte("test-mta-session")

// testSetup generates the two Paillier keypairs + DLN parameters
// needed by the MtA protocol.  Slow (~5s) due to safe prime gen.
type testParams struct {
	ec                func() *big.Int // returns curve order
	pkA, pkB          *paillier.PublicKey
	skA, skB          *paillier.PrivateKey
	NTildeA, h1A, h2A *big.Int
	NTildeB, h1B, h2B *big.Int
}

func setup(t *testing.T) *testParams {
	t.Helper()
	ec := tss.S256()

	skA, pkA, err := paillier.GenerateKeyPair(context.Background(),
		rand.Reader, testSafePrimeBits*2)
	if err != nil {
		t.Fatalf("GenerateKeyPair(A): %v", err)
	}
	skB, pkB, err := paillier.GenerateKeyPair(context.Background(),
		rand.Reader, testSafePrimeBits*2)
	if err != nil {
		t.Fatalf("GenerateKeyPair(B): %v", err)
	}

	primesA := [2]*big.Int{
		common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits),
		common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits),
	}
	NTildeA, h1A, h2A, err := crypto.GenerateNTildei(rand.Reader, primesA)
	if err != nil {
		t.Fatalf("GenerateNTildei(A): %v", err)
	}

	primesB := [2]*big.Int{
		common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits),
		common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits),
	}
	NTildeB, h1B, h2B, err := crypto.GenerateNTildei(rand.Reader, primesB)
	if err != nil {
		t.Fatalf("GenerateNTildei(B): %v", err)
	}

	return &testParams{
		ec:  func() *big.Int { return ec.Params().N },
		pkA: pkA, skA: skA,
		pkB: pkB, skB: skB,
		NTildeA: NTildeA, h1A: h1A, h2A: h2A,
		NTildeB: NTildeB, h1B: h1B, h2B: h2B,
	}
}

// TestMtAFullProtocol runs the complete Alice→Bob→Alice MtA without
// witness check: AliceInit → BobMid → AliceEnd.
func TestMtAFullProtocol(t *testing.T) {
	p := setup(t)
	ec := tss.S256()
	q := ec.Params().N

	a := common.GetRandomPositiveInt(rand.Reader, q)
	b := common.GetRandomPositiveInt(rand.Reader, q)

	aliceSession := append(testSession, byte(0))
	bobSession := append(testSession, byte(1))

	// Alice step 1: encrypt a, produce range proof
	cA, pfA, err := mta.AliceInit(aliceSession, ec, p.pkA, a,
		p.NTildeB, p.h1B, p.h2B, rand.Reader)
	if err != nil {
		t.Fatalf("AliceInit: %v", err)
	}
	if cA == nil || pfA == nil {
		t.Fatal("AliceInit returned nil")
	}

	// Bob step: multiply cA by b, add blinding, produce Bob proof
	beta, cB, betaPrm, piB, err := mta.BobMid(aliceSession, bobSession,
		ec, p.pkA, pfA, b, cA,
		p.NTildeA, p.h1A, p.h2A,
		p.NTildeB, p.h1B, p.h2B, rand.Reader)
	if err != nil {
		t.Fatalf("BobMid: %v", err)
	}
	if beta == nil || cB == nil || betaPrm == nil || piB == nil {
		t.Fatal("BobMid returned nil")
	}

	// Alice step 2: decrypt cB to get alpha
	alpha, err := mta.AliceEnd(bobSession, ec, p.pkA, piB,
		p.h1A, p.h2A, cA, cB, p.NTildeA, p.skA)
	if err != nil {
		t.Fatalf("AliceEnd: %v", err)
	}

	// Verify: alpha + beta ≡ a*b (mod q)
	sum := new(big.Int).Add(alpha, beta)
	sum.Mod(sum, q)
	product := new(big.Int).Mul(a, b)
	product.Mod(product, q)
	if sum.Cmp(product) != 0 {
		t.Fatalf("MtA failed: alpha+beta != a*b mod q\n  sum=%x\n  product=%x", sum, product)
	}
	t.Logf("MtA OK: alpha+beta ≡ a*b (mod q)")
}

// TestMtAWithWitnessCheck runs AliceInit → BobMidWC → AliceEndWC.
func TestMtAWithWitnessCheck(t *testing.T) {
	p := setup(t)
	ec := tss.S256()
	q := ec.Params().N

	a := common.GetRandomPositiveInt(rand.Reader, q)
	b := common.GetRandomPositiveInt(rand.Reader, q)
	B := crypto.ScalarBaseMult(ec, b) // witness: b*G

	aliceSession := append(testSession, byte(0))
	bobSession := append(testSession, byte(1))

	cA, pfA, err := mta.AliceInit(aliceSession, ec, p.pkA, a,
		p.NTildeB, p.h1B, p.h2B, rand.Reader)
	if err != nil {
		t.Fatalf("AliceInit: %v", err)
	}

	beta, cB, betaPrm, piB, err := mta.BobMidWC(aliceSession, bobSession,
		ec, p.pkA, pfA, b, cA,
		p.NTildeA, p.h1A, p.h2A,
		p.NTildeB, p.h1B, p.h2B,
		B, rand.Reader)
	if err != nil {
		t.Fatalf("BobMidWC: %v", err)
	}
	if beta == nil || cB == nil || betaPrm == nil || piB == nil {
		t.Fatal("BobMidWC returned nil")
	}

	alpha, err := mta.AliceEndWC(bobSession, ec, p.pkA, piB, B,
		cA, cB, p.NTildeA, p.h1A, p.h2A, p.skA)
	if err != nil {
		t.Fatalf("AliceEndWC: %v", err)
	}

	sum := new(big.Int).Add(alpha, beta)
	sum.Mod(sum, q)
	product := new(big.Int).Mul(a, b)
	product.Mod(product, q)
	if sum.Cmp(product) != 0 {
		t.Fatalf("MtA(WC) failed: alpha+beta != a*b mod q")
	}
	t.Logf("MtA(WC) OK: alpha+beta ≡ a*b (mod q)")
}

// TestRangeProofAliceRoundTrip tests create → verify → bytes → from bytes.
func TestRangeProofAliceRoundTrip(t *testing.T) {
	p := setup(t)
	ec := tss.S256()
	q := ec.Params().N

	m := common.GetRandomPositiveInt(rand.Reader, q)
	c, r, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, m)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	pf, err := mta.ProveRangeAlice(testSession, ec, p.pkA, c,
		p.NTildeB, p.h1B, p.h2B, m, r, rand.Reader)
	if err != nil {
		t.Fatalf("ProveRangeAlice: %v", err)
	}
	if !pf.Verify(testSession, ec, p.pkA, p.NTildeB, p.h1B, p.h2B, c) {
		t.Fatal("RangeProofAlice verify failed")
	}
	if !pf.ValidateBasic() {
		t.Fatal("ValidateBasic failed")
	}

	// Bytes round-trip
	bzs := pf.Bytes()
	pf2, err := mta.RangeProofAliceFromBytes(bzs[:])
	if err != nil {
		t.Fatalf("FromBytes: %v", err)
	}
	if !pf2.Verify(testSession, ec, p.pkA, p.NTildeB, p.h1B, p.h2B, c) {
		t.Fatal("RangeProofAlice verify after round-trip failed")
	}
	t.Log("RangeProofAlice round-trip OK")
}

// TestProofBobRoundTrip tests create → verify → bytes → from bytes.
func TestProofBobRoundTrip(t *testing.T) {
	p := setup(t)
	ec := tss.S256()
	q := ec.Params().N

	b := common.GetRandomPositiveInt(rand.Reader, q)
	a := common.GetRandomPositiveInt(rand.Reader, q)

	cA, _, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, a)
	if err != nil {
		t.Fatalf("Encrypt(a): %v", err)
	}

	// c2 = cA^b * Enc(beta') homomorphically
	beta := common.GetRandomPositiveInt(rand.Reader, q)
	cBeta, rBeta, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, beta)
	if err != nil {
		t.Fatalf("Encrypt(beta): %v", err)
	}
	// c2 = cA^b * cBeta mod N^2
	N2 := new(big.Int).Mul(p.pkA.N, p.pkA.N)
	c2 := new(big.Int).Exp(cA, b, N2)
	c2.Mul(c2, cBeta)
	c2.Mod(c2, N2)

	pf, err := mta.ProveBob(testSession, ec, p.pkA,
		p.NTildeA, p.h1A, p.h2A, cA, c2, b, beta, rBeta, rand.Reader)
	if err != nil {
		t.Fatalf("ProveBob: %v", err)
	}
	if !pf.Verify(testSession, ec, p.pkA, p.NTildeA, p.h1A, p.h2A, cA, c2) {
		t.Fatal("ProofBob verify failed")
	}
	if !pf.ValidateBasic() {
		t.Fatal("ValidateBasic failed")
	}

	bzs := pf.Bytes()
	pf2, err := mta.ProofBobFromBytes(bzs[:])
	if err != nil {
		t.Fatalf("FromBytes: %v", err)
	}
	if !pf2.Verify(testSession, ec, p.pkA, p.NTildeA, p.h1A, p.h2A, cA, c2) {
		t.Fatal("ProofBob verify after round-trip failed")
	}
	t.Log("ProofBob round-trip OK")
}

// TestAliceInitNilArgs verifies nil argument rejection.
func TestAliceInitNilArgs(t *testing.T) {
	ec := tss.S256()
	_, _, err := mta.AliceInit(testSession, ec, nil, big.NewInt(1),
		big.NewInt(1), big.NewInt(1), big.NewInt(1), rand.Reader)
	if err == nil {
		t.Fatal("expected error for nil pkA")
	}
}

// TestProofBobWCRoundTrip tests create → verify → bytes → from bytes for WC variant.
func TestProofBobWCRoundTrip(t *testing.T) {
	p := setup(t)
	ec := tss.S256()
	q := ec.Params().N

	b := common.GetRandomPositiveInt(rand.Reader, q)
	a := common.GetRandomPositiveInt(rand.Reader, q)
	B := crypto.ScalarBaseMult(ec, b) // witness

	cA, _, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, a)
	if err != nil {
		t.Fatalf("Encrypt(a): %v", err)
	}

	beta := common.GetRandomPositiveInt(rand.Reader, q)
	cBeta, rBeta, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, beta)
	if err != nil {
		t.Fatalf("Encrypt(beta): %v", err)
	}
	N2 := new(big.Int).Mul(p.pkA.N, p.pkA.N)
	c2 := new(big.Int).Exp(cA, b, N2)
	c2.Mul(c2, cBeta)
	c2.Mod(c2, N2)

	pf, err := mta.ProveBobWC(testSession, ec, p.pkA,
		p.NTildeA, p.h1A, p.h2A, cA, c2, b, beta, rBeta, B, rand.Reader)
	if err != nil {
		t.Fatalf("ProveBobWC: %v", err)
	}
	if !pf.Verify(testSession, ec, p.pkA, p.NTildeA, p.h1A, p.h2A, cA, c2, B) {
		t.Fatal("ProofBobWC verify failed")
	}
	if !pf.ValidateBasic() {
		t.Fatal("ValidateBasic failed")
	}

	// Bytes round-trip
	bzs := pf.Bytes()
	pf2, err := mta.ProofBobWCFromBytes(ec, bzs[:])
	if err != nil {
		t.Fatalf("ProofBobWCFromBytes: %v", err)
	}
	if !pf2.Verify(testSession, ec, p.pkA, p.NTildeA, p.h1A, p.h2A, cA, c2, B) {
		t.Fatal("ProofBobWC verify after round-trip failed")
	}
	if !pf2.ValidateBasic() {
		t.Fatal("ValidateBasic after round-trip failed")
	}
	t.Log("ProofBobWC round-trip OK")
}

// TestRangeProofAliceRejectsWrongSession verifies that a RangeProofAlice
// generated with one session tag is rejected when verified with a different
// session tag. This is critical for domain separation: proofs from one
// ceremony must not be replayable in another.
func TestRangeProofAliceRejectsWrongSession(t *testing.T) {
	p := setup(t)
	ec := tss.S256()
	q := ec.Params().N

	m := common.GetRandomPositiveInt(rand.Reader, q)
	c, r, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, m)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	pf, err := mta.ProveRangeAlice(testSession, ec, p.pkA, c,
		p.NTildeB, p.h1B, p.h2B, m, r, rand.Reader)
	if err != nil {
		t.Fatalf("ProveRangeAlice: %v", err)
	}

	// Sanity: honest proof verifies with the original session.
	if !pf.Verify(testSession, ec, p.pkA, p.NTildeB, p.h1B, p.h2B, c) {
		t.Fatal("honest proof must verify with correct session")
	}

	// Cross-session: proof must be rejected with a different session tag.
	wrongSession := []byte("wrong-session")
	if pf.Verify(wrongSession, ec, p.pkA, p.NTildeB, p.h1B, p.h2B, c) {
		t.Fatal("RangeProofAlice must be rejected with wrong session")
	}
	t.Log("RangeProofAlice correctly rejected with wrong session")
}

// TestProofBobRejectsWrongSession verifies that a ProofBob generated with one
// session tag is rejected when verified with a different session tag. This is
// critical for domain separation: proofs from one ceremony must not be
// replayable in another.
func TestProofBobRejectsWrongSession(t *testing.T) {
	p := setup(t)
	ec := tss.S256()
	q := ec.Params().N

	b := common.GetRandomPositiveInt(rand.Reader, q)
	a := common.GetRandomPositiveInt(rand.Reader, q)

	cA, _, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, a)
	if err != nil {
		t.Fatalf("Encrypt(a): %v", err)
	}

	// c2 = cA^b * Enc(beta') homomorphically
	beta := common.GetRandomPositiveInt(rand.Reader, q)
	cBeta, rBeta, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, beta)
	if err != nil {
		t.Fatalf("Encrypt(beta): %v", err)
	}
	N2 := new(big.Int).Mul(p.pkA.N, p.pkA.N)
	c2 := new(big.Int).Exp(cA, b, N2)
	c2.Mul(c2, cBeta)
	c2.Mod(c2, N2)

	pf, err := mta.ProveBob(testSession, ec, p.pkA,
		p.NTildeA, p.h1A, p.h2A, cA, c2, b, beta, rBeta, rand.Reader)
	if err != nil {
		t.Fatalf("ProveBob: %v", err)
	}

	// Sanity: honest proof verifies with the original session.
	if !pf.Verify(testSession, ec, p.pkA, p.NTildeA, p.h1A, p.h2A, cA, c2) {
		t.Fatal("honest proof must verify with correct session")
	}

	// Cross-session: proof must be rejected with a different session tag.
	wrongSession := []byte("wrong-session")
	if pf.Verify(wrongSession, ec, p.pkA, p.NTildeA, p.h1A, p.h2A, cA, c2) {
		t.Fatal("ProofBob must be rejected with wrong session")
	}
	t.Log("ProofBob correctly rejected with wrong session")
}

// ---------------------------------------------------------------------------
// Category 1: Negative tests using external package (adversarial Verify params)
// ---------------------------------------------------------------------------

// TestRangeProofAliceRejectsDegeneratePedersen verifies that the fork rejects
// RangeProofAlice when the verifier is given degenerate Pedersen parameters
// (h1=1 or h2=1), which eliminate binding or hiding respectively.
func TestRangeProofAliceRejectsDegeneratePedersen(t *testing.T) {
	p := setup(t)
	ec := tss.S256()
	q := ec.Params().N

	m := common.GetRandomPositiveInt(rand.Reader, q)
	c, r, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, m)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	pf, err := mta.ProveRangeAlice(testSession, ec, p.pkA, c,
		p.NTildeB, p.h1B, p.h2B, m, r, rand.Reader)
	if err != nil {
		t.Fatalf("ProveRangeAlice: %v", err)
	}

	// Sanity: honest proof verifies.
	if !pf.Verify(testSession, ec, p.pkA, p.NTildeB, p.h1B, p.h2B, c) {
		t.Fatal("honest proof must verify")
	}

	// Degenerate h1=1.
	if pf.Verify(testSession, ec, p.pkA, p.NTildeB, big.NewInt(1), p.h2B, c) {
		t.Fatal("RangeProofAlice must be rejected with h1=1")
	}

	// Degenerate h2=1.
	if pf.Verify(testSession, ec, p.pkA, p.NTildeB, p.h1B, big.NewInt(1), c) {
		t.Fatal("RangeProofAlice must be rejected with h2=1")
	}
	t.Log("RangeProofAlice correctly rejected with degenerate Pedersen params")
}

// TestProofBobRejectsDegeneratePedersen verifies that the fork rejects
// ProofBob when the verifier is given degenerate Pedersen parameters.
func TestProofBobRejectsDegeneratePedersen(t *testing.T) {
	p := setup(t)
	ec := tss.S256()
	q := ec.Params().N

	b := common.GetRandomPositiveInt(rand.Reader, q)
	a := common.GetRandomPositiveInt(rand.Reader, q)

	cA, _, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, a)
	if err != nil {
		t.Fatalf("Encrypt(a): %v", err)
	}

	beta := common.GetRandomPositiveInt(rand.Reader, q)
	cBeta, rBeta, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, beta)
	if err != nil {
		t.Fatalf("Encrypt(beta): %v", err)
	}
	N2 := new(big.Int).Mul(p.pkA.N, p.pkA.N)
	c2 := new(big.Int).Exp(cA, b, N2)
	c2.Mul(c2, cBeta)
	c2.Mod(c2, N2)

	pf, err := mta.ProveBob(testSession, ec, p.pkA,
		p.NTildeA, p.h1A, p.h2A, cA, c2, b, beta, rBeta, rand.Reader)
	if err != nil {
		t.Fatalf("ProveBob: %v", err)
	}

	// Sanity: honest proof verifies.
	if !pf.Verify(testSession, ec, p.pkA, p.NTildeA, p.h1A, p.h2A, cA, c2) {
		t.Fatal("honest proof must verify")
	}

	// Degenerate h1=1.
	if pf.Verify(testSession, ec, p.pkA, p.NTildeA, big.NewInt(1), p.h2A, cA, c2) {
		t.Fatal("ProofBob must be rejected with h1=1")
	}

	// Degenerate h2=1.
	if pf.Verify(testSession, ec, p.pkA, p.NTildeA, p.h1A, big.NewInt(1), cA, c2) {
		t.Fatal("ProofBob must be rejected with h2=1")
	}
	t.Log("ProofBob correctly rejected with degenerate Pedersen params")
}

// TestRangeProofAliceRejectsSmallNTilde verifies that the fork rejects
// RangeProofAlice when verified with a ~512-bit NTilde (below the 2048-bit minimum).
func TestRangeProofAliceRejectsSmallNTilde(t *testing.T) {
	p := setup(t)
	ec := tss.S256()
	q := ec.Params().N

	m := common.GetRandomPositiveInt(rand.Reader, q)
	c, r, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, m)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	pf, err := mta.ProveRangeAlice(testSession, ec, p.pkA, c,
		p.NTildeB, p.h1B, p.h2B, m, r, rand.Reader)
	if err != nil {
		t.Fatalf("ProveRangeAlice: %v", err)
	}

	// Sanity: honest proof verifies.
	if !pf.Verify(testSession, ec, p.pkA, p.NTildeB, p.h1B, p.h2B, c) {
		t.Fatal("honest proof must verify")
	}

	// Small NTilde: product of two 256-bit primes (~512 bits).
	p1 := common.GetRandomPrimeInt(rand.Reader, 256)
	p2 := common.GetRandomPrimeInt(rand.Reader, 256)
	smallNTilde := new(big.Int).Mul(p1, p2)

	if pf.Verify(testSession, ec, p.pkA, smallNTilde, p.h1B, p.h2B, c) {
		t.Fatal("RangeProofAlice must be rejected with small NTilde")
	}
	t.Log("RangeProofAlice correctly rejected with small NTilde")
}

// TestProofBobRejectsSmallNTilde verifies that the fork rejects ProofBob
// when verified with a ~512-bit NTilde.
func TestProofBobRejectsSmallNTilde(t *testing.T) {
	p := setup(t)
	ec := tss.S256()
	q := ec.Params().N

	b := common.GetRandomPositiveInt(rand.Reader, q)
	a := common.GetRandomPositiveInt(rand.Reader, q)

	cA, _, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, a)
	if err != nil {
		t.Fatalf("Encrypt(a): %v", err)
	}

	beta := common.GetRandomPositiveInt(rand.Reader, q)
	cBeta, rBeta, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, beta)
	if err != nil {
		t.Fatalf("Encrypt(beta): %v", err)
	}
	N2 := new(big.Int).Mul(p.pkA.N, p.pkA.N)
	c2 := new(big.Int).Exp(cA, b, N2)
	c2.Mul(c2, cBeta)
	c2.Mod(c2, N2)

	pf, err := mta.ProveBob(testSession, ec, p.pkA,
		p.NTildeA, p.h1A, p.h2A, cA, c2, b, beta, rBeta, rand.Reader)
	if err != nil {
		t.Fatalf("ProveBob: %v", err)
	}

	// Sanity: honest proof verifies.
	if !pf.Verify(testSession, ec, p.pkA, p.NTildeA, p.h1A, p.h2A, cA, c2) {
		t.Fatal("honest proof must verify")
	}

	// Small NTilde: product of two 256-bit primes (~512 bits).
	p1 := common.GetRandomPrimeInt(rand.Reader, 256)
	p2 := common.GetRandomPrimeInt(rand.Reader, 256)
	smallNTilde := new(big.Int).Mul(p1, p2)

	if pf.Verify(testSession, ec, p.pkA, smallNTilde, p.h1A, p.h2A, cA, c2) {
		t.Fatal("ProofBob must be rejected with small NTilde")
	}
	t.Log("ProofBob correctly rejected with small NTilde")
}

// TestRangeProofAliceRejectsNonCoprimeC verifies that the fork rejects
// RangeProofAlice when the ciphertext c shares a factor with N^2
// (i.e., c = pkA.N). This would reveal N's factorization.
func TestRangeProofAliceRejectsNonCoprimeC(t *testing.T) {
	p := setup(t)
	ec := tss.S256()
	q := ec.Params().N

	m := common.GetRandomPositiveInt(rand.Reader, q)
	c, r, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, m)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	pf, err := mta.ProveRangeAlice(testSession, ec, p.pkA, c,
		p.NTildeB, p.h1B, p.h2B, m, r, rand.Reader)
	if err != nil {
		t.Fatalf("ProveRangeAlice: %v", err)
	}

	// Sanity: honest proof verifies.
	if !pf.Verify(testSession, ec, p.pkA, p.NTildeB, p.h1B, p.h2B, c) {
		t.Fatal("honest proof must verify")
	}

	// Adversarial c = pkA.N (shares factor with N^2).
	badC := new(big.Int).Set(p.pkA.N)
	if pf.Verify(testSession, ec, p.pkA, p.NTildeB, p.h1B, p.h2B, badC) {
		t.Fatal("RangeProofAlice must be rejected when c shares factor with N")
	}
	t.Log("RangeProofAlice correctly rejected with non-coprime c")
}

// TestProofBobRejectsNonCoprimeC1 verifies that the fork rejects ProofBob
// when c1 shares a factor with pkA.N.
func TestProofBobRejectsNonCoprimeC1(t *testing.T) {
	p := setup(t)
	ec := tss.S256()
	q := ec.Params().N

	b := common.GetRandomPositiveInt(rand.Reader, q)
	a := common.GetRandomPositiveInt(rand.Reader, q)

	cA, _, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, a)
	if err != nil {
		t.Fatalf("Encrypt(a): %v", err)
	}

	beta := common.GetRandomPositiveInt(rand.Reader, q)
	cBeta, rBeta, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, beta)
	if err != nil {
		t.Fatalf("Encrypt(beta): %v", err)
	}
	N2 := new(big.Int).Mul(p.pkA.N, p.pkA.N)
	c2 := new(big.Int).Exp(cA, b, N2)
	c2.Mul(c2, cBeta)
	c2.Mod(c2, N2)

	pf, err := mta.ProveBob(testSession, ec, p.pkA,
		p.NTildeA, p.h1A, p.h2A, cA, c2, b, beta, rBeta, rand.Reader)
	if err != nil {
		t.Fatalf("ProveBob: %v", err)
	}

	// Sanity: honest proof verifies.
	if !pf.Verify(testSession, ec, p.pkA, p.NTildeA, p.h1A, p.h2A, cA, c2) {
		t.Fatal("honest proof must verify")
	}

	// Adversarial c1 = pkA.N.
	badC1 := new(big.Int).Set(p.pkA.N)
	if pf.Verify(testSession, ec, p.pkA, p.NTildeA, p.h1A, p.h2A, badC1, c2) {
		t.Fatal("ProofBob must be rejected when c1 shares factor with N")
	}
	t.Log("ProofBob correctly rejected with non-coprime c1")
}

// TestProofBobRejectsNonCoprimeC2 verifies that the fork rejects ProofBob
// when c2 shares a factor with pkA.N.
func TestProofBobRejectsNonCoprimeC2(t *testing.T) {
	p := setup(t)
	ec := tss.S256()
	q := ec.Params().N

	b := common.GetRandomPositiveInt(rand.Reader, q)
	a := common.GetRandomPositiveInt(rand.Reader, q)

	cA, _, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, a)
	if err != nil {
		t.Fatalf("Encrypt(a): %v", err)
	}

	beta := common.GetRandomPositiveInt(rand.Reader, q)
	cBeta, rBeta, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, beta)
	if err != nil {
		t.Fatalf("Encrypt(beta): %v", err)
	}
	N2 := new(big.Int).Mul(p.pkA.N, p.pkA.N)
	c2 := new(big.Int).Exp(cA, b, N2)
	c2.Mul(c2, cBeta)
	c2.Mod(c2, N2)

	pf, err := mta.ProveBob(testSession, ec, p.pkA,
		p.NTildeA, p.h1A, p.h2A, cA, c2, b, beta, rBeta, rand.Reader)
	if err != nil {
		t.Fatalf("ProveBob: %v", err)
	}

	// Sanity: honest proof verifies.
	if !pf.Verify(testSession, ec, p.pkA, p.NTildeA, p.h1A, p.h2A, cA, c2) {
		t.Fatal("honest proof must verify")
	}

	// Adversarial c2 = pkA.N.
	badC2 := new(big.Int).Set(p.pkA.N)
	if pf.Verify(testSession, ec, p.pkA, p.NTildeA, p.h1A, p.h2A, cA, badC2) {
		t.Fatal("ProofBob must be rejected when c2 shares factor with N")
	}
	t.Log("ProofBob correctly rejected with non-coprime c2")
}

// TestRangeProofAliceRejectsSmallPaillierN verifies that the fork rejects
// RangeProofAlice when verified with a small (512-bit) Paillier public key.
func TestRangeProofAliceRejectsSmallPaillierN(t *testing.T) {
	p := setup(t)
	ec := tss.S256()
	q := ec.Params().N

	m := common.GetRandomPositiveInt(rand.Reader, q)
	c, r, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, m)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	pf, err := mta.ProveRangeAlice(testSession, ec, p.pkA, c,
		p.NTildeB, p.h1B, p.h2B, m, r, rand.Reader)
	if err != nil {
		t.Fatalf("ProveRangeAlice: %v", err)
	}

	// Sanity: honest proof verifies.
	if !pf.Verify(testSession, ec, p.pkA, p.NTildeB, p.h1B, p.h2B, c) {
		t.Fatal("honest proof must verify")
	}

	// Generate a small (512-bit) Paillier key.
	_, smallPK, err := paillier.GenerateKeyPair(context.Background(), rand.Reader, 512)
	if err != nil {
		t.Fatalf("GenerateKeyPair(small): %v", err)
	}

	if pf.Verify(testSession, ec, smallPK, p.NTildeB, p.h1B, p.h2B, c) {
		t.Fatal("RangeProofAlice must be rejected with small Paillier N")
	}
	t.Log("RangeProofAlice correctly rejected with small Paillier N")
}

// TestProofBobRejectsSmallPaillierN verifies that the fork rejects ProofBob
// when verified with a small (512-bit) Paillier public key.
func TestProofBobRejectsSmallPaillierN(t *testing.T) {
	p := setup(t)
	ec := tss.S256()
	q := ec.Params().N

	b := common.GetRandomPositiveInt(rand.Reader, q)
	a := common.GetRandomPositiveInt(rand.Reader, q)

	cA, _, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, a)
	if err != nil {
		t.Fatalf("Encrypt(a): %v", err)
	}

	beta := common.GetRandomPositiveInt(rand.Reader, q)
	cBeta, rBeta, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, beta)
	if err != nil {
		t.Fatalf("Encrypt(beta): %v", err)
	}
	N2 := new(big.Int).Mul(p.pkA.N, p.pkA.N)
	c2 := new(big.Int).Exp(cA, b, N2)
	c2.Mul(c2, cBeta)
	c2.Mod(c2, N2)

	pf, err := mta.ProveBob(testSession, ec, p.pkA,
		p.NTildeA, p.h1A, p.h2A, cA, c2, b, beta, rBeta, rand.Reader)
	if err != nil {
		t.Fatalf("ProveBob: %v", err)
	}

	// Sanity: honest proof verifies.
	if !pf.Verify(testSession, ec, p.pkA, p.NTildeA, p.h1A, p.h2A, cA, c2) {
		t.Fatal("honest proof must verify")
	}

	// Generate a small (512-bit) Paillier key.
	_, smallPK, err := paillier.GenerateKeyPair(context.Background(), rand.Reader, 512)
	if err != nil {
		t.Fatalf("GenerateKeyPair(small): %v", err)
	}

	if pf.Verify(testSession, ec, smallPK, p.NTildeA, p.h1A, p.h2A, cA, c2) {
		t.Fatal("ProofBob must be rejected with small Paillier N")
	}
	t.Log("ProofBob correctly rejected with small Paillier N")
}

// ---------------------------------------------------------------------------
// B32: ProofBobWC rejects s1ModQ == 0 (proofs.go:361)
// ---------------------------------------------------------------------------

// TestProofBobWCRejectsS1ModQZero verifies that the fork's s1ModQ=0 guard
// (proofs.go:361) rejects a ProofBobWC whose S1 is set to the curve order q.
// When S1 = q, s1ModQ = q mod q = 0 and the EC scalar multiply would produce
// the identity point, so the check fires first.
func TestProofBobWCRejectsS1ModQZero(t *testing.T) {
	p := setup(t)
	ec := tss.S256()
	q := ec.Params().N

	b := common.GetRandomPositiveInt(rand.Reader, q)
	a := common.GetRandomPositiveInt(rand.Reader, q)
	B := crypto.ScalarBaseMult(ec, b) // witness: b*G

	cA, _, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, a)
	if err != nil {
		t.Fatalf("Encrypt(a): %v", err)
	}

	beta := common.GetRandomPositiveInt(rand.Reader, q)
	cBeta, rBeta, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, beta)
	if err != nil {
		t.Fatalf("Encrypt(beta): %v", err)
	}
	N2 := new(big.Int).Mul(p.pkA.N, p.pkA.N)
	c2 := new(big.Int).Exp(cA, b, N2)
	c2.Mul(c2, cBeta)
	c2.Mod(c2, N2)

	pf, err := mta.ProveBobWC(testSession, ec, p.pkA,
		p.NTildeA, p.h1A, p.h2A, cA, c2, b, beta, rBeta, B, rand.Reader)
	if err != nil {
		t.Fatalf("ProveBobWC: %v", err)
	}

	// Sanity: honest proof verifies.
	if !pf.Verify(testSession, ec, p.pkA, p.NTildeA, p.h1A, p.h2A, cA, c2, B) {
		t.Fatal("honest ProofBobWC must verify")
	}

	// Tamper: set S1 = q so that s1ModQ = q mod q = 0.
	pf.S1 = new(big.Int).Set(q)
	if pf.Verify(testSession, ec, p.pkA, p.NTildeA, p.h1A, p.h2A, cA, c2, B) {
		t.Fatal("ProofBobWC must be rejected when S1 = q (s1ModQ = 0)")
	}
	t.Log("ProofBobWC correctly rejected with S1 = q (B32: s1ModQ == 0)")
}

// ---------------------------------------------------------------------------
// B33: ProofBobWC e == 0 guard (proofs.go:364)
// ---------------------------------------------------------------------------

// TestProofBobWCRejectsEZero documents that the e=0 guard at proofs.go:364
// exists as defense-in-depth. The challenge e is computed as
// RejectionSample(q, SHA512_256i_TAGGED(Session, ...)), which outputs 0 only
// if the hash maps to 0 mod q -- computationally infeasible (requires a
// 256-bit hash preimage). We cannot trigger this via external inputs, so
// this test documents the check and verifies the surrounding code path.
func TestProofBobWCRejectsEZero(t *testing.T) {
	p := setup(t)
	ec := tss.S256()
	q := ec.Params().N

	b := common.GetRandomPositiveInt(rand.Reader, q)
	a := common.GetRandomPositiveInt(rand.Reader, q)
	B := crypto.ScalarBaseMult(ec, b)

	cA, _, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, a)
	if err != nil {
		t.Fatalf("Encrypt(a): %v", err)
	}

	beta := common.GetRandomPositiveInt(rand.Reader, q)
	cBeta, rBeta, err := p.pkA.EncryptAndReturnRandomness(rand.Reader, beta)
	if err != nil {
		t.Fatalf("Encrypt(beta): %v", err)
	}
	N2 := new(big.Int).Mul(p.pkA.N, p.pkA.N)
	c2 := new(big.Int).Exp(cA, b, N2)
	c2.Mul(c2, cBeta)
	c2.Mod(c2, N2)

	pf, err := mta.ProveBobWC(testSession, ec, p.pkA,
		p.NTildeA, p.h1A, p.h2A, cA, c2, b, beta, rBeta, B, rand.Reader)
	if err != nil {
		t.Fatalf("ProveBobWC: %v", err)
	}

	// Sanity: honest proof verifies (the e=0 path is not hit).
	if !pf.Verify(testSession, ec, p.pkA, p.NTildeA, p.h1A, p.h2A, cA, c2, B) {
		t.Fatal("honest ProofBobWC must verify")
	}
	t.Log("ProofBobWC sanity verified (B33: e=0 guard is defense-in-depth, computationally infeasible to trigger)")

	// B33: e=0 requires SHA512_256i_TAGGED to produce 0 mod q, which is a
	// hash preimage problem. Cannot be triggered via external inputs.
	t.Skip("B33: e=0 requires hash preimage, computationally infeasible to trigger externally")
}

// ---------------------------------------------------------------------------
// Category 2: Negative tests (proof field mutation) — migrated from internal
// ---------------------------------------------------------------------------

// aliceProofFixture creates an honest RangeProofAlice with fresh Paillier keys
// and Pedersen parameters. Returns the proof and all public verification inputs.
func aliceProofFixture(t *testing.T) (*mta.RangeProofAlice, *paillier.PublicKey, *big.Int, *big.Int, *big.Int, *big.Int) {
	t.Helper()
	ec := tss.S256()
	q := ec.Params().N

	_, pk, err := paillier.GenerateKeyPair(context.Background(), rand.Reader, testPaillierKeyLength)
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	m := common.GetRandomPositiveInt(rand.Reader, q)
	c, r, err := pk.EncryptAndReturnRandomness(rand.Reader, m)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	primes := [2]*big.Int{
		common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits),
		common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits),
	}
	NTilde, h1, h2, err := crypto.GenerateNTildei(rand.Reader, primes)
	if err != nil {
		t.Fatalf("GenerateNTildei: %v", err)
	}

	proof, err := mta.ProveRangeAlice(testSession, ec, pk, c, NTilde, h1, h2, m, r, rand.Reader)
	if err != nil {
		t.Fatalf("ProveRangeAlice: %v", err)
	}

	return proof, pk, NTilde, h1, h2, c
}

// bobProofFixture creates an honest ProofBob with fresh Paillier keys and
// Pedersen parameters. Returns the proof and all public verification inputs.
func bobProofFixture(t *testing.T) (*mta.ProofBob, *paillier.PublicKey, *big.Int, *big.Int, *big.Int, *big.Int, *big.Int) {
	t.Helper()
	ec := tss.S256()
	q := ec.Params().N

	_, pk, err := paillier.GenerateKeyPair(context.Background(), rand.Reader, testPaillierKeyLength)
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	// Alice's ciphertext c1 = Enc(a).
	a := common.GetRandomPositiveInt(rand.Reader, q)
	c1, _, err := pk.EncryptAndReturnRandomness(rand.Reader, a)
	if err != nil {
		t.Fatalf("Encrypt(a): %v", err)
	}

	// Bob's secrets: b (multiplier), betaPrm (additive share).
	b := common.GetRandomPositiveInt(rand.Reader, q)
	betaPrm := common.GetRandomPositiveInt(rand.Reader, q)

	// Bob computes c2 = c1^b * Enc(betaPrm, cRand) mod N^2.
	cBTimesA, err := pk.HomoMult(b, c1)
	if err != nil {
		t.Fatalf("HomoMult: %v", err)
	}
	cBetaPrm, cRand, err := pk.EncryptAndReturnRandomness(rand.Reader, betaPrm)
	if err != nil {
		t.Fatalf("Encrypt(betaPrm): %v", err)
	}
	c2, err := pk.HomoAdd(cBTimesA, cBetaPrm)
	if err != nil {
		t.Fatalf("HomoAdd: %v", err)
	}

	primes := [2]*big.Int{
		common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits),
		common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits),
	}
	NTilde, h1, h2, err := crypto.GenerateNTildei(rand.Reader, primes)
	if err != nil {
		t.Fatalf("GenerateNTildei: %v", err)
	}

	proof, err := mta.ProveBob(testSession, ec, pk, NTilde, h1, h2, c1, c2, b, betaPrm, cRand, rand.Reader)
	if err != nil {
		t.Fatalf("ProveBob: %v", err)
	}

	return proof, pk, NTilde, h1, h2, c1, c2
}

// TestRangeProofAliceRejectsOversizedS2 verifies that the fork's S2 upper
// bound check (S2 <= 2*q^3*NTilde) rejects a tampered proof with S2 just
// above the bound.
func TestRangeProofAliceRejectsOversizedS2(t *testing.T) {
	pf, pk, NTilde, h1, h2, c := aliceProofFixture(t)
	ec := tss.S256()

	// Sanity: honest proof verifies.
	if !pf.Verify(testSession, ec, pk, NTilde, h1, h2, c) {
		t.Fatal("honest proof must verify")
	}

	// Compute the S2 upper bound: 2 * q^3 * NTilde.
	q := ec.Params().N
	q3 := new(big.Int).Mul(q, new(big.Int).Mul(q, q))
	s2Bound := new(big.Int).Lsh(new(big.Int).Mul(q3, NTilde), 1)

	// Tamper: set S2 = s2Bound + 1 (just over the limit).
	pf.S2 = new(big.Int).Add(s2Bound, big.NewInt(1))
	if pf.Verify(testSession, ec, pk, NTilde, h1, h2, c) {
		t.Fatal("proof with oversized S2 must be rejected")
	}
	t.Log("RangeProofAlice correctly rejected with oversized S2")
}

// TestProofBobRejectsOversizedS2 verifies that the fork's S2 upper bound
// check rejects a tampered ProofBob with S2 just above the bound.
func TestProofBobRejectsOversizedS2(t *testing.T) {
	pf, pk, NTilde, h1, h2, c1, c2 := bobProofFixture(t)
	ec := tss.S256()

	// Sanity: honest proof verifies.
	if !pf.Verify(testSession, ec, pk, NTilde, h1, h2, c1, c2) {
		t.Fatal("honest proof must verify")
	}

	// Compute the S2/T2 upper bound: 2 * q^3 * NTilde.
	q := ec.Params().N
	q3 := new(big.Int).Mul(q, new(big.Int).Mul(q, q))
	s2t2Bound := new(big.Int).Lsh(new(big.Int).Mul(q3, NTilde), 1)

	// Tamper: set S2 = s2t2Bound + 1.
	pf.S2 = new(big.Int).Add(s2t2Bound, big.NewInt(1))
	if pf.Verify(testSession, ec, pk, NTilde, h1, h2, c1, c2) {
		t.Fatal("proof with oversized S2 must be rejected")
	}
	t.Log("ProofBob correctly rejected with oversized S2")
}

// TestProofBobRejectsOversizedT2 verifies that the fork's T2 upper bound
// check rejects a tampered ProofBob with T2 just above the bound.
func TestProofBobRejectsOversizedT2(t *testing.T) {
	pf, pk, NTilde, h1, h2, c1, c2 := bobProofFixture(t)
	ec := tss.S256()

	// Sanity: honest proof verifies.
	if !pf.Verify(testSession, ec, pk, NTilde, h1, h2, c1, c2) {
		t.Fatal("honest proof must verify")
	}

	// Compute the S2/T2 upper bound: 2 * q^3 * NTilde.
	q := ec.Params().N
	q3 := new(big.Int).Mul(q, new(big.Int).Mul(q, q))
	s2t2Bound := new(big.Int).Lsh(new(big.Int).Mul(q3, NTilde), 1)

	// Tamper: set T2 = s2t2Bound + 1.
	pf.T2 = new(big.Int).Add(s2t2Bound, big.NewInt(1))
	if pf.Verify(testSession, ec, pk, NTilde, h1, h2, c1, c2) {
		t.Fatal("proof with oversized T2 must be rejected")
	}
	t.Log("ProofBob correctly rejected with oversized T2")
}

// TestProofBobRejectsZeroS verifies that the fork rejects a ProofBob
// when S is set to zero (degenerate element).
func TestProofBobRejectsZeroS(t *testing.T) {
	pf, pk, NTilde, h1, h2, c1, c2 := bobProofFixture(t)
	ec := tss.S256()

	// Sanity: honest proof verifies.
	if !pf.Verify(testSession, ec, pk, NTilde, h1, h2, c1, c2) {
		t.Fatal("honest proof must verify")
	}

	// Tamper: set S = 0.
	pf.S = big.NewInt(0)
	if pf.Verify(testSession, ec, pk, NTilde, h1, h2, c1, c2) {
		t.Fatal("proof with S=0 must be rejected")
	}
	t.Log("ProofBob correctly rejected with S=0")
}

// TestProofBobRejectsZeroV verifies that the fork rejects a ProofBob
// when V is set to zero (degenerate element).
func TestProofBobRejectsZeroV(t *testing.T) {
	pf, pk, NTilde, h1, h2, c1, c2 := bobProofFixture(t)
	ec := tss.S256()

	// Sanity: honest proof verifies.
	if !pf.Verify(testSession, ec, pk, NTilde, h1, h2, c1, c2) {
		t.Fatal("honest proof must verify")
	}

	// Tamper: set V = 0.
	pf.V = big.NewInt(0)
	if pf.Verify(testSession, ec, pk, NTilde, h1, h2, c1, c2) {
		t.Fatal("proof with V=0 must be rejected")
	}
	t.Log("ProofBob correctly rejected with V=0")
}

// TestProofBobRejectsNonCoprimeS verifies that the fork rejects a ProofBob
// when S shares a factor with pkA.N (GCD(S, N) != 1).
func TestProofBobRejectsNonCoprimeS(t *testing.T) {
	pf, pk, NTilde, h1, h2, c1, c2 := bobProofFixture(t)
	ec := tss.S256()

	// Sanity: honest proof verifies.
	if !pf.Verify(testSession, ec, pk, NTilde, h1, h2, c1, c2) {
		t.Fatal("honest proof must verify")
	}

	// Tamper: set S = pk.N (shares a factor with N, so GCD(S, N) = N != 1).
	pf.S = new(big.Int).Set(pk.N)
	if pf.Verify(testSession, ec, pk, NTilde, h1, h2, c1, c2) {
		t.Fatal("proof with S sharing factor with N must be rejected")
	}
	t.Log("ProofBob correctly rejected with non-coprime S")
}

// TestProofBobRejectsNonCoprimeV verifies that the fork rejects a ProofBob
// when V shares a factor with pkA.N (GCD(V, N) != 1).
func TestProofBobRejectsNonCoprimeV(t *testing.T) {
	pf, pk, NTilde, h1, h2, c1, c2 := bobProofFixture(t)
	ec := tss.S256()

	// Sanity: honest proof verifies.
	if !pf.Verify(testSession, ec, pk, NTilde, h1, h2, c1, c2) {
		t.Fatal("honest proof must verify")
	}

	// Tamper: set V = pk.N (shares a factor with N, so GCD(V, N) = N != 1).
	pf.V = new(big.Int).Set(pk.N)
	if pf.Verify(testSession, ec, pk, NTilde, h1, h2, c1, c2) {
		t.Fatal("proof with V sharing factor with N must be rejected")
	}
	t.Log("ProofBob correctly rejected with non-coprime V")
}
