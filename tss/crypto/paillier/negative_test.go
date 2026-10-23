// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package paillier

import (
	"context"
	"crypto/rand"
	"math/big"
	"testing"
	"time"

	crypto2 "github.com/hemilabs/x/tss/v3/crypto"
	"github.com/hemilabs/x/tss/v3/tss"
)

// --- HomoMult error paths ---

func TestHomoMultNegativeM(t *testing.T) {
	pk := &PublicKey{N: big.NewInt(100)}
	_, err := pk.HomoMult(big.NewInt(-1), big.NewInt(5))
	if err == nil {
		t.Fatal("expected error for negative m")
	}
}

func TestHomoMultMTooLarge(t *testing.T) {
	pk := &PublicKey{N: big.NewInt(100)}
	_, err := pk.HomoMult(big.NewInt(100), big.NewInt(5))
	if err == nil {
		t.Fatal("expected error for m >= N")
	}
}

func TestHomoMultC1Negative(t *testing.T) {
	pk := &PublicKey{N: big.NewInt(100)}
	_, err := pk.HomoMult(big.NewInt(1), big.NewInt(-5))
	if err == nil {
		t.Fatal("expected error for negative c1")
	}
}

func TestHomoMultC1GCDFails(t *testing.T) {
	// c1 shares a factor with N → GCD != 1
	pk := &PublicKey{N: big.NewInt(15)} // 3*5
	N2 := pk.NSquare()
	c1 := big.NewInt(3) // shares factor 3 with N=15
	if c1.Cmp(N2) >= 0 {
		t.Skip("c1 too large")
	}
	_, err := pk.HomoMult(big.NewInt(1), c1)
	if err == nil {
		t.Fatal("expected error for c1 sharing factor with N")
	}
}

// --- HomoAdd error paths ---

func TestHomoAddC1Negative(t *testing.T) {
	pk := &PublicKey{N: big.NewInt(100)}
	_, err := pk.HomoAdd(big.NewInt(-1), big.NewInt(5))
	if err == nil {
		t.Fatal("expected error for negative c1")
	}
}

func TestHomoAddC2Negative(t *testing.T) {
	pk := &PublicKey{N: big.NewInt(100)}
	_, err := pk.HomoAdd(big.NewInt(5), big.NewInt(-1))
	if err == nil {
		t.Fatal("expected error for negative c2")
	}
}

func TestHomoAddC1GCDFails(t *testing.T) {
	pk := &PublicKey{N: big.NewInt(15)}
	_, err := pk.HomoAdd(big.NewInt(3), big.NewInt(1))
	if err == nil {
		t.Fatal("expected error for c1 sharing factor with N")
	}
}

func TestHomoAddC2GCDFails(t *testing.T) {
	pk := &PublicKey{N: big.NewInt(15)}
	_, err := pk.HomoAdd(big.NewInt(1), big.NewInt(3))
	if err == nil {
		t.Fatal("expected error for c2 sharing factor with N")
	}
}

// --- EncryptAndReturnRandomness error path ---

func TestEncryptNegativeMessage(t *testing.T) {
	sk, pk, err := GenerateKeyPair(context.Background(), rand.Reader, 512)
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	_ = sk
	_, _, err = pk.EncryptAndReturnRandomness(rand.Reader, big.NewInt(-1))
	if err == nil {
		t.Fatal("expected error for negative message")
	}
}

// --- Decrypt error path ---

func TestDecryptOutOfRange(t *testing.T) {
	sk, _, err := GenerateKeyPair(context.Background(), rand.Reader, 512)
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	// Pass a value >= N^2
	N2 := sk.NSquare()
	_, err = sk.Decrypt(N2)
	if err == nil {
		t.Fatal("expected error for c >= N^2")
	}
}

// --- Proof/Verify with wrong key ---

func TestVerifyWithWrongKey(t *testing.T) {
	sk1, pk1, err := GenerateKeyPair(context.Background(), rand.Reader, 512)
	if err != nil {
		t.Fatalf("GenerateKeyPair 1: %v", err)
	}
	_, pk2, err := GenerateKeyPair(context.Background(), rand.Reader, 512)
	if err != nil {
		t.Fatalf("GenerateKeyPair 2: %v", err)
	}

	// Need a k and ecdsaPub for Proof
	ec := crypto2.ScalarBaseMult(tss.S256(), big.NewInt(1))
	k := big.NewInt(42)

	proof := sk1.Proof(k, ec)
	// Verify with wrong key — should fail
	ok, err := proof.Verify(pk2.N, k, ec)
	if err == nil && ok {
		t.Fatal("proof from different key should fail verification")
	}
	// Verify with correct key — should pass
	ok, err = proof.Verify(pk1.N, k, ec)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if !ok {
		t.Fatal("proof with correct key should pass")
	}
}

// --- GenerateKeyPair with bad concurrency ---

func TestGenerateKeyPairSmallBits(t *testing.T) {
	// Very small modulus, just verify it doesn't hang.
	sk, pk, err := GenerateKeyPair(context.Background(), rand.Reader, 128)
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	if sk == nil || pk == nil {
		t.Fatal("expected non-nil keys")
	}
}

// TestProofVerifyBadInputsReturns verifies that Verify returns an
// error, and does not hang or panic, for an N of a bit length that is
// not a multiple of 256 and for a nil k.
func TestProofVerifyBadInputsReturns(t *testing.T) {
	var pf Proof
	for i := range pf {
		pf[i] = big.NewInt(1)
	}
	pub := crypto2.ScalarBaseMult(tss.S256(), big.NewInt(1))
	n2049 := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 2048), big.NewInt(1))
	n2048 := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 2047), big.NewInt(1))
	done := make(chan struct{})
	go func() {
		defer close(done)
		if ok, err := pf.Verify(n2049, big.NewInt(1), pub); ok || err == nil {
			t.Errorf("2049-bit N: got %v, %v; want an error", ok, err)
		}
		if ok, err := pf.Verify(n2048, nil, pub); ok || err == nil {
			t.Errorf("nil k: got %v, %v; want an error", ok, err)
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Verify did not return")
	}
}
