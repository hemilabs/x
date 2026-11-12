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

// TestUpdatePublicKeyAndAdjustBigXjRejectsWrongChild verifies that a
// child key that is not ECDSAPub + delta*G is rejected, and that the
// keys do not change.
func TestUpdatePublicKeyAndAdjustBigXjRejectsWrongChild(t *testing.T) {
	ec := tss.S256()
	pub := crypto.ScalarBaseMult(ec, big.NewInt(42))
	save := keygen.LocalPartySaveData{ECDSAPub: pub, BigXj: []*crypto.ECPoint{pub}}
	wrong := crypto.ScalarBaseMult(ec, big.NewInt(99))
	extPub := &ecdsa.PublicKey{Curve: ec, X: wrong.X(), Y: wrong.Y()}

	keys := []keygen.LocalPartySaveData{save}
	if err := UpdatePublicKeyAndAdjustBigXj(big.NewInt(7), keys, extPub, ec); err == nil {
		t.Fatal("expected an error for a wrong child key")
	}
	if !keys[0].ECDSAPub.Equals(pub) || !keys[0].BigXj[0].Equals(pub) {
		t.Fatal("a rejected call changed the keys")
	}
}

// TestUpdatePublicKeyAndAdjustBigXjErrorKeepsKeys verifies that an error
// on a later BigXj leaves the key and the earlier BigXj unchanged.
func TestUpdatePublicKeyAndAdjustBigXjErrorKeepsKeys(t *testing.T) {
	ec := tss.S256()
	delta := big.NewInt(7)
	pub := crypto.ScalarBaseMult(ec, big.NewInt(42))
	// minusDelta + delta*G is the identity point, so Add fails.
	minusDelta := crypto.ScalarBaseMult(ec, new(big.Int).Sub(ec.Params().N, delta))
	save := keygen.LocalPartySaveData{ECDSAPub: pub, BigXj: []*crypto.ECPoint{pub, minusDelta}}
	child, err := pub.Add(crypto.ScalarBaseMult(ec, delta))
	if err != nil {
		t.Fatal(err)
	}
	extPub := &ecdsa.PublicKey{Curve: ec, X: child.X(), Y: child.Y()}

	keys := []keygen.LocalPartySaveData{save}
	if err := UpdatePublicKeyAndAdjustBigXj(delta, keys, extPub, ec); err == nil {
		t.Fatal("expected an error for an identity BigXj")
	}
	if !keys[0].ECDSAPub.Equals(pub) || !keys[0].BigXj[0].Equals(pub) {
		t.Fatal("a failed call changed the keys")
	}
}
