// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package schnorr

import (
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss-lib/v3/crypto"
	"github.com/hemilabs/x/tss-lib/v3/tss"
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
