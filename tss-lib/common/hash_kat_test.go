// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package common

import (
	"encoding/hex"
	"math/big"
	"testing"
)

// TestSHA512_256KnownAnswer pins SHA512_256 output to a known digest.
// If the function is replaced with a stub (e.g. returning a constant),
// this test fails immediately.
//
// The expected value was computed from the production implementation
// and captures the length-prefixed, delimited internal format.
func TestSHA512_256KnownAnswer(t *testing.T) {
	got := hex.EncodeToString(SHA512_256([]byte("abc")))
	const want = "decbe7e7d33a897617c3f6fbe553c3598f786a93a2c4237f3dcdd2c8dd817532"
	if got != want {
		t.Fatalf("SHA512_256(\"abc\"):\n  got:  %s\n  want: %s", got, want)
	}
}

// TestSHA512_256iKnownAnswer pins SHA512_256i to a known digest.
func TestSHA512_256iKnownAnswer(t *testing.T) {
	got := SHA512_256i(big.NewInt(42), big.NewInt(99))
	const want = "3bc659aa5672f076492e3c116fbd036134244f5178b07b8441a149708f929942"
	if hex.EncodeToString(got.Bytes()) != want {
		t.Fatalf("SHA512_256i(42, 99): got %x, want %s", got, want)
	}
}

// TestSHA512_256iOneKnownAnswer pins SHA512_256iOne to a known digest.
func TestSHA512_256iOneKnownAnswer(t *testing.T) {
	got := SHA512_256iOne(big.NewInt(12345))
	const want = "795c165105ac2e5c09cc7a734a82c95b7ce8f870a48e419737150a2eb8c0520b"
	if hex.EncodeToString(got.Bytes()) != want {
		t.Fatalf("SHA512_256iOne(12345): got %x, want %s", got, want)
	}
}

// TestSHA512_256TAGGEDDomainSeparation verifies the tagged hash
// produces a different digest than the untagged variant, and is
// deterministic.
func TestSHA512_256TAGGEDDomainSeparation(t *testing.T) {
	tag := []byte("test-domain")
	input := big.NewInt(77)

	tagged := SHA512_256i_TAGGED(tag, input)
	untagged := SHA512_256i(input)

	if tagged.Cmp(untagged) == 0 {
		t.Fatal("tagged and untagged hash must differ")
	}

	// Deterministic.
	tagged2 := SHA512_256i_TAGGED(tag, input)
	if tagged.Cmp(tagged2) != 0 {
		t.Fatal("tagged hash is not deterministic")
	}
}

// TestSHA512_256Collision verifies distinct inputs produce distinct
// hashes.  A stub returning a constant fails this.
func TestSHA512_256Collision(t *testing.T) {
	a := hex.EncodeToString(SHA512_256([]byte("input one")))
	b := hex.EncodeToString(SHA512_256([]byte("input two")))
	if a == b {
		t.Fatal("distinct inputs must produce distinct hashes")
	}
}
