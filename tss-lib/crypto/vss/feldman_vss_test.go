// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package vss_test

import (
	"crypto/rand"
	"math/big"
	"testing"


	"github.com/hemilabs/x/tss-lib/v3/common"
	. "github.com/hemilabs/x/tss-lib/v3/crypto/vss"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

func TestCheckIndexesDup(t *testing.T) {
	indexes := make([]*big.Int, 0)
	for i := 0; i < 1000; i++ {
		indexes = append(indexes, common.GetRandomPositiveInt(rand.Reader, tss.EC().Params().N))
	}
	_, e := CheckIndexes(tss.EC(), indexes)
	if e != nil {
		t.Fatal(e)
	}

	indexes = append(indexes, indexes[99])
	_, e = CheckIndexes(tss.EC(), indexes)
	if e == nil {
		t.Fatal("expected error")
	}
}

func TestCheckIndexesZero(t *testing.T) {
	indexes := make([]*big.Int, 0)
	for i := 0; i < 1000; i++ {
		indexes = append(indexes, common.GetRandomPositiveInt(rand.Reader, tss.EC().Params().N))
	}
	_, e := CheckIndexes(tss.EC(), indexes)
	if e != nil {
		t.Fatal(e)
	}

	indexes = append(indexes, tss.EC().Params().N)
	_, e = CheckIndexes(tss.EC(), indexes)
	if e == nil {
		t.Fatal("expected error")
	}
}

func TestCreate(t *testing.T) {
	num, threshold := 5, 3

	secret := common.GetRandomPositiveInt(rand.Reader, tss.EC().Params().N)

	ids := make([]*big.Int, 0)
	for i := 0; i < num; i++ {
		ids = append(ids, common.GetRandomPositiveInt(rand.Reader, tss.EC().Params().N))
	}

	vs, _, _, err := Create(tss.EC(), threshold, secret, ids, rand.Reader)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if threshold+1 != len(vs) {
		t.Fatalf("got %v, want %v", len(vs), threshold+1)
	}
	// assert.Equal(t, num, params.NumShares)

	if threshold+1 != len(vs) {
		t.Fatalf("got %v, want %v", len(vs), threshold+1)
	}

	// ensure that each vs has two points on the curve
	for i, pg := range vs {
		if pg.X() == nil {
			t.Fatal("expected non-zero")
		}
		if pg.Y() == nil {
			t.Fatal("expected non-zero")
		}
		if !pg.IsOnCurve() {
			t.Fatal("expected true")
		}
		if vs[i].X() == nil {
			t.Fatal("expected non-zero")
		}
		if vs[i].Y() == nil {
			t.Fatal("expected non-zero")
		}
	}
}

func TestVerify(t *testing.T) {
	num, threshold := 5, 3

	secret := common.GetRandomPositiveInt(rand.Reader, tss.EC().Params().N)

	ids := make([]*big.Int, 0)
	for i := 0; i < num; i++ {
		ids = append(ids, common.GetRandomPositiveInt(rand.Reader, tss.EC().Params().N))
	}

	vs, shares, _, err := Create(tss.EC(), threshold, secret, ids, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < num; i++ {
		if !(shares[i].Verify(tss.EC(), threshold, vs)) {
			t.Fatal("expected true")
		}
	}
}

func TestReconstruct(t *testing.T) {
	num, threshold := 5, 3

	secret := common.GetRandomPositiveInt(rand.Reader, tss.EC().Params().N)

	ids := make([]*big.Int, 0)
	for i := 0; i < num; i++ {
		ids = append(ids, common.GetRandomPositiveInt(rand.Reader, tss.EC().Params().N))
	}

	_, shares, _, err := Create(tss.EC(), threshold, secret, ids, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	secret2, err2 := shares[:threshold-1].ReConstruct(tss.EC())
	if err2 == nil {
		t.Fatal("expected error")
	}
	if secret2 != nil {
		t.Fatalf("expected nil, got %v", secret2)
	}

	secret3, err3 := shares[:threshold].ReConstruct(tss.EC())
	if err3 != nil {
		t.Fatal(err3)
	}
	if secret3 == nil {
		t.Fatal("expected non-zero")
	}

	secret4, err4 := shares[:num].ReConstruct(tss.EC())
	if err4 != nil {
		t.Fatal(err4)
	}
	if secret4 == nil {
		t.Fatal("expected non-zero")
	}
}
