// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package schnorr_test

import (
	"crypto/rand"
	"testing"


	"github.com/hemilabs/x/tss-lib/v3/common"
	"github.com/hemilabs/x/tss-lib/v3/crypto"
	. "github.com/hemilabs/x/tss-lib/v3/crypto/schnorr"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

var Session = []byte("session")

func TestSchnorrProof(t *testing.T) {
	q := tss.EC().Params().N
	u := common.GetRandomPositiveInt(rand.Reader, q)
	uG := crypto.ScalarBaseMult(tss.EC(), u)
	proof, _ := NewZKProof(Session, u, uG, rand.Reader)

	if !proof.Alpha.IsOnCurve() {
		t.Fatal("expected true")
	}
	if proof.Alpha.X() == nil {
		t.Fatal("expected non-zero")
	}
	if proof.Alpha.Y() == nil {
		t.Fatal("expected non-zero")
	}
	if proof.T == nil {
		t.Fatal("expected non-zero")
	}
}

func TestSchnorrProofVerify(t *testing.T) {
	q := tss.EC().Params().N
	u := common.GetRandomPositiveInt(rand.Reader, q)
	X := crypto.ScalarBaseMult(tss.EC(), u)

	proof, _ := NewZKProof(Session, u, X, rand.Reader)
	res := proof.Verify(Session, X)

	if !res {
		t.Fatal("verify result must be true")
	}
}

func TestSchnorrProofVerifyBadX(t *testing.T) {
	q := tss.EC().Params().N
	u := common.GetRandomPositiveInt(rand.Reader, q)
	u2 := common.GetRandomPositiveInt(rand.Reader, q)
	X := crypto.ScalarBaseMult(tss.EC(), u)
	X2 := crypto.ScalarBaseMult(tss.EC(), u2)

	proof, _ := NewZKProof(Session, u2, X2, rand.Reader)
	res := proof.Verify(Session, X)

	if res {
		t.Fatal("verify result must be false")
	}
}

func TestSchnorrVProofVerify(t *testing.T) {
	q := tss.EC().Params().N
	k := common.GetRandomPositiveInt(rand.Reader, q)
	s := common.GetRandomPositiveInt(rand.Reader, q)
	l := common.GetRandomPositiveInt(rand.Reader, q)
	R := crypto.ScalarBaseMult(tss.EC(), k) // k_-1 * G
	Rs := R.ScalarMult(s)
	lG := crypto.ScalarBaseMult(tss.EC(), l)
	V, _ := Rs.Add(lG)

	proof, _ := NewZKVProof(Session, V, R, s, l, rand.Reader)
	res := proof.Verify(Session, V, R)

	if !res {
		t.Fatal("verify result must be true")
	}
}

func TestSchnorrVProofVerifyBadPartialV(t *testing.T) {
	q := tss.EC().Params().N
	k := common.GetRandomPositiveInt(rand.Reader, q)
	s := common.GetRandomPositiveInt(rand.Reader, q)
	l := common.GetRandomPositiveInt(rand.Reader, q)
	R := crypto.ScalarBaseMult(tss.EC(), k) // k_-1 * G
	Rs := R.ScalarMult(s)
	V := Rs

	proof, _ := NewZKVProof(Session, V, R, s, l, rand.Reader)
	res := proof.Verify(Session, V, R)

	if res {
		t.Fatal("verify result must be false")
	}
}

func TestSchnorrVProofVerifyBadS(t *testing.T) {
	q := tss.EC().Params().N
	k := common.GetRandomPositiveInt(rand.Reader, q)
	s := common.GetRandomPositiveInt(rand.Reader, q)
	s2 := common.GetRandomPositiveInt(rand.Reader, q)
	l := common.GetRandomPositiveInt(rand.Reader, q)
	R := crypto.ScalarBaseMult(tss.EC(), k) // k_-1 * G
	Rs := R.ScalarMult(s)
	lG := crypto.ScalarBaseMult(tss.EC(), l)
	V, _ := Rs.Add(lG)

	proof, _ := NewZKVProof(Session, V, R, s2, l, rand.Reader)
	res := proof.Verify(Session, V, R)

	if res {
		t.Fatal("verify result must be false")
	}
}
