// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package common

import (
	"crypto/rand"
	"math/big"
	"testing"
)

// --- slice.go ---

func TestNonEmptyBytes(t *testing.T) {
	if NonEmptyBytes(nil) {
		t.Fatal("nil should fail")
	}
	if NonEmptyBytes([]byte{}) {
		t.Fatal("empty should fail")
	}
	if !NonEmptyBytes([]byte{0x01}) {
		t.Fatal("non-empty should pass")
	}
}

func TestNonEmptyMultiBytes(t *testing.T) {
	if NonEmptyMultiBytes(nil) {
		t.Fatal("nil should fail")
	}
	if NonEmptyMultiBytes([][]byte{}) {
		t.Fatal("empty slice should fail")
	}
	if NonEmptyMultiBytes([][]byte{{0x01}, nil}) {
		t.Fatal("contains nil should fail")
	}
	if NonEmptyMultiBytes([][]byte{{0x01}, {}}) {
		t.Fatal("contains empty should fail")
	}
	if NonEmptyMultiBytes([][]byte{{0x01}}, 2) {
		t.Fatal("wrong expectLen should fail")
	}
	if !NonEmptyMultiBytes([][]byte{{0x01}, {0x02}}) {
		t.Fatal("valid should pass")
	}
	if !NonEmptyMultiBytes([][]byte{{0x01}, {0x02}}, 2) {
		t.Fatal("valid with expectLen should pass")
	}
}

func TestBigIntsToBytesAndBack(t *testing.T) {
	input := []*big.Int{big.NewInt(42), nil, big.NewInt(99)}
	bzs := BigIntsToBytes(input)
	if len(bzs) != 3 {
		t.Fatalf("want 3, got %d", len(bzs))
	}
	if bzs[1] != nil {
		t.Fatal("nil big.Int should produce nil bytes")
	}

	back := MultiBytesToBigInts(bzs[:1])
	if back[0].Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("round-trip failed: got %v", back[0])
	}
}

func TestPadToLengthBytesInPlaceEdgeCases(t *testing.T) {
	src := []byte{0x01, 0x02}
	padded := PadToLengthBytesInPlace(src, 4)
	if len(padded) != 4 {
		t.Fatalf("want 4, got %d", len(padded))
	}
	if padded[0] != 0 || padded[1] != 0 || padded[2] != 1 || padded[3] != 2 {
		t.Fatalf("padding wrong: %x", padded)
	}

	// Already long enough
	long := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	same := PadToLengthBytesInPlace(long, 3)
	if len(same) != 5 {
		t.Fatal("should not truncate")
	}
}

// --- random.go ---

func TestGetRandomBytes(t *testing.T) {
	bz, err := GetRandomBytes(rand.Reader, 32)
	if err != nil {
		t.Fatal(err)
	}
	if len(bz) != 32 {
		t.Fatalf("want 32, got %d", len(bz))
	}

	_, err = GetRandomBytes(rand.Reader, 0)
	if err == nil {
		t.Fatal("length 0 should fail")
	}
	_, err = GetRandomBytes(rand.Reader, -1)
	if err == nil {
		t.Fatal("negative length should fail")
	}
}

func TestGetRandomGeneratorOfTheQuadraticResidue(t *testing.T) {
	// Use a small safe-prime product: p=5 (q=2), p2=7 (q2=3), N=35
	// This is tiny but exercises the code path.
	n := big.NewInt(35)
	g := GetRandomGeneratorOfTheQuadraticResidue(rand.Reader, n)
	if g == nil {
		t.Fatal("returned nil")
	}
	if g.Sign() <= 0 || g.Cmp(n) >= 0 {
		t.Fatalf("out of range: %v", g)
	}
}

func TestGetRandomQuadraticNonResidue(t *testing.T) {
	n := big.NewInt(35)
	w := GetRandomQuadraticNonResidue(rand.Reader, n)
	if w == nil {
		t.Fatal("returned nil")
	}
	if big.Jacobi(w, n) != -1 {
		t.Fatalf("should have Jacobi -1, got %d", big.Jacobi(w, n))
	}
}

func TestMustGetRandomIntPanicsBadBits(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("should panic on 0 bits")
		}
	}()
	MustGetRandomInt(rand.Reader, 0)
}

func TestGetRandomPrimeInt(t *testing.T) {
	p := GetRandomPrimeInt(rand.Reader, 64)
	if p == nil {
		t.Fatal("returned nil")
	}
	if !p.ProbablyPrime(20) {
		t.Fatal("not prime")
	}
}

func TestIsNumberInMultiplicativeGroup(t *testing.T) {
	n := big.NewInt(15)
	if !IsNumberInMultiplicativeGroup(n, big.NewInt(7)) {
		t.Fatal("7 should be in Z*_15")
	}
	if IsNumberInMultiplicativeGroup(n, big.NewInt(3)) {
		t.Fatal("3 should NOT be in Z*_15 (gcd=3)")
	}
	if IsNumberInMultiplicativeGroup(n, big.NewInt(0)) {
		t.Fatal("0 should NOT be in Z*_15")
	}
	if IsNumberInMultiplicativeGroup(n, big.NewInt(-1)) {
		t.Fatal("negative should NOT be in Z*_15")
	}
}

// --- safe_prime.go ---

func TestGermainSafePrimeAccessors(t *testing.T) {
	// q=11 is prime, p = 2*11+1 = 23 is also prime → safe prime pair
	q := big.NewInt(11)
	p := big.NewInt(23)
	gsp := &GermainSafePrime{q: q, p: p}

	if gsp.Prime().Cmp(q) != 0 {
		t.Fatal("Prime() mismatch")
	}
	if gsp.SafePrime().Cmp(p) != 0 {
		t.Fatal("SafePrime() mismatch")
	}
	if !gsp.Validate() {
		t.Fatal("valid safe prime should pass Validate()")
	}
}

// --- hash.go ---

func TestSHA512_256iOne(t *testing.T) {
	h := SHA512_256iOne(big.NewInt(42))
	if h == nil {
		t.Fatal("returned nil")
	}
	if h.Sign() <= 0 {
		t.Fatal("hash should be positive")
	}

	// Nil input returns nil
	h2 := SHA512_256iOne(nil)
	if h2 != nil {
		t.Fatal("nil input should return nil")
	}
}
