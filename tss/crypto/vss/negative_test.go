// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package vss

import (
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss-lib/v3/crypto"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

func TestCreateThresholdTooHigh(t *testing.T) {
	ec := tss.S256()
	ids := []*big.Int{big.NewInt(1), big.NewInt(2)}
	_, _, _, err := Create(ec, 5, big.NewInt(42), ids, rand.Reader)
	if err == nil {
		t.Fatal("expected error for threshold >= len(ids)")
	}
}

func TestCreateZeroSecret(t *testing.T) {
	ec := tss.S256()
	ids := []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)}
	_, _, _, err := Create(ec, 1, big.NewInt(0), ids, rand.Reader)
	if err == nil {
		t.Fatal("expected error for zero secret")
	}
}

func TestCreateNilIDs(t *testing.T) {
	ec := tss.S256()
	_, _, _, err := Create(ec, 1, big.NewInt(42), nil, rand.Reader)
	if err == nil {
		t.Fatal("expected error for nil ids")
	}
}

func TestCreateEmptyIDs(t *testing.T) {
	ec := tss.S256()
	_, _, _, err := Create(ec, 1, big.NewInt(42), []*big.Int{}, rand.Reader)
	if err == nil {
		t.Fatal("expected error for empty ids")
	}
}

func TestVerifyBadShare(t *testing.T) {
	ec := tss.S256()
	ids := []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)}
	vs, shares, _, err := Create(ec, 1, big.NewInt(42), ids, rand.Reader)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	badShare := new(big.Int).Add(shares[0].Share, big.NewInt(1))
	badShareCopy := &Share{Threshold: shares[0].Threshold, ID: shares[0].ID, Share: badShare}
	if ok := badShareCopy.Verify(ec, 1, vs); ok {
		t.Fatal("corrupted share should fail verification")
	}
}

func TestReConstructNotEnoughShares(t *testing.T) {
	ec := tss.S256()
	// threshold=2 means 3 shares needed for reconstruction.
	ids := []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3), big.NewInt(4)}
	_, shares, _, err := Create(ec, 2, big.NewInt(42), ids, rand.Reader)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Only pass 1 share — need 3 (threshold+1).
	_, err = shares[:1].ReConstruct(ec)
	if err == nil {
		t.Fatal("expected error with insufficient shares")
	}
}

func TestVerifyWrongThreshold(t *testing.T) {
	ec := tss.S256()
	ids := []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)}
	vs, shares, _, err := Create(ec, 1, big.NewInt(42), ids, rand.Reader)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if ok := shares[0].Verify(ec, 0, vs); ok {
		t.Fatal("wrong threshold should fail verification")
	}
}

func TestVerifyNilVs(t *testing.T) {
	ec := tss.S256()
	share := &Share{Threshold: 1, ID: big.NewInt(1), Share: big.NewInt(42)}
	if ok := share.Verify(ec, 1, nil); ok {
		t.Fatal("nil vs should fail verification")
	}
}

func TestCreateReconstructSuccess(t *testing.T) {
	ec := tss.S256()
	secret := big.NewInt(12345)
	ids := []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)}
	_, shares, _, err := Create(ec, 1, secret, ids, rand.Reader)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	reconstructed, err := shares[:2].ReConstruct(ec)
	if err != nil {
		t.Fatalf("ReConstruct: %v", err)
	}
	bigP := crypto.ScalarBaseMult(ec, secret)
	bigR := crypto.ScalarBaseMult(ec, reconstructed)
	if !bigP.Equals(bigR) {
		t.Fatal("reconstructed secret doesn't match original")
	}
}
