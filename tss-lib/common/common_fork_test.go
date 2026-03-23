// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

// Tests for fork changes in common utility functions.

package common

import (
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- SHA512_256i nil guard (hash.go:73) ---

func TestSHA512_256iNilInput(t *testing.T) {
	// [FORK] nil big.Int in input should be treated as zero, not panic
	result := SHA512_256i(big.NewInt(1), nil, big.NewInt(3))
	assert.NotNil(t, result, "SHA512_256i should handle nil input without panic")

	// Result should match hashing with explicit zero
	expected := SHA512_256i(big.NewInt(1), big.NewInt(0), big.NewInt(3))
	assert.Equal(t, 0, result.Cmp(expected), "nil should hash as zero")
}

// --- SHA512_256i_TAGGED nil guard ---

func TestSHA512_256iTaggedNilInput(t *testing.T) {
	tag := []byte("test-tag")
	result := SHA512_256i_TAGGED(tag, big.NewInt(1), nil, big.NewInt(3))
	assert.NotNil(t, result, "SHA512_256i_TAGGED should handle nil input without panic")

	expected := SHA512_256i_TAGGED(tag, big.NewInt(1), big.NewInt(0), big.NewInt(3))
	assert.Equal(t, 0, result.Cmp(expected), "nil should hash as zero in tagged variant")
}

// --- RejectionSample no in-place mutation (hash_utils.go:24) ---

func TestRejectionSampleNoMutation(t *testing.T) {
	// [FORK] Upstream mutates eHash in-place. Fork allocates new big.Int.
	q := big.NewInt(97) // small prime for testing
	eHash := big.NewInt(150)
	eHashCopy := new(big.Int).Set(eHash)

	result := RejectionSample(q, eHash)

	// eHash should NOT be modified
	assert.Equal(t, 0, eHash.Cmp(eHashCopy), "RejectionSample must not mutate eHash")
	// Result should be eHash mod q
	expected := new(big.Int).Mod(eHashCopy, q)
	assert.Equal(t, 0, result.Cmp(expected), "result should be eHash mod q")
}

// --- IsInInterval nil guard (int.go:60) ---

func TestIsInIntervalNilB(t *testing.T) {
	assert.False(t, IsInInterval(nil, big.NewInt(10)), "nil b should return false")
}

func TestIsInIntervalNilBound(t *testing.T) {
	assert.False(t, IsInInterval(big.NewInt(5), nil), "nil bound should return false")
}

func TestIsInIntervalBothNil(t *testing.T) {
	assert.False(t, IsInInterval(nil, nil), "both nil should return false")
}

func TestIsInIntervalValid(t *testing.T) {
	assert.True(t, IsInInterval(big.NewInt(5), big.NewInt(10)), "5 in [0, 10) should be true")
	assert.False(t, IsInInterval(big.NewInt(10), big.NewInt(10)), "10 not in [0, 10)")
	assert.False(t, IsInInterval(big.NewInt(-1), big.NewInt(10)), "-1 not in [0, 10)")
}

// --- AppendBigIntToBytesSlice length-prefixed (int.go:75) ---

func TestAppendBigIntToBytesSlice(t *testing.T) {
	// [FORK] Uses length prefix to distinguish zero from absent
	base := []byte{0xAA, 0xBB}

	// Append zero: should get [AA BB 00 00 00 00] (4-byte length prefix, no data)
	result := AppendBigIntToBytesSlice(base, big.NewInt(0))
	assert.Equal(t, 6, len(result), "zero value should append 4-byte length prefix only")
	assert.Equal(t, byte(0xAA), result[0])
	assert.Equal(t, byte(0xBB), result[1])
	// Length should be 0 (big-endian)
	assert.Equal(t, byte(0), result[2])
	assert.Equal(t, byte(0), result[3])
	assert.Equal(t, byte(0), result[4])
	assert.Equal(t, byte(0), result[5])

	// Append non-zero
	result2 := AppendBigIntToBytesSlice(base, big.NewInt(256))
	// 256 = 0x0100, so 2 bytes
	assert.Equal(t, 8, len(result2), "256 should append 4-byte length + 2 data bytes")
	assert.Equal(t, byte(0), result2[2]) // length = 2 big-endian
	assert.Equal(t, byte(0), result2[3])
	assert.Equal(t, byte(0), result2[4])
	assert.Equal(t, byte(2), result2[5])

	// Append nil: should be same as zero
	resultNil := AppendBigIntToBytesSlice(base, nil)
	assert.Equal(t, 6, len(resultNil), "nil should append 4-byte length prefix (zero length)")
}

func TestAppendBigIntToBytesSliceDoesNotMutateBase(t *testing.T) {
	base := []byte{0xAA, 0xBB}
	baseCopy := make([]byte, len(base))
	copy(baseCopy, base)

	_ = AppendBigIntToBytesSlice(base, big.NewInt(42))

	assert.Equal(t, baseCopy, base, "base slice should not be mutated")
}

// --- GetRandomPositiveInt rejects lessThan < 2 (random.go:45) ---

func TestGetRandomPositiveIntRejectsNil(t *testing.T) {
	result := GetRandomPositiveInt(rand.Reader, nil)
	assert.Nil(t, result, "nil lessThan should return nil")
}

func TestGetRandomPositiveIntRejectsZero(t *testing.T) {
	result := GetRandomPositiveInt(rand.Reader, big.NewInt(0))
	assert.Nil(t, result, "lessThan=0 should return nil")
}

func TestGetRandomPositiveIntRejectsOne(t *testing.T) {
	// [FORK] lessThan=1 means interval [1, 1) is empty. Upstream allowed this.
	result := GetRandomPositiveInt(rand.Reader, big.NewInt(1))
	assert.Nil(t, result, "lessThan=1 should return nil (empty interval)")
}

func TestGetRandomPositiveIntAcceptsTwo(t *testing.T) {
	// lessThan=2 -> only valid result is 1
	result := GetRandomPositiveInt(rand.Reader, big.NewInt(2))
	assert.NotNil(t, result)
	assert.Equal(t, 0, result.Cmp(big.NewInt(1)), "only valid positive int less than 2 is 1")
}

// --- PadToLengthBytesInPlace (slice.go:59) ---

func TestPadToLengthBytesInPlace(t *testing.T) {
	// Shorter than target: should be zero-padded on the left
	src := []byte{0x01, 0x02}
	result := PadToLengthBytesInPlace(src, 4)
	assert.Equal(t, []byte{0x00, 0x00, 0x01, 0x02}, result)

	// Already correct length: should return as-is
	src2 := []byte{0x01, 0x02, 0x03, 0x04}
	result2 := PadToLengthBytesInPlace(src2, 4)
	assert.Equal(t, src2, result2)

	// Longer than target: should return as-is (no truncation)
	src3 := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	result3 := PadToLengthBytesInPlace(src3, 4)
	assert.Equal(t, src3, result3)

	// Empty source
	result4 := PadToLengthBytesInPlace([]byte{}, 3)
	assert.Equal(t, []byte{0x00, 0x00, 0x00}, result4)
}
