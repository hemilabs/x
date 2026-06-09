// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package signing

import (
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss/v3/crypto"
	"github.com/hemilabs/x/tss/v3/ecdsa/keygen"
	"github.com/hemilabs/x/tss/v3/tss"
)

func TestUpdatePublicKeyAndAdjustBigXj(t *testing.T) {
	ec := tss.S256()

	// Create a fake key share with a known public key.
	privKey := big.NewInt(42)
	pub := crypto.ScalarBaseMult(ec, privKey)
	bigXj := []*crypto.ECPoint{pub}

	save := keygen.LocalPartySaveData{
		ECDSAPub: pub,
		BigXj:    bigXj,
	}

	delta := big.NewInt(7)
	deltaG := crypto.ScalarBaseMult(ec, delta)

	// New public key = pub + delta*G
	newPub, err := pub.Add(deltaG)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	extPub := &ecdsa.PublicKey{
		Curve: ec,
		X:     newPub.X(),
		Y:     newPub.Y(),
	}

	keys := []keygen.LocalPartySaveData{save}
	err = UpdatePublicKeyAndAdjustBigXj(delta, keys, extPub, ec)
	if err != nil {
		t.Fatalf("UpdatePublicKeyAndAdjustBigXj: %v", err)
	}

	// ECDSAPub should now be newPub.
	if !keys[0].ECDSAPub.Equals(newPub) {
		t.Fatal("ECDSAPub not updated")
	}
	// BigXj[0] should be original + delta*G.
	expectedBigXj, err := pub.Add(deltaG)
	if err != nil {
		t.Fatalf("expected Add: %v", err)
	}
	if !keys[0].BigXj[0].Equals(expectedBigXj) {
		t.Fatal("BigXj[0] not adjusted")
	}
}

func TestUpdatePublicKeyAndAdjustBigXjZeroDelta(t *testing.T) {
	ec := tss.S256()
	pub := crypto.ScalarBaseMult(ec, big.NewInt(42))
	save := keygen.LocalPartySaveData{
		ECDSAPub: pub,
		BigXj:    []*crypto.ECPoint{pub},
	}
	extPub := &ecdsa.PublicKey{Curve: ec, X: pub.X(), Y: pub.Y()}

	err := UpdatePublicKeyAndAdjustBigXj(big.NewInt(0), []keygen.LocalPartySaveData{save}, extPub, ec)
	if err == nil {
		t.Fatal("expected error for zero delta")
	}
}
