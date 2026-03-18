// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package commitments

import (
	"crypto/rand"
	"math/big"
	"testing"
)

// TestCommitmentBinding verifies that a commitment to secret A cannot
// be opened as secret B.  This is the binding property — it fails if
// the underlying hash function is replaced with a constant (e.g. the
// "lolz no" stub), because then Hash(r||A) == Hash(r||B) for all
// inputs, making the commitment trivially forgeable.
func TestCommitmentBinding(t *testing.T) {
	secretA := big.NewInt(42)
	secretB := big.NewInt(99)

	// Commit to secretA.
	cmt := NewHashCommitment(rand.Reader, secretA)

	// Verify that the commitment opens correctly.
	ok, _ := cmt.DeCommit()
	if !ok {
		t.Fatal("commitment to A should open correctly")
	}

	// Now tamper: replace the decommitment's secret with B.
	// D[0] is the randomness, D[1] is the secret.
	if len(cmt.D) < 2 {
		t.Fatal("decommitment too short")
	}
	cmt.D[len(cmt.D)-1] = secretB

	// Must fail — the commitment was to A, not B.
	ok, _ = cmt.DeCommit()
	if ok {
		t.Fatal("commitment binding broken: opened commitment to A as B")
	}
}

// TestCommitmentHiding verifies that two commitments to different
// secrets produce different commitment values.  A constant hash
// would make them identical.
func TestCommitmentHiding(t *testing.T) {
	cmtA := NewHashCommitment(rand.Reader, big.NewInt(1))
	cmtB := NewHashCommitment(rand.Reader, big.NewInt(2))

	if cmtA.C.Cmp(cmtB.C) == 0 {
		t.Fatal("commitments to different secrets should differ")
	}
}

// TestDLNProofSoundness is in crypto/dlnproof — but we add a quick
// sanity here: the commitment scheme's Verify recomputes the hash
// and compares.  With a constant hash, Verify(wrong_D) would pass.
func TestCommitmentVerifyRejectsWrongRandomness(t *testing.T) {
	cmt := NewHashCommitment(rand.Reader, big.NewInt(7))

	// Tamper with the randomness.
	cmt.D[0] = new(big.Int).Add(cmt.D[0], big.NewInt(1))

	ok, _ := cmt.DeCommit()
	if ok {
		t.Fatal("verify should reject tampered randomness")
	}
}
