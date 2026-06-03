// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package common

import (
	"math/big"
	"testing"
)

// ---------------------------------------------------------------------------
// modInt.Add
// ---------------------------------------------------------------------------

func TestModIntAddBasic(t *testing.T) {
	mod := ModInt(big.NewInt(7))
	// 3 + 5 = 8 mod 7 = 1
	result := mod.Add(big.NewInt(3), big.NewInt(5))
	if result.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("Add(3, 5) mod 7 = %s, want 1", result)
	}
}

func TestModIntAddNoWrap(t *testing.T) {
	mod := ModInt(big.NewInt(10))
	// 3 + 4 = 7 mod 10 = 7
	result := mod.Add(big.NewInt(3), big.NewInt(4))
	if result.Cmp(big.NewInt(7)) != 0 {
		t.Fatalf("Add(3, 4) mod 10 = %s, want 7", result)
	}
}

func TestModIntAddWithZero(t *testing.T) {
	mod := ModInt(big.NewInt(7))
	result := mod.Add(big.NewInt(5), big.NewInt(0))
	if result.Cmp(big.NewInt(5)) != 0 {
		t.Fatalf("Add(5, 0) mod 7 = %s, want 5", result)
	}
}

func TestModIntAddNegativeInputs(t *testing.T) {
	mod := ModInt(big.NewInt(7))
	// -3 + 5 = 2 mod 7 = 2
	result := mod.Add(big.NewInt(-3), big.NewInt(5))
	if result.Cmp(big.NewInt(2)) != 0 {
		t.Fatalf("Add(-3, 5) mod 7 = %s, want 2", result)
	}
}

// ---------------------------------------------------------------------------
// modInt.Sub
// ---------------------------------------------------------------------------

func TestModIntSubBasic(t *testing.T) {
	mod := ModInt(big.NewInt(7))
	// 5 - 3 = 2 mod 7 = 2
	result := mod.Sub(big.NewInt(5), big.NewInt(3))
	if result.Cmp(big.NewInt(2)) != 0 {
		t.Fatalf("Sub(5, 3) mod 7 = %s, want 2", result)
	}
}

func TestModIntSubNegativeResult(t *testing.T) {
	mod := ModInt(big.NewInt(7))
	// 3 - 5 = -2 mod 7 = 5 (Go's Mod returns non-negative for positive modulus)
	result := mod.Sub(big.NewInt(3), big.NewInt(5))
	if result.Cmp(big.NewInt(5)) != 0 {
		t.Fatalf("Sub(3, 5) mod 7 = %s, want 5", result)
	}
}

func TestModIntSubSelf(t *testing.T) {
	mod := ModInt(big.NewInt(7))
	result := mod.Sub(big.NewInt(4), big.NewInt(4))
	if result.Cmp(big.NewInt(0)) != 0 {
		t.Fatalf("Sub(4, 4) mod 7 = %s, want 0", result)
	}
}

// ---------------------------------------------------------------------------
// modInt.Mul
// ---------------------------------------------------------------------------

func TestModIntMulBasic(t *testing.T) {
	mod := ModInt(big.NewInt(7))
	// 3 * 5 = 15 mod 7 = 1
	result := mod.Mul(big.NewInt(3), big.NewInt(5))
	if result.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("Mul(3, 5) mod 7 = %s, want 1", result)
	}
}

func TestModIntMulByZero(t *testing.T) {
	mod := ModInt(big.NewInt(7))
	result := mod.Mul(big.NewInt(5), big.NewInt(0))
	if result.Cmp(big.NewInt(0)) != 0 {
		t.Fatalf("Mul(5, 0) mod 7 = %s, want 0", result)
	}
}

func TestModIntMulByOne(t *testing.T) {
	mod := ModInt(big.NewInt(7))
	result := mod.Mul(big.NewInt(5), big.NewInt(1))
	if result.Cmp(big.NewInt(5)) != 0 {
		t.Fatalf("Mul(5, 1) mod 7 = %s, want 5", result)
	}
}

func TestModIntMulLargeValues(t *testing.T) {
	mod := ModInt(big.NewInt(100))
	// 50 * 50 = 2500 mod 100 = 0
	result := mod.Mul(big.NewInt(50), big.NewInt(50))
	if result.Cmp(big.NewInt(0)) != 0 {
		t.Fatalf("Mul(50, 50) mod 100 = %s, want 0", result)
	}
}

// ---------------------------------------------------------------------------
// modInt.Exp
// ---------------------------------------------------------------------------

func TestModIntExpBasic(t *testing.T) {
	mod := ModInt(big.NewInt(7))
	// 2^3 = 8 mod 7 = 1
	result := mod.Exp(big.NewInt(2), big.NewInt(3))
	if result.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("Exp(2, 3) mod 7 = %s, want 1", result)
	}
}

func TestModIntExpZeroExponent(t *testing.T) {
	mod := ModInt(big.NewInt(7))
	// x^0 = 1 mod 7 = 1
	result := mod.Exp(big.NewInt(5), big.NewInt(0))
	if result.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("Exp(5, 0) mod 7 = %s, want 1", result)
	}
}

func TestModIntExpZeroBase(t *testing.T) {
	mod := ModInt(big.NewInt(7))
	// 0^5 = 0 mod 7 = 0
	result := mod.Exp(big.NewInt(0), big.NewInt(5))
	if result.Cmp(big.NewInt(0)) != 0 {
		t.Fatalf("Exp(0, 5) mod 7 = %s, want 0", result)
	}
}

func TestModIntExpLargeValues(t *testing.T) {
	mod := ModInt(big.NewInt(13))
	// 2^10 = 1024 mod 13 = 11 (1024 = 78*13 + 10... let me compute: 78*13=1014, 1024-1014=10)
	result := mod.Exp(big.NewInt(2), big.NewInt(10))
	expected := new(big.Int).Exp(big.NewInt(2), big.NewInt(10), big.NewInt(13))
	if result.Cmp(expected) != 0 {
		t.Fatalf("Exp(2, 10) mod 13 = %s, want %s", result, expected)
	}
}

// TestModIntExpNegativeExponentCoprime documents that since Go 1.12
// (https://github.com/golang/go/issues/25865), big.Int.Exp with a negative
// exponent and non-nil modulus computes the modular inverse raised to |exp|.
// Before Go 1.12, this silently returned 1 (wrong answer, no error).
// This behavior is relied upon by crypto/mta/range_proof.go (minusE pattern).
func TestModIntExpNegativeExponentCoprime(t *testing.T) {
	mod := ModInt(big.NewInt(7))
	result := mod.Exp(big.NewInt(2), big.NewInt(-3))
	// 2^{-3} mod 7 = (2^{-1})^3 mod 7 = 4^3 mod 7 = 64 mod 7 = 1
	if result.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("Exp(2, -3) mod 7 = %s, want 1", result)
	}

	// Verify against manual modular inverse computation.
	inv := mod.ModInverse(big.NewInt(2))
	manual := mod.Exp(inv, big.NewInt(3))
	if result.Cmp(manual) != 0 {
		t.Fatalf("Exp(2, -3) mod 7 = %s, manual (2^-1)^3 = %s — mismatch", result, manual)
	}
}

// TestModIntExpNegativeExponentNonCoprime documents that when the base and
// modulus are NOT coprime, big.Int.Exp returns nil. Since modInt.Exp uses
// `return new(big.Int).Exp(x, y, mi.i())`, it returns the nil from Exp
// directly. Any caller that uses the result without a nil check will PANIC.
//
// This is why the FAC proof Verify() explicitly handles negative V with
// ModInverse + nil check rather than relying on modInt.Exp with a negative
// exponent. The MTA range proof's use of minusE is safe only because
// Paillier ciphertexts are always coprime to NSquare.
func TestModIntExpNegativeExponentNonCoprime(t *testing.T) {
	mod := ModInt(big.NewInt(6))
	// 2 and 6 are not coprime (gcd=2), so 2^{-1} mod 6 does not exist.
	// big.Int.Exp returns nil, and modInt.Exp propagates that nil.
	result := mod.Exp(big.NewInt(2), big.NewInt(-3))

	// modInt.Exp returns nil — any subsequent use will panic.
	if result != nil {
		t.Fatalf("modInt.Exp should return nil for non-coprime base/modulus, got %s", result)
	}

	// Contrast: ModInverse also correctly returns nil for non-invertible input.
	inv := mod.ModInverse(big.NewInt(2))
	if inv != nil {
		t.Fatalf("ModInverse(2) mod 6 should be nil, got %s", inv)
	}

	// Demonstrate the panic: using nil result would crash.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("using nil result of Exp should panic")
		}
	}()
	_ = mod.Mul(mod.Exp(big.NewInt(2), big.NewInt(-3)), big.NewInt(1))
}

// TestBigIntExpNilReturnContract directly tests big.Int.Exp's nil return
// contract to detect if a future Go version changes this behavior.
// If this test fails, all code using modInt.Exp with negative exponents
// must be audited.
func TestBigIntExpNilReturnContract(t *testing.T) {
	z := new(big.Int)
	ret := z.Exp(big.NewInt(2), big.NewInt(-3), big.NewInt(6))

	// big.Int.Exp returns nil when base and modulus are not coprime.
	if ret != nil {
		t.Fatal("big.Int.Exp should return nil for non-coprime base/modulus with negative exponent")
	}

	// z (the receiver) should remain unchanged at its initial value (0).
	if z.Sign() != 0 {
		t.Fatalf("receiver z should remain 0, got %s", z)
	}

	// Positive case: coprime base/modulus returns non-nil.
	z2 := new(big.Int)
	ret2 := z2.Exp(big.NewInt(5), big.NewInt(-3), big.NewInt(7))
	if ret2 == nil {
		t.Fatal("big.Int.Exp should return non-nil for coprime base/modulus")
	}
	if ret2 != z2 {
		t.Fatal("big.Int.Exp should return the receiver on success")
	}
}

// ---------------------------------------------------------------------------
// modInt.ModInverse
// ---------------------------------------------------------------------------

func TestModIntModInverseBasic(t *testing.T) {
	mod := ModInt(big.NewInt(7))
	// 3^{-1} mod 7 = 5 (since 3*5=15=2*7+1)
	result := mod.ModInverse(big.NewInt(3))
	if result == nil {
		t.Fatal("ModInverse(3) mod 7 should not be nil")
	}
	if result.Cmp(big.NewInt(5)) != 0 {
		t.Fatalf("ModInverse(3) mod 7 = %s, want 5", result)
	}
}

func TestModIntModInverseOne(t *testing.T) {
	mod := ModInt(big.NewInt(7))
	// 1^{-1} mod 7 = 1
	result := mod.ModInverse(big.NewInt(1))
	if result.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("ModInverse(1) mod 7 = %s, want 1", result)
	}
}

func TestModIntModInverseNonInvertible(t *testing.T) {
	mod := ModInt(big.NewInt(6))
	// 2 is not invertible mod 6 (gcd(2,6)=2!=1)
	result := mod.ModInverse(big.NewInt(2))
	if result != nil {
		t.Fatalf("ModInverse(2) mod 6 should be nil (non-invertible), got %s", result)
	}
}

func TestModIntModInverseVerify(t *testing.T) {
	mod := ModInt(big.NewInt(13))
	g := big.NewInt(5)
	inv := mod.ModInverse(g)
	if inv == nil {
		t.Fatal("ModInverse(5) mod 13 should not be nil")
	}
	// g * inv mod 13 should equal 1.
	product := mod.Mul(g, inv)
	if product.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("5 * ModInverse(5) mod 13 = %s, want 1", product)
	}
}

// ---------------------------------------------------------------------------
// modInt does not mutate inputs
// ---------------------------------------------------------------------------

func TestModIntDoesNotMutateInputs(t *testing.T) {
	mod := ModInt(big.NewInt(7))
	x := big.NewInt(10)
	y := big.NewInt(3)
	xCopy := new(big.Int).Set(x)
	yCopy := new(big.Int).Set(y)

	mod.Add(x, y)
	if x.Cmp(xCopy) != 0 {
		t.Fatalf("Add mutated x: was %s, now %s", xCopy, x)
	}
	if y.Cmp(yCopy) != 0 {
		t.Fatalf("Add mutated y: was %s, now %s", yCopy, y)
	}

	mod.Sub(x, y)
	if x.Cmp(xCopy) != 0 {
		t.Fatalf("Sub mutated x: was %s, now %s", xCopy, x)
	}

	mod.Mul(x, y)
	if x.Cmp(xCopy) != 0 {
		t.Fatalf("Mul mutated x: was %s, now %s", xCopy, x)
	}
}

// ---------------------------------------------------------------------------
// IsInInterval
// ---------------------------------------------------------------------------

func TestIsInIntervalInRange(t *testing.T) {
	// 5 is in [0, 10)
	if !IsInInterval(big.NewInt(5), big.NewInt(10)) {
		t.Fatal("5 should be in [0, 10)")
	}
}

func TestIsInIntervalZero(t *testing.T) {
	// 0 is in [0, 10) — lower bound is inclusive
	if !IsInInterval(big.NewInt(0), big.NewInt(10)) {
		t.Fatal("0 should be in [0, 10)")
	}
}

func TestIsInIntervalAtBound(t *testing.T) {
	// 10 is NOT in [0, 10) — upper bound is exclusive
	if IsInInterval(big.NewInt(10), big.NewInt(10)) {
		t.Fatal("10 should not be in [0, 10)")
	}
}

func TestIsInIntervalNegative(t *testing.T) {
	// -1 is NOT in [0, 10)
	if IsInInterval(big.NewInt(-1), big.NewInt(10)) {
		t.Fatal("-1 should not be in [0, 10)")
	}
}

func TestIsInIntervalAboveBound(t *testing.T) {
	// 15 is NOT in [0, 10)
	if IsInInterval(big.NewInt(15), big.NewInt(10)) {
		t.Fatal("15 should not be in [0, 10)")
	}
}

func TestIsInIntervalBoundOne(t *testing.T) {
	// Only 0 is in [0, 1)
	if !IsInInterval(big.NewInt(0), big.NewInt(1)) {
		t.Fatal("0 should be in [0, 1)")
	}
	if IsInInterval(big.NewInt(1), big.NewInt(1)) {
		t.Fatal("1 should not be in [0, 1)")
	}
}
