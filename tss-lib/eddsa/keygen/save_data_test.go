package keygen

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hemilabs/x/tss-lib/v2/crypto"
)

// copySaveData returns a copy of LocalPartySaveData with deep-copied scalar
// fields and shallow-copied slices (individual elements are not mutated by
// the tests that use this helper -- only slice headers or top-level fields
// are replaced).
func copySaveData(sd LocalPartySaveData) LocalPartySaveData {
	cp := sd
	cp.Xi = new(big.Int).Set(sd.Xi)
	cp.ShareID = new(big.Int).Set(sd.ShareID)
	cp.Ks = make([]*big.Int, len(sd.Ks))
	copy(cp.Ks, sd.Ks)
	cp.BigXj = make([]*crypto.ECPoint, len(sd.BigXj))
	copy(cp.BigXj, sd.BigXj)
	return cp
}

// ---------------------------------------------------------------------------
// ValidateSaveData tests
// ---------------------------------------------------------------------------

func TestEdDSAValidateSaveDataHappyPath(t *testing.T) {
	fixtures, _, err := LoadKeygenTestFixtures(1)
	if err != nil {
		t.Skipf("skipping: could not load EdDSA keygen fixtures: %v", err)
	}
	key := fixtures[0]
	assert.NoError(t, key.ValidateSaveData(), "valid save data should pass validation")
}

func TestEdDSAValidateSaveDataRejectsNilXi(t *testing.T) {
	fixtures, _, err := LoadKeygenTestFixtures(1)
	if err != nil {
		t.Skipf("skipping: could not load EdDSA keygen fixtures: %v", err)
	}
	key := copySaveData(fixtures[0])
	key.Xi = nil
	assert.Error(t, key.ValidateSaveData(), "nil Xi should be rejected")
}

func TestEdDSAValidateSaveDataRejectsTamperedXi(t *testing.T) {
	fixtures, _, err := LoadKeygenTestFixtures(1)
	if err != nil {
		t.Skipf("skipping: could not load EdDSA keygen fixtures: %v", err)
	}
	key := copySaveData(fixtures[0])
	key.Xi = new(big.Int).Add(key.Xi, big.NewInt(1))
	assert.Error(t, key.ValidateSaveData(), "tampered Xi should fail Feldman check")
}

func TestEdDSAValidateSaveDataRejectsArrayMismatch(t *testing.T) {
	fixtures, _, err := LoadKeygenTestFixtures(1)
	if err != nil {
		t.Skipf("skipping: could not load EdDSA keygen fixtures: %v", err)
	}
	key := copySaveData(fixtures[0])
	key.BigXj = key.BigXj[:len(key.BigXj)-1]
	assert.Error(t, key.ValidateSaveData(), "mismatched array lengths should be rejected")
}

func TestEdDSAValidateSaveDataRejectsShareIDNotInKs(t *testing.T) {
	fixtures, _, err := LoadKeygenTestFixtures(1)
	if err != nil {
		t.Skipf("skipping: could not load EdDSA keygen fixtures: %v", err)
	}
	key := copySaveData(fixtures[0])
	key.ShareID = big.NewInt(999999)
	assert.Error(t, key.ValidateSaveData(), "ShareID not in Ks should be rejected")
}
