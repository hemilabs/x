// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package facproof_test

import (
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hemilabs/x/tss-lib/v3/common"
	"github.com/hemilabs/x/tss-lib/v3/crypto"
	. "github.com/hemilabs/x/tss-lib/v3/crypto/facproof"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// Using a modulus length of 2048 is recommended in the GG18 spec
const (
	testSafePrimeBits = 1024
)

var Session = []byte("session")

func TestFac(test *testing.T) {
	ec := tss.EC()

	N0p := common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits)
	N0q := common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits)
	N0 := new(big.Int).Mul(N0p, N0q)

	primes := [2]*big.Int{common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits), common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits)}
	NCap, s, t, err := crypto.GenerateNTildei(rand.Reader, primes)
	assert.NoError(test, err)
	proof, err := NewProof(Session, ec, N0, NCap, s, t, N0p, N0q, rand.Reader)
	assert.NoError(test, err)

	ok := proof.Verify(Session, ec, N0, NCap, s, t)
	assert.True(test, ok, "proof must verify")

	N0p = common.GetRandomPrimeInt(rand.Reader, 1024)
	N0q = common.GetRandomPrimeInt(rand.Reader, 1024)
	N0 = new(big.Int).Mul(N0p, N0q)

	proof, err = NewProof(Session, ec, N0, NCap, s, t, N0p, N0q, rand.Reader)
	assert.NoError(test, err)

	ok = proof.Verify(Session, ec, N0, NCap, s, t)
	assert.True(test, ok, "proof must verify")
}

// TestFacProofBytesRoundTrip verifies that Bytes() -> NewProofFromBytes() preserves
// all fields including the sign of V.
func TestFacProofBytesRoundTrip(test *testing.T) {
	ec := tss.EC()

	N0p := common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits)
	N0q := common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits)
	N0 := new(big.Int).Mul(N0p, N0q)

	primes := [2]*big.Int{common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits), common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits)}
	NCap, s, t, err := crypto.GenerateNTildei(rand.Reader, primes)
	assert.NoError(test, err)

	proof, err := NewProof(Session, ec, N0, NCap, s, t, N0p, N0q, rand.Reader)
	assert.NoError(test, err)

	bzs := proof.Bytes()
	recovered, err := NewProofFromBytes(bzs[:])
	assert.NoError(test, err)

	// All fields must match exactly.
	assert.Equal(test, proof.P, recovered.P, "P mismatch")
	assert.Equal(test, proof.Q, recovered.Q, "Q mismatch")
	assert.Equal(test, proof.A, recovered.A, "A mismatch")
	assert.Equal(test, proof.B, recovered.B, "B mismatch")
	assert.Equal(test, proof.T, recovered.T, "T mismatch")
	assert.Equal(test, proof.Sigma, recovered.Sigma, "Sigma mismatch")
	assert.Equal(test, proof.Z1, recovered.Z1, "Z1 mismatch")
	assert.Equal(test, proof.Z2, recovered.Z2, "Z2 mismatch")
	assert.Equal(test, proof.W1, recovered.W1, "W1 mismatch")
	assert.Equal(test, proof.W2, recovered.W2, "W2 mismatch")
	assert.Equal(test, proof.V, recovered.V, "V mismatch")

	// Recovered proof must also verify.
	ok := recovered.Verify(Session, ec, N0, NCap, s, t)
	assert.True(test, ok, "recovered proof must verify")
}

// TestFacProofVSignMagnitudeNegative verifies that a negative V is preserved
// through the sign-magnitude encoding.
func TestFacProofVSignMagnitudeNegative(test *testing.T) {
	proof := &ProofFac{
		P: big.NewInt(1), Q: big.NewInt(2), A: big.NewInt(3),
		B: big.NewInt(4), T: big.NewInt(5), Sigma: big.NewInt(6),
		Z1: big.NewInt(7), Z2: big.NewInt(8),
		W1: big.NewInt(9), W2: big.NewInt(10),
		V: big.NewInt(-42),
	}

	bzs := proof.Bytes()
	recovered, err := NewProofFromBytes(bzs[:])
	assert.NoError(test, err)
	assert.Equal(test, big.NewInt(-42), recovered.V, "negative V must survive round-trip")
	assert.Equal(test, -1, recovered.V.Sign(), "V sign must be negative")
}

// TestFacProofVSignMagnitudePositive verifies that a positive V is preserved.
func TestFacProofVSignMagnitudePositive(test *testing.T) {
	proof := &ProofFac{
		P: big.NewInt(1), Q: big.NewInt(2), A: big.NewInt(3),
		B: big.NewInt(4), T: big.NewInt(5), Sigma: big.NewInt(6),
		Z1: big.NewInt(7), Z2: big.NewInt(8),
		W1: big.NewInt(9), W2: big.NewInt(10),
		V: big.NewInt(42),
	}

	bzs := proof.Bytes()
	recovered, err := NewProofFromBytes(bzs[:])
	assert.NoError(test, err)
	assert.Equal(test, big.NewInt(42), recovered.V, "positive V must survive round-trip")
	assert.Equal(test, 1, recovered.V.Sign(), "V sign must be positive")
}

// TestFacProofVSignMagnitudeZero verifies that zero V is preserved.
func TestFacProofVSignMagnitudeZero(test *testing.T) {
	proof := &ProofFac{
		P: big.NewInt(1), Q: big.NewInt(2), A: big.NewInt(3),
		B: big.NewInt(4), T: big.NewInt(5), Sigma: big.NewInt(6),
		Z1: big.NewInt(7), Z2: big.NewInt(8),
		W1: big.NewInt(9), W2: big.NewInt(10),
		V: big.NewInt(0),
	}

	bzs := proof.Bytes()
	recovered, err := NewProofFromBytes(bzs[:])
	assert.NoError(test, err)
	assert.Equal(test, 0, recovered.V.Sign(), "zero V must survive round-trip")
}

// TestFacProofVSignMagnitudeLargeNegative tests a large negative V value.
func TestFacProofVSignMagnitudeLargeNegative(test *testing.T) {
	largeNeg := new(big.Int).SetBytes(make([]byte, 256)) // 2048-bit
	largeNeg.SetBit(largeNeg, 2047, 1)                   // Set high bit
	largeNeg.Neg(largeNeg)                               // Make negative

	proof := &ProofFac{
		P: big.NewInt(1), Q: big.NewInt(2), A: big.NewInt(3),
		B: big.NewInt(4), T: big.NewInt(5), Sigma: big.NewInt(6),
		Z1: big.NewInt(7), Z2: big.NewInt(8),
		W1: big.NewInt(9), W2: big.NewInt(10),
		V: largeNeg,
	}

	bzs := proof.Bytes()
	recovered, err := NewProofFromBytes(bzs[:])
	assert.NoError(test, err)
	assert.Equal(test, 0, largeNeg.Cmp(recovered.V), "large negative V must survive round-trip")
}

// TestFacProofFromBytesTruncated verifies that truncated input produces an error.
func TestFacProofFromBytesTruncated(test *testing.T) {
	// Only 5 parts instead of 11.
	truncated := make([][]byte, 5)
	for i := range truncated {
		truncated[i] = []byte{0x01}
	}
	_, err := NewProofFromBytes(truncated)
	assert.Error(test, err, "truncated input should error")
}

// TestFacProofFromBytesEmptyV verifies that empty V field produces an error.
func TestFacProofFromBytesEmptyV(test *testing.T) {
	parts := make([][]byte, ProofFacBytesParts)
	for i := range parts {
		parts[i] = []byte{0x01}
	}
	// V field (index 10) is empty.
	parts[10] = []byte{}
	_, err := NewProofFromBytes(parts)
	assert.Error(test, err, "empty V should error")
}

// TestFacProofFromBytesInvalidSignByte verifies that non-canonical sign bytes are rejected.
func TestFacProofFromBytesInvalidSignByte(test *testing.T) {
	for _, badSign := range []byte{0x02, 0x03, 0xFF, 0x80} {
		parts := make([][]byte, ProofFacBytesParts)
		for i := range parts {
			parts[i] = []byte{0x01}
		}
		// V field with invalid sign byte prefix.
		parts[10] = []byte{badSign, 0x2A} // sign=badSign, magnitude=42
		_, err := NewProofFromBytes(parts)
		assert.Error(test, err, "sign byte 0x%02x should be rejected", badSign)
	}
}

// TestFacProofFromBytesNegativeZero verifies that negative-zero encoding is rejected.
func TestFacProofFromBytesNegativeZero(test *testing.T) {
	parts := make([][]byte, ProofFacBytesParts)
	for i := range parts {
		parts[i] = []byte{0x01}
	}
	// V field: sign=negative(0x01), magnitude=empty (zero).
	parts[10] = []byte{0x01}
	_, err := NewProofFromBytes(parts)
	assert.Error(test, err, "negative zero V should be rejected")
}

// TestFacProofFromBytesExtraParts verifies that too many parts are rejected.
func TestFacProofFromBytesExtraParts(test *testing.T) {
	parts := make([][]byte, ProofFacBytesParts+1)
	for i := range parts {
		parts[i] = []byte{0x01}
	}
	_, err := NewProofFromBytes(parts)
	assert.Error(test, err, "extra parts should be rejected")
}

// TestFacProofVerifyNegativeVNoPanic verifies that Verify does not panic
// when V is negative. This is the critical fix for the big.Int.Exp panic.
func TestFacProofVerifyNegativeVNoPanic(test *testing.T) {
	ec := tss.EC()

	primes := [2]*big.Int{
		common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits),
		common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits),
	}
	NCap, s, t, err := crypto.GenerateNTildei(rand.Reader, primes)
	assert.NoError(test, err)

	// Construct a proof with negative V and verify it doesn't panic.
	// The proof won't pass verification (since it's hand-crafted), but
	// it MUST NOT panic.
	proof := &ProofFac{
		P: big.NewInt(1), Q: big.NewInt(2), A: big.NewInt(3),
		B: big.NewInt(4), T: big.NewInt(5), Sigma: big.NewInt(6),
		Z1: big.NewInt(7), Z2: big.NewInt(8),
		W1: big.NewInt(9), W2: big.NewInt(10),
		V: big.NewInt(-42),
	}
	// Must not panic -- this was the critical bug (big.Int.Exp panics
	// with negative exponent + non-nil modulus in Go 1.13+).
	assert.NotPanics(test, func() {
		proof.Verify(Session, ec, big.NewInt(100), NCap, s, t)
	}, "Verify must not panic with negative V")
}

// TestFacProofVerifyNegativeVRealProof generates real proofs until one has
// negative V, then verifies it survives a round-trip and passes verification.
func TestFacProofVerifyNegativeVRealProof(test *testing.T) {
	ec := tss.EC()

	primes := [2]*big.Int{
		common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits),
		common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits),
	}
	NCap, s, t, err := crypto.GenerateNTildei(rand.Reader, primes)
	assert.NoError(test, err)

	// Generate proofs until we find one with negative V, up to 100 attempts.
	foundNegative := false
	for i := 0; i < 100; i++ {
		N0p := common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits)
		N0q := common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits)
		N0 := new(big.Int).Mul(N0p, N0q)

		proof, err := NewProof(Session, ec, N0, NCap, s, t, N0p, N0q, rand.Reader)
		assert.NoError(test, err)

		if proof.V.Sign() < 0 {
			foundNegative = true
			test.Logf("Found negative V at iteration %d: %s", i, proof.V)

			// Must verify directly.
			ok := proof.Verify(Session, ec, N0, NCap, s, t)
			assert.True(test, ok, "proof with negative V must verify")

			// Must survive round-trip.
			bzs := proof.Bytes()
			recovered, err := NewProofFromBytes(bzs[:])
			assert.NoError(test, err)
			assert.Equal(test, proof.V.Sign(), recovered.V.Sign())

			ok = recovered.Verify(Session, ec, N0, NCap, s, t)
			assert.True(test, ok, "recovered proof with negative V must verify")
			break
		}
	}
	if !foundNegative {
		test.Skip("No negative V found in 100 iterations (rare but possible); skipping")
	}
}

// TestFacProofVerifyNegativeVMathCorrectness verifies the mathematical correctness
// of the negative-V fix using known values: t^{-V} mod NCap = (t^{-1})^{|V|} mod NCap.
func TestFacProofVerifyNegativeVMathCorrectness(test *testing.T) {
	NCap := big.NewInt(221) // 13 * 17
	t_ := big.NewInt(5)     // coprime to 221
	V := big.NewInt(-7)

	// Method 1 (old, panics): t^V mod NCap -- cannot use big.Int.Exp
	// Method 2 (new fix): (t^{-1})^{|V|} mod NCap
	tInv := new(big.Int).ModInverse(t_, NCap) // 5^{-1} mod 221
	if tInv == nil {
		test.Fatal("t not invertible mod NCap")
	}
	result := new(big.Int).Exp(tInv, new(big.Int).Abs(V), NCap)

	// Verify independently: t^7 mod NCap, then check (t^7)*(t^{-7}) = 1 mod NCap.
	tPosExp := new(big.Int).Exp(t_, big.NewInt(7), NCap)
	product := new(big.Int).Mul(tPosExp, result)
	product.Mod(product, NCap)
	if product.Cmp(big.NewInt(1)) != 0 {
		test.Fatalf("t^7 * t^{-7} mod NCap = %s, want 1", product)
	}

	test.Logf("t^{-1} mod %d = %s", NCap, tInv)
	test.Logf("(t^{-1})^7 mod %d = %s", NCap, result)
	test.Logf("t^7 * (t^{-1})^7 mod %d = %s (should be 1)", NCap, product)
}

// TestFacProofMultipleRoundTripsVerify generates multiple real proofs, serializes
// and deserializes each, and verifies all still pass. This catches any sign-dependent
// verification failure under real conditions.
func TestFacProofMultipleRoundTripsVerify(test *testing.T) {
	ec := tss.EC()

	primes := [2]*big.Int{
		common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits),
		common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits),
	}
	NCap, s, t, err := crypto.GenerateNTildei(rand.Reader, primes)
	assert.NoError(test, err)

	for i := 0; i < 10; i++ {
		N0p := common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits)
		N0q := common.GetRandomPrimeInt(rand.Reader, testSafePrimeBits)
		N0 := new(big.Int).Mul(N0p, N0q)

		proof, err := NewProof(Session, ec, N0, NCap, s, t, N0p, N0q, rand.Reader)
		assert.NoError(test, err, "iteration %d: NewProof failed", i)

		// Serialize and deserialize.
		bzs := proof.Bytes()
		recovered, err := NewProofFromBytes(bzs[:])
		assert.NoError(test, err, "iteration %d: NewProofFromBytes failed", i)

		// V sign must be preserved.
		assert.Equal(test, proof.V.Sign(), recovered.V.Sign(),
			"iteration %d: V sign changed after round-trip (original=%s, recovered=%s)",
			i, proof.V, recovered.V)

		// Recovered proof must verify.
		ok := recovered.Verify(Session, ec, N0, NCap, s, t)
		assert.True(test, ok, "iteration %d: recovered proof failed verification", i)
	}
}

// TestFacProofBytesVSignGoldenVector verifies the exact sign-magnitude encoding
// of the V field (index 10) for known values: positive, negative, and zero.
func TestFacProofBytesVSignGoldenVector(test *testing.T) {
	makeProof := func(v *big.Int) *ProofFac {
		return &ProofFac{
			P: big.NewInt(1), Q: big.NewInt(2), A: big.NewInt(3),
			B: big.NewInt(4), T: big.NewInt(5), Sigma: big.NewInt(6),
			Z1: big.NewInt(7), Z2: big.NewInt(8),
			W1: big.NewInt(9), W2: big.NewInt(10),
			V: v,
		}
	}

	// V = 42: sign byte 0x00 (positive) + magnitude 0x2a
	bzs42 := makeProof(big.NewInt(42)).Bytes()
	assert.Equal(test, []byte{0x00, 0x2a}, bzs42[10],
		"V=42 should encode as [0x00, 0x2a]")

	// V = -42: sign byte 0x01 (negative) + magnitude 0x2a
	bzsNeg42 := makeProof(big.NewInt(-42)).Bytes()
	assert.Equal(test, []byte{0x01, 0x2a}, bzsNeg42[10],
		"V=-42 should encode as [0x01, 0x2a]")

	// V = 0: sign byte 0x00 (positive) + empty magnitude
	bzs0 := makeProof(big.NewInt(0)).Bytes()
	assert.Equal(test, []byte{0x00}, bzs0[10],
		"V=0 should encode as [0x00]")
}

// TestFacProofFromBytesOldFormatNoSignByte verifies that the old format (raw
// magnitude with no sign prefix) is rejected. A raw byte 0x2a is not a valid
// sign byte (must be 0x00 or 0x01), so NewProofFromBytes should return an error.
func TestFacProofFromBytesOldFormatNoSignByte(test *testing.T) {
	parts := make([][]byte, ProofFacBytesParts)
	for i := range parts {
		parts[i] = []byte{0x01}
	}
	// V field: raw magnitude 0x2a without sign prefix (old format).
	// The first byte 0x2a will be interpreted as the sign byte,
	// which is invalid (not 0x00 or 0x01).
	parts[10] = []byte{0x2a}
	_, err := NewProofFromBytes(parts)
	assert.Error(test, err, "old-format V (no sign byte) should be rejected")
}

// TestFacProofFromBytesNilInput verifies that nil input returns an error.
func TestFacProofFromBytesNilInput(test *testing.T) {
	_, err := NewProofFromBytes(nil)
	assert.Error(test, err, "nil input should return error")
}

// TestFacProofFromBytesWrongPartCount verifies that providing fewer than
// ProofFacBytesParts (11) parts returns an error.
func TestFacProofFromBytesWrongPartCount(test *testing.T) {
	parts := make([][]byte, 10) // one fewer than required (11)
	for i := range parts {
		parts[i] = []byte{0x01}
	}
	_, err := NewProofFromBytes(parts)
	assert.Error(test, err, "10 parts instead of 11 should return error")
}

// TestFacProofValidateBasicNilFields verifies that ValidateBasic returns false
// when any individual field is nil.
func TestFacProofValidateBasicNilFields(test *testing.T) {
	fields := []string{"P", "Q", "A", "B", "T", "Sigma", "Z1", "Z2", "W1", "W2", "V"}
	for i, name := range fields {
		proof := &ProofFac{
			P: big.NewInt(1), Q: big.NewInt(2), A: big.NewInt(3),
			B: big.NewInt(4), T: big.NewInt(5), Sigma: big.NewInt(6),
			Z1: big.NewInt(7), Z2: big.NewInt(8),
			W1: big.NewInt(9), W2: big.NewInt(10),
			V: big.NewInt(11),
		}
		// Set field i to nil using reflection-like approach via index.
		switch i {
		case 0:
			proof.P = nil
		case 1:
			proof.Q = nil
		case 2:
			proof.A = nil
		case 3:
			proof.B = nil
		case 4:
			proof.T = nil
		case 5:
			proof.Sigma = nil
		case 6:
			proof.Z1 = nil
		case 7:
			proof.Z2 = nil
		case 8:
			proof.W1 = nil
		case 9:
			proof.W2 = nil
		case 10:
			proof.V = nil
		}
		assert.False(test, proof.ValidateBasic(),
			"ValidateBasic should return false when %s is nil", name)
	}
}

// TestFacProofValidateBasicAllNonNil verifies that ValidateBasic returns true
// when all fields are non-nil.
func TestFacProofValidateBasicAllNonNil(test *testing.T) {
	proof := &ProofFac{
		P: big.NewInt(1), Q: big.NewInt(2), A: big.NewInt(3),
		B: big.NewInt(4), T: big.NewInt(5), Sigma: big.NewInt(6),
		Z1: big.NewInt(7), Z2: big.NewInt(8),
		W1: big.NewInt(9), W2: big.NewInt(10),
		V: big.NewInt(11),
	}
	assert.True(test, proof.ValidateBasic(), "ValidateBasic should return true for all non-nil fields")
}

// TestFacProofVerifyNonInvertibleT verifies that Verify returns false (not panic)
// when t is not invertible mod NCap and V is negative.
func TestFacProofVerifyNonInvertibleT(test *testing.T) {
	ec := tss.EC()

	// Use NCap = 6 and t = 2, which are not coprime (gcd(2,6)=2).
	NCap := big.NewInt(6)
	s := big.NewInt(1)
	t_ := big.NewInt(2) // not invertible mod 6

	proof := &ProofFac{
		P: big.NewInt(1), Q: big.NewInt(2), A: big.NewInt(3),
		B: big.NewInt(4), T: big.NewInt(5), Sigma: big.NewInt(6),
		Z1: big.NewInt(1), Z2: big.NewInt(1),
		W1: big.NewInt(1), W2: big.NewInt(1),
		V: big.NewInt(-42), // negative V to trigger the tInv path
	}

	// Must not panic — should return false because t is not invertible mod NCap.
	assert.NotPanics(test, func() {
		result := proof.Verify(Session, ec, big.NewInt(100), NCap, s, t_)
		assert.False(test, result, "Verify should return false for non-invertible t")
	})
}
