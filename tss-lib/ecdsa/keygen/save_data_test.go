// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss-lib/v3/crypto"
	"github.com/hemilabs/x/tss-lib/v3/crypto/paillier"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// validSaveData builds a minimal LocalPartySaveData that passes
// ValidateSaveData.  n=2, Xi=7, ShareID=Ks[0].
func validSaveData(t *testing.T) LocalPartySaveData {
	t.Helper()
	ec := tss.S256()
	xi := big.NewInt(7)
	sd := NewLocalPartySaveData(2)

	sd.Xi = xi
	sd.ShareID = big.NewInt(100)

	// Ks: own share ID first, then another.
	sd.Ks[0] = new(big.Int).Set(sd.ShareID)
	sd.Ks[1] = big.NewInt(200)

	// BigXj: own = Xi·G, other = arbitrary on-curve point.
	sd.BigXj[0] = crypto.ScalarBaseMult(ec, xi)
	sd.BigXj[1] = crypto.ScalarBaseMult(ec, big.NewInt(13))

	sd.ECDSAPub = crypto.ScalarBaseMult(ec, big.NewInt(42))

	for i := 0; i < 2; i++ {
		sd.NTildej[i] = big.NewInt(int64(1000 + i))
		sd.H1j[i] = big.NewInt(int64(2000 + i))
		sd.H2j[i] = big.NewInt(int64(3000 + i))
		sd.PaillierPKs[i] = &paillier.PublicKey{N: big.NewInt(int64(4000 + i))}
	}
	return sd
}

func TestValidateSaveDataHappy(t *testing.T) {
	sd := validSaveData(t)
	if err := sd.ValidateSaveData(); err != nil {
		t.Fatalf("valid data should pass: %v", err)
	}
}

func TestValidateSaveDataNilXi(t *testing.T) {
	sd := validSaveData(t)
	sd.Xi = nil
	if err := sd.ValidateSaveData(); err == nil {
		t.Fatal("nil Xi should fail")
	}
}

func TestValidateSaveDataNilShareID(t *testing.T) {
	sd := validSaveData(t)
	sd.ShareID = nil
	if err := sd.ValidateSaveData(); err == nil {
		t.Fatal("nil ShareID should fail")
	}
}

func TestValidateSaveDataNilECDSAPub(t *testing.T) {
	sd := validSaveData(t)
	sd.ECDSAPub = nil
	if err := sd.ValidateSaveData(); err == nil {
		t.Fatal("nil ECDSAPub should fail")
	}
}

func TestValidateSaveDataTooFewParties(t *testing.T) {
	sd := validSaveData(t)
	sd.Ks = []*big.Int{big.NewInt(1)}
	sd.BigXj = sd.BigXj[:1]
	sd.NTildej = sd.NTildej[:1]
	sd.H1j = sd.H1j[:1]
	sd.H2j = sd.H2j[:1]
	sd.PaillierPKs = sd.PaillierPKs[:1]
	if err := sd.ValidateSaveData(); err == nil {
		t.Fatal("n < 2 should fail")
	}
}

func TestValidateSaveDataArrayMismatch(t *testing.T) {
	sd := validSaveData(t)
	sd.BigXj = sd.BigXj[:1] // length 1 vs Ks length 2
	if err := sd.ValidateSaveData(); err == nil {
		t.Fatal("array length mismatch should fail")
	}
}

func TestValidateSaveDataNilKsElement(t *testing.T) {
	sd := validSaveData(t)
	sd.Ks[1] = nil
	if err := sd.ValidateSaveData(); err == nil {
		t.Fatal("nil Ks element should fail")
	}
}

func TestValidateSaveDataNilBigXjElement(t *testing.T) {
	sd := validSaveData(t)
	sd.BigXj[1] = nil
	if err := sd.ValidateSaveData(); err == nil {
		t.Fatal("nil BigXj element should fail")
	}
}

func TestValidateSaveDataOffCurveBigXj(t *testing.T) {
	sd := validSaveData(t)
	sd.BigXj[1] = crypto.NewECPointNoCurveCheck(tss.S256(), big.NewInt(999), big.NewInt(999))
	if err := sd.ValidateSaveData(); err == nil {
		t.Fatal("off-curve BigXj should fail")
	}
}

func TestValidateSaveDataNilNTildej(t *testing.T) {
	sd := validSaveData(t)
	sd.NTildej[0] = nil
	if err := sd.ValidateSaveData(); err == nil {
		t.Fatal("nil NTildej should fail")
	}
}

func TestValidateSaveDataNilH1j(t *testing.T) {
	sd := validSaveData(t)
	sd.H1j[0] = nil
	if err := sd.ValidateSaveData(); err == nil {
		t.Fatal("nil H1j should fail")
	}
}

func TestValidateSaveDataNilH2j(t *testing.T) {
	sd := validSaveData(t)
	sd.H2j[0] = nil
	if err := sd.ValidateSaveData(); err == nil {
		t.Fatal("nil H2j should fail")
	}
}

func TestValidateSaveDataNilPaillierPK(t *testing.T) {
	sd := validSaveData(t)
	sd.PaillierPKs[0] = nil
	if err := sd.ValidateSaveData(); err == nil {
		t.Fatal("nil PaillierPKs should fail")
	}
}

func TestValidateSaveDataShareIDNotInKs(t *testing.T) {
	sd := validSaveData(t)
	sd.ShareID = big.NewInt(999) // not in Ks
	if err := sd.ValidateSaveData(); err == nil {
		t.Fatal("ShareID not in Ks should fail")
	}
}

func TestValidateSaveDataZeroXi(t *testing.T) {
	sd := validSaveData(t)
	sd.Xi = big.NewInt(0)
	if err := sd.ValidateSaveData(); err == nil {
		t.Fatal("zero Xi should fail")
	}
}

func TestValidateSaveDataFeldmanFail(t *testing.T) {
	sd := validSaveData(t)
	// Xi·G won't match BigXj[0] anymore.
	sd.Xi = big.NewInt(99)
	if err := sd.ValidateSaveData(); err == nil {
		t.Fatal("Feldman check should fail when Xi doesn't match BigXj")
	}
}

func TestBuildLocalSaveDataSubsetSuccess(t *testing.T) {
	sd := validSaveData(t)
	ids := tss.GenerateTestPartyIDs(2)
	// Align Ks with party keys so the lookup succeeds.
	sd.Ks[0] = new(big.Int).SetBytes(ids[0].Key)
	sd.Ks[1] = new(big.Int).SetBytes(ids[1].Key)
	sd.ShareID = sd.Ks[0]

	// Fix BigXj[0] to match Xi·G after ShareID change.
	ec := tss.S256()
	sd.BigXj[0] = crypto.ScalarBaseMult(ec, sd.Xi)

	result := BuildLocalSaveDataSubset(sd, ids)
	if len(result.Ks) != 2 {
		t.Fatalf("expected 2 Ks, got %d", len(result.Ks))
	}
	if result.Ks[0].Cmp(sd.Ks[0]) != 0 {
		t.Fatal("Ks[0] mismatch")
	}
}

func TestValidatePreParamsNilFields(t *testing.T) {
	pp := LocalPreParams{}
	if pp.Validate() {
		t.Fatal("all-nil should be invalid")
	}
}

func TestValidateWithProofNilAlpha(t *testing.T) {
	pp := LocalPreParams{
		PaillierSK: &paillier.PrivateKey{PublicKey: paillier.PublicKey{N: big.NewInt(1)}},
		NTildei:    big.NewInt(1),
		H1i:        big.NewInt(1),
		H2i:        big.NewInt(1),
		// Alpha nil
		Beta: big.NewInt(1),
		P:    big.NewInt(1),
		Q:    big.NewInt(2),
	}
	if pp.ValidateWithProof() {
		t.Fatal("nil Alpha should be invalid")
	}
}

func TestValidateWithProofPEqualsQ(t *testing.T) {
	pp := LocalPreParams{
		PaillierSK: &paillier.PrivateKey{PublicKey: paillier.PublicKey{N: big.NewInt(1)}},
		NTildei:    big.NewInt(1),
		H1i:        big.NewInt(1),
		H2i:        big.NewInt(1),
		Alpha:      big.NewInt(1),
		Beta:       big.NewInt(1),
		P:          big.NewInt(5),
		Q:          big.NewInt(5),
	}
	if pp.ValidateWithProof() {
		t.Fatal("P == Q should be invalid")
	}
}

func TestValidateWithProofBadNTilde(t *testing.T) {
	pp := LocalPreParams{
		PaillierSK: &paillier.PrivateKey{PublicKey: paillier.PublicKey{N: big.NewInt(1)}},
		NTildei:    big.NewInt(999), // wrong
		H1i:        big.NewInt(1),
		H2i:        big.NewInt(1),
		Alpha:      big.NewInt(1),
		Beta:       big.NewInt(1),
		P:          big.NewInt(5),
		Q:          big.NewInt(7),
	}
	if pp.ValidateWithProof() {
		t.Fatal("wrong NTilde should be invalid")
	}
}
