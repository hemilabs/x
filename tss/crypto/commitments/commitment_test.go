// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package commitments_test

import (
	"crypto/rand"
	"math/big"
	"testing"

	. "github.com/hemilabs/x/tss-lib/v3/crypto/commitments"
)

func TestCreateVerify(t *testing.T) {
	one := big.NewInt(1)
	zero := big.NewInt(0)

	commitment := NewHashCommitment(rand.Reader, zero, one)
	pass := commitment.Verify()

	if !pass {
		t.Fatal("must pass")
	}
}

func TestDeCommit(t *testing.T) {
	one := big.NewInt(1)
	zero := big.NewInt(0)

	commitment := NewHashCommitment(rand.Reader, zero, one)
	pass, secrets := commitment.DeCommit()

	if !pass {
		t.Fatal("must pass")
	}

	if len(secrets) == 0 {
		t.Fatal("len(secrets) must be non-zero")
	}
}
