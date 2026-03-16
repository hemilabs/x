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
