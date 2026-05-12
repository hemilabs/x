// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package commitments

import (
	"crypto/rand"
	"math/big"
	"testing"
)

func TestVerifyRejectsEmptyD(t *testing.T) {
	cmt := NewHashCommitment(rand.Reader, big.NewInt(42))
	cmt.D = []*big.Int{}
	if cmt.Verify() {
		t.Fatal("empty D should be rejected")
	}
}

func TestVerifyRejectsSingletonD(t *testing.T) {
	cmt := NewHashCommitment(rand.Reader, big.NewInt(42))
	cmt.D = []*big.Int{big.NewInt(42)}
	if cmt.Verify() {
		t.Fatal("singleton D (missing randomness or secret) should be rejected")
	}
}

func TestVerifyRejectsNilD(t *testing.T) {
	cmt := NewHashCommitment(rand.Reader, big.NewInt(42))
	cmt.D = nil
	if cmt.Verify() {
		t.Fatal("nil D should be rejected")
	}
}

func TestDeCommitRejectsSingletonD(t *testing.T) {
	cmt := NewHashCommitment(rand.Reader, big.NewInt(42))
	cmt.D = []*big.Int{big.NewInt(42)}
	ok, _ := cmt.DeCommit()
	if ok {
		t.Fatal("DeCommit should fail when D has only one element")
	}
}
