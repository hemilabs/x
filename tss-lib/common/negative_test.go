// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package common

import (
	"math/big"
	"testing"
)

// --- SHA512_256 error paths ---

func TestSHA512_256Empty(t *testing.T) {
	result := SHA512_256()
	if result != nil {
		t.Fatal("empty input should return nil")
	}
}

func TestSHA512_256iOneNilNeg(t *testing.T) {
	result := SHA512_256iOne(nil)
	if result != nil {
		t.Fatal("nil input should return nil")
	}
}

func TestSHA512_256iOneValid(t *testing.T) {
	result := SHA512_256iOne(big.NewInt(42))
	if result == nil || result.Sign() == 0 {
		t.Fatal("valid input should return non-nil, non-zero")
	}
}

// --- MustGetRandomInt panic paths ---

func TestMustGetRandomIntZeroBits(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for zero bits")
		}
	}()
	MustGetRandomInt(nil, 0)
}

func TestMustGetRandomIntNegativeBits(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for negative bits")
		}
	}()
	MustGetRandomInt(nil, -1)
}

// --- GetRandomPrimeInt edge cases ---

func TestGetRandomPrimeIntZeroBits(t *testing.T) {
	result := GetRandomPrimeInt(nil, 0)
	if result != nil {
		t.Fatal("zero bits should return nil")
	}
}

func TestGetRandomPrimeIntNegativeBits(t *testing.T) {
	result := GetRandomPrimeInt(nil, -1)
	if result != nil {
		t.Fatal("negative bits should return nil")
	}
}

// --- GetRandomPositiveRelativelyPrimeInt edge cases ---

func TestGetRandomRelPrimeNilN(t *testing.T) {
	result := GetRandomPositiveRelativelyPrimeInt(nil, nil)
	if result != nil {
		t.Fatal("nil n should return nil")
	}
}

func TestGetRandomRelPrimeZeroN(t *testing.T) {
	result := GetRandomPositiveRelativelyPrimeInt(nil, big.NewInt(0))
	if result != nil {
		t.Fatal("zero n should return nil")
	}
}

// --- IsNumberInMultiplicativeGroup edge cases ---

func TestIsNumberInMultiplicativeGroupNilArgs(t *testing.T) {
	if IsNumberInMultiplicativeGroup(nil, big.NewInt(1)) {
		t.Fatal("nil n should return false")
	}
	if IsNumberInMultiplicativeGroup(big.NewInt(10), nil) {
		t.Fatal("nil v should return false")
	}
	if IsNumberInMultiplicativeGroup(big.NewInt(0), big.NewInt(1)) {
		t.Fatal("zero n should return false")
	}
}

// --- GetRandomBytes edge cases ---

func TestGetRandomBytesZeroLength(t *testing.T) {
	_, err := GetRandomBytes(nil, 0)
	if err == nil {
		t.Fatal("expected error for zero length")
	}
}

func TestGetRandomBytesNegativeLength(t *testing.T) {
	_, err := GetRandomBytes(nil, -1)
	if err == nil {
		t.Fatal("expected error for negative length")
	}
}
