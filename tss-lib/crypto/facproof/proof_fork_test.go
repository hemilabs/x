// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

// Tests for FacProof fork changes: SSID session domain separation,
// V sign-magnitude encoding, N0/NCap BitLen < 2048 rejection.

package facproof

import (
	"context"
	"crypto/rand"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"

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
	if err != nil {
		t.Fatal(err)
	}
	N0 := sk.N
	N0p := sk.P
	N0q := sk.Q

	// Generate Pedersen parameters (NCap, s, t).
	primes := [2]*big.Int{
		common.GetRandomPrimeInt(rand.Reader, 1024),
		common.GetRandomPrimeInt(rand.Reader, 1024),
	}
	NCap, s, tt, err := crypto.GenerateNTildei(rand.Reader, primes)
	if err != nil {
		t.Fatal(err)
	}

	proof, err := NewProof(forkSession, ec, N0, NCap, s, tt, N0p, N0q, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	return proof, N0, NCap, s, tt
}

func TestFacProofForkVerifyHappyPath(t *testing.T) {
	proof, N0, NCap, s, tt := generateFacProofFixture(t)
	if !(proof.Verify(forkSession, tss.EC(), N0, NCap, s, tt)) {
		t.Fatal("expected true")
	}
}

func TestFacProofForkRejectsWrongSession(t *testing.T) {
	proof, N0, NCap, s, tt := generateFacProofFixture(t)
	if !(proof.Verify(forkSession, tss.EC(), N0, NCap, s, tt)) {
		t.Fatal("expected true")
	}
	if proof.Verify([]byte("wrong-session"), tss.EC(), N0, NCap, s, tt) {
		t.Fatal("expected false")
	}
}

func TestFacProofForkVSignMagnitudeRoundTrip(t *testing.T) {
	// [FORK] V can be negative. Test Bytes()/NewProofFromBytes() round-trip.
	proof, N0, NCap, s, tt := generateFacProofFixture(t)

	// Serialize and deserialize.
	bzs := proof.Bytes()
	recovered, err := NewProofFromBytes(bzs[:])
	if err != nil {
		t.Fatal(err)
	}

	// All fields should match.
	if proof.P.Cmp(recovered.P) != 0 {
		t.Fatalf("P mismatch")
	}
	if proof.Q.Cmp(recovered.Q) != 0 {
		t.Fatalf("Q mismatch")
	}
	if proof.V.Cmp(recovered.V) != 0 {
		t.Fatalf("V mismatch (sign-magnitude)")
	}
	if !reflect.DeepEqual(proof.V.Sign(), recovered.V.Sign()) {
		t.Fatalf("V sign mismatch")
	}

	// Recovered proof should still verify.
	if !(recovered.Verify(forkSession, tss.EC(), N0, NCap, s, tt)) {
		t.Fatal("expected true")
	}
}

func TestFacProofForkRejectsSmallN0(t *testing.T) {
	// [FORK] N0.BitLen() < 2048 should be rejected.
	proof, _, NCap, s, tt := generateFacProofFixture(t)
	smallN0 := common.GetRandomPrimeInt(rand.Reader, 512)
	if proof.Verify(forkSession, tss.EC(), smallN0, NCap, s, tt) {
		t.Fatal("expected false")
	}
}

func TestFacProofForkRejectsSmallNCap(t *testing.T) {
	// [FORK] NCap.BitLen() < 2048 should be rejected.
	proof, N0, _, s, tt := generateFacProofFixture(t)
	smallNCap := common.GetRandomPrimeInt(rand.Reader, 512)
	if proof.Verify(forkSession, tss.EC(), N0, smallNCap, s, tt) {
		t.Fatal("expected false")
	}
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
	if err == nil {
		t.Fatal("invalid V sign byte should error")
	}
	if !strings.Contains(err.Error(), "sign byte") {
		t.Fatalf("expected %q to contain %q", err.Error(), "sign byte")
	}
}

func TestFacProofForkFromBytesRejectsNegativeZero(t *testing.T) {
	proof, _, _, _, _ := generateFacProofFixture(t)
	bzs := proof.Bytes()

	// Craft a V field that encodes "negative zero" (sign=0x01, no magnitude bytes = zero).
	bzs[10] = []byte{0x01}

	_, err := NewProofFromBytes(bzs[:])
	if err == nil {
		t.Fatal("negative zero V should error")
	}
}

func TestFacProofForkRejectsNilProof(t *testing.T) {
	var proof *ProofFac
	if proof.Verify(forkSession, tss.EC(), big.NewInt(1), big.NewInt(1), big.NewInt(1), big.NewInt(1)) {
		t.Fatal("expected false")
	}
}

func TestFacProofForkRejectsNilN0(t *testing.T) {
	proof, _, NCap, s, tt := generateFacProofFixture(t)
	if proof.Verify(forkSession, tss.EC(), nil, NCap, s, tt) {
		t.Fatal("nil N0 should be rejected")
	}
}

func TestFacProofForkRejectsNilNCap(t *testing.T) {
	proof, N0, _, s, tt := generateFacProofFixture(t)
	if proof.Verify(forkSession, tss.EC(), N0, nil, s, tt) {
		t.Fatal("nil NCap should be rejected")
	}
}

func TestFacProofForkRejectsNilEC(t *testing.T) {
	proof, N0, NCap, s, tt := generateFacProofFixture(t)
	if proof.Verify(forkSession, nil, N0, NCap, s, tt) {
		t.Fatal("nil EC should be rejected")
	}
}

func TestFacProofForkNilSession(t *testing.T) {
	ec := tss.EC()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	sk, _, err := paillier.GenerateKeyPair(ctx, rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	N0 := sk.N

	primes := [2]*big.Int{
		common.GetRandomPrimeInt(rand.Reader, 1024),
		common.GetRandomPrimeInt(rand.Reader, 1024),
	}
	NCap, s, tt, err := crypto.GenerateNTildei(rand.Reader, primes)
	if err != nil {
		t.Fatal(err)
	}

	// A nil session should produce a valid proof that verifies with nil session
	// but fails with a different session.
	proof, err := NewProof(nil, ec, N0, NCap, s, tt, sk.P, sk.Q, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if !proof.Verify(nil, ec, N0, NCap, s, tt) {
		t.Fatal("nil session proof should verify with nil session")
	}
	if proof.Verify(forkSession, ec, N0, NCap, s, tt) {
		t.Fatal("nil session proof should not verify with non-nil session")
	}
}

func TestFacProofForkRejectsZeroN0(t *testing.T) {
	proof, _, NCap, s, tt := generateFacProofFixture(t)
	if proof.Verify(forkSession, tss.EC(), big.NewInt(0), NCap, s, tt) {
		t.Fatal("zero N0 should be rejected")
	}
}

func TestFacProofForkRejectsNegativeN0(t *testing.T) {
	proof, N0, NCap, s, tt := generateFacProofFixture(t)
	negN0 := new(big.Int).Neg(N0)
	if proof.Verify(forkSession, tss.EC(), negN0, NCap, s, tt) {
		t.Fatal("negative N0 should be rejected")
	}
}
