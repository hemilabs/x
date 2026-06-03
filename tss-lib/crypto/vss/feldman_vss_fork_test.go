// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package vss

import (
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss-lib/v3/common"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

func makeValidShares(t *testing.T) (Vs, Shares) {
	t.Helper()
	ec := tss.S256()
	q := ec.Params().N
	secret := common.GetRandomPositiveInt(rand.Reader, q)
	indexes := []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)}
	vs, shares, _, err := Create(ec, 1, secret, indexes, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return vs, shares
}

func TestVerifyRejectsZeroShare(t *testing.T) {
	vs, shares := makeValidShares(t)
	share := *shares[0]
	share.Share = big.NewInt(0)
	if share.Verify(tss.S256(), 1, vs) {
		t.Fatal("zero share should be rejected")
	}
}

func TestVerifyRejectsNegativeShare(t *testing.T) {
	vs, shares := makeValidShares(t)
	share := *shares[0]
	share.Share = big.NewInt(-1)
	if share.Verify(tss.S256(), 1, vs) {
		t.Fatal("negative share should be rejected")
	}
}

func TestVerifyRejectsOutOfRangeShare(t *testing.T) {
	vs, shares := makeValidShares(t)
	q := tss.S256().Params().N
	share := *shares[0]
	share.Share = new(big.Int).Set(q)
	if share.Verify(tss.S256(), 1, vs) {
		t.Fatal("share >= q should be rejected")
	}
}

func TestVerifyRejectsNilShare(t *testing.T) {
	vs, shares := makeValidShares(t)
	share := *shares[0]
	share.Share = nil
	if share.Verify(tss.S256(), 1, vs) {
		t.Fatal("nil share should be rejected")
	}
}

func TestVerifyRejectsNilShareID(t *testing.T) {
	vs, shares := makeValidShares(t)
	share := *shares[0]
	share.ID = nil
	if share.Verify(tss.S256(), 1, vs) {
		t.Fatal("nil share ID should be rejected")
	}
}

func TestVerifyRejectsZeroShareID(t *testing.T) {
	vs, shares := makeValidShares(t)
	share := *shares[0]
	share.ID = big.NewInt(0)
	if share.Verify(tss.S256(), 1, vs) {
		t.Fatal("zero share ID should be rejected")
	}
}

func TestVerifyRejectsShareIDEqualToQ(t *testing.T) {
	vs, shares := makeValidShares(t)
	q := tss.S256().Params().N
	share := *shares[0]
	share.ID = new(big.Int).Set(q)
	if share.Verify(tss.S256(), 1, vs) {
		t.Fatal("share ID == q should be rejected (q mod q == 0)")
	}
}

func TestReconstructRejectsDuplicateIDs(t *testing.T) {
	_, shares := makeValidShares(t)
	shares[1].ID = new(big.Int).Set(shares[0].ID)
	_, err := shares.ReConstruct(tss.S256())
	if err == nil {
		t.Fatal("duplicate share IDs should cause ReConstruct to fail")
	}
}

func TestReconstructRejectsDuplicateModQ(t *testing.T) {
	_, shares := makeValidShares(t)
	q := tss.S256().Params().N
	shares[1].ID = new(big.Int).Add(shares[0].ID, q)
	_, err := shares.ReConstruct(tss.S256())
	if err == nil {
		t.Fatal("share IDs congruent mod q should cause ReConstruct to fail")
	}
}
