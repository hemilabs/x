// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package common_test

import (
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss-lib/v3/common"
)

const (
	randomIntBitLen = 1024
)

func TestGetRandomInt(t *testing.T) {
	rnd := common.MustGetRandomInt(rand.Reader, randomIntBitLen)
	if rnd == nil {
		t.Fatal("rand int should not be zero")
	}
}

func TestGetRandomPositiveInt(t *testing.T) {
	rnd := common.MustGetRandomInt(rand.Reader, randomIntBitLen)
	rndPos := common.GetRandomPositiveInt(rand.Reader, rnd)
	if rndPos == nil {
		t.Fatal("rand int should not be zero")
	}
	if rndPos.Cmp(big.NewInt(0)) != 1 {
		t.Fatal("rand int should be positive")
	}
}

func TestGetRandomPositiveRelativelyPrimeInt(t *testing.T) {
	rnd := common.MustGetRandomInt(rand.Reader, randomIntBitLen)
	rndPosRP := common.GetRandomPositiveRelativelyPrimeInt(rand.Reader, rnd)
	if rndPosRP == nil {
		t.Fatal("rand int should not be zero")
	}
	if !common.IsNumberInMultiplicativeGroup(rnd, rndPosRP) {
		t.Fatal("expected true")
	}
	if rndPosRP.Cmp(big.NewInt(0)) != 1 {
		t.Fatal("rand int should be positive")
	}
	// TODO test for relative primeness
}

func TestGetRandomPrimeInt(t *testing.T) {
	prime := common.GetRandomPrimeInt(rand.Reader, randomIntBitLen)
	if prime == nil {
		t.Fatal("rand prime should not be zero")
	}
	if !prime.ProbablyPrime(50) {
		t.Fatal("rand prime should be prime")
	}
}
