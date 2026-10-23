// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package schnorr

import (
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss/v3/common"
	"github.com/hemilabs/x/tss/v3/crypto"
	"github.com/hemilabs/x/tss/v3/tss"
)

var negSession = []byte("negative-test")

func TestNewZKProofNilArgs(t *testing.T) {
	_, err := NewZKProof(negSession, nil, nil, rand.Reader)
	if err == nil {
		t.Fatal("expected error for nil args")
	}
}

func TestZKProofVerifyRejectsWrongSession(t *testing.T) {
	ec := tss.S256()
	x := big.NewInt(42)
	X := crypto.ScalarBaseMult(ec, x)
	pf, err := NewZKProof(negSession, x, X, rand.Reader)
	if err != nil {
		t.Fatalf("NewZKProof: %v", err)
	}
	if pf.Verify([]byte("wrong-session"), X) {
		t.Fatal("wrong session should fail")
	}
}

func TestZKProofVerifyRejectsWrongX(t *testing.T) {
	ec := tss.S256()
	x := big.NewInt(42)
	X := crypto.ScalarBaseMult(ec, x)
	pf, err := NewZKProof(negSession, x, X, rand.Reader)
	if err != nil {
		t.Fatalf("NewZKProof: %v", err)
	}
	wrongX := crypto.ScalarBaseMult(ec, big.NewInt(99))
	if pf.Verify(negSession, wrongX) {
		t.Fatal("wrong X should fail")
	}
}

func TestNewZKVProofNilArgs(t *testing.T) {
	_, err := NewZKVProof(negSession, nil, nil, nil, nil, rand.Reader)
	if err == nil {
		t.Fatal("expected error for nil args")
	}
}

func TestZKVProofVerifyRejectsWrongSession(t *testing.T) {
	ec := tss.S256()
	s := big.NewInt(42)
	l := big.NewInt(7)
	V := crypto.ScalarBaseMult(ec, s)
	R := crypto.ScalarBaseMult(ec, l)
	pf, err := NewZKVProof(negSession, V, R, s, l, rand.Reader)
	if err != nil {
		t.Fatalf("NewZKVProof: %v", err)
	}
	if pf.Verify([]byte("wrong"), V, R) {
		t.Fatal("wrong session should fail")
	}
}

func TestZKVProofVerifyRejectsWrongR(t *testing.T) {
	ec := tss.S256()
	s := big.NewInt(42)
	l := big.NewInt(7)
	V := crypto.ScalarBaseMult(ec, s)
	R := crypto.ScalarBaseMult(ec, l)
	pf, err := NewZKVProof(negSession, V, R, s, l, rand.Reader)
	if err != nil {
		t.Fatalf("NewZKVProof: %v", err)
	}
	wrongR := crypto.ScalarBaseMult(ec, big.NewInt(99))
	if pf.Verify(negSession, V, wrongR) {
		t.Fatal("wrong R should fail")
	}
}

// TestVerifyRejectsIdentityAndSmallOrder verifies that Verify rejects
// the identity statement and an Edwards point of small order.
func TestVerifyRejectsIdentityAndSmallOrder(t *testing.T) {
	ec := tss.Edwards()
	// Without the check, Alpha = T*G proves the identity for any c.
	identity, err := crypto.NewECPoint(ec, big.NewInt(0), big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}
	pf := &ZKProof{Alpha: crypto.ScalarBaseMult(ec, big.NewInt(5)), T: big.NewInt(5)}
	if pf.Verify([]byte("session"), identity) {
		t.Fatal("Verify accepted the identity")
	}
	// (0, -1) has order 2 on Ed25519, so c*X is the identity for an
	// even c.  Search a session that gives an even c; then Alpha = T*G
	// passes the equation without the subgroup check.
	order2, err := crypto.NewECPoint(ec, big.NewInt(0), new(big.Int).Sub(ec.Params().P, big.NewInt(1)))
	if err != nil {
		t.Fatal(err)
	}
	pf = &ZKProof{Alpha: crypto.ScalarBaseMult(ec, big.NewInt(2)), T: big.NewInt(2)}
	g := crypto.NewECPointNoCurveCheck(ec, ec.Params().Gx, ec.Params().Gy)
	for i := 0; i < 64; i++ {
		session := []byte{byte(i)}
		c := common.RejectionSample(ec.Params().N, common.SHA512_256i_TAGGED(session,
			order2.X(), order2.Y(), g.X(), g.Y(), pf.Alpha.X(), pf.Alpha.Y()))
		if c.Bit(0) != 0 {
			continue
		}
		if pf.Verify(session, order2) {
			t.Fatal("Verify accepted a point of small order")
		}
		return
	}
	t.Fatal("no session with an even challenge")
}
