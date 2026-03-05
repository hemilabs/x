package keygen

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hemilabs/x/tss-lib/v2/crypto"
	"github.com/hemilabs/x/tss-lib/v2/crypto/paillier"
)

// copyPreParams returns a deep copy of LocalPreParams so that mutations
// in one test do not affect the shared fixture data.
func copyPreParams(pp LocalPreParams) LocalPreParams {
	cp := pp
	if pp.NTildei != nil {
		cp.NTildei = new(big.Int).Set(pp.NTildei)
	}
	if pp.H1i != nil {
		cp.H1i = new(big.Int).Set(pp.H1i)
	}
	if pp.H2i != nil {
		cp.H2i = new(big.Int).Set(pp.H2i)
	}
	if pp.Alpha != nil {
		cp.Alpha = new(big.Int).Set(pp.Alpha)
	}
	if pp.Beta != nil {
		cp.Beta = new(big.Int).Set(pp.Beta)
	}
	if pp.P != nil {
		cp.P = new(big.Int).Set(pp.P)
	}
	if pp.Q != nil {
		cp.Q = new(big.Int).Set(pp.Q)
	}
	// PaillierSK is a pointer; shallow copy is fine for validation tests.
	return cp
}

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
	cp.NTildej = make([]*big.Int, len(sd.NTildej))
	copy(cp.NTildej, sd.NTildej)
	cp.H1j = make([]*big.Int, len(sd.H1j))
	copy(cp.H1j, sd.H1j)
	cp.H2j = make([]*big.Int, len(sd.H2j))
	copy(cp.H2j, sd.H2j)
	cp.PaillierPKs = make([]*paillier.PublicKey, len(sd.PaillierPKs))
	copy(cp.PaillierPKs, sd.PaillierPKs)
	return cp
}

// ---------------------------------------------------------------------------
// ValidateWithProof tests
// ---------------------------------------------------------------------------

func TestValidateWithProofHappyPath(t *testing.T) {
	fixtures, _, err := LoadKeygenTestFixtures(1)
	if err != nil {
		t.Skipf("skipping: could not load ECDSA keygen fixtures: %v", err)
	}
	pp := fixtures[0].LocalPreParams
	assert.True(t, pp.ValidateWithProof(), "valid pre-params should pass ValidateWithProof")
}

func TestValidateWithProofRejectsPEqualsQ(t *testing.T) {
	fixtures, _, err := LoadKeygenTestFixtures(1)
	if err != nil {
		t.Skipf("skipping: could not load ECDSA keygen fixtures: %v", err)
	}
	pp := copyPreParams(fixtures[0].LocalPreParams)
	pp.Q = new(big.Int).Set(pp.P)
	assert.False(t, pp.ValidateWithProof(), "P == Q should be rejected")
}

func TestValidateWithProofRejectsTamperedNTilde(t *testing.T) {
	fixtures, _, err := LoadKeygenTestFixtures(1)
	if err != nil {
		t.Skipf("skipping: could not load ECDSA keygen fixtures: %v", err)
	}
	pp := copyPreParams(fixtures[0].LocalPreParams)
	pp.NTildei = new(big.Int).Add(pp.NTildei, big.NewInt(1))
	assert.False(t, pp.ValidateWithProof(), "tampered NTilde should be rejected")
}

func TestValidateWithProofRejectsTamperedH2(t *testing.T) {
	fixtures, _, err := LoadKeygenTestFixtures(1)
	if err != nil {
		t.Skipf("skipping: could not load ECDSA keygen fixtures: %v", err)
	}
	pp := copyPreParams(fixtures[0].LocalPreParams)
	pp.H2i = new(big.Int).Add(pp.H2i, big.NewInt(1))
	assert.False(t, pp.ValidateWithProof(), "tampered H2 should be rejected")
}

func TestValidateWithProofRejectsNilFields(t *testing.T) {
	fixtures, _, err := LoadKeygenTestFixtures(1)
	if err != nil {
		t.Skipf("skipping: could not load ECDSA keygen fixtures: %v", err)
	}

	t.Run("NilP", func(t *testing.T) {
		pp := copyPreParams(fixtures[0].LocalPreParams)
		pp.P = nil
		assert.False(t, pp.ValidateWithProof(), "nil P should be rejected")
	})
	t.Run("NilQ", func(t *testing.T) {
		pp := copyPreParams(fixtures[0].LocalPreParams)
		pp.Q = nil
		assert.False(t, pp.ValidateWithProof(), "nil Q should be rejected")
	})
	t.Run("NilAlpha", func(t *testing.T) {
		pp := copyPreParams(fixtures[0].LocalPreParams)
		pp.Alpha = nil
		assert.False(t, pp.ValidateWithProof(), "nil Alpha should be rejected")
	})
	t.Run("NilBeta", func(t *testing.T) {
		pp := copyPreParams(fixtures[0].LocalPreParams)
		pp.Beta = nil
		assert.False(t, pp.ValidateWithProof(), "nil Beta should be rejected")
	})
}

// ---------------------------------------------------------------------------
// ValidateSaveData tests
// ---------------------------------------------------------------------------

func TestValidateSaveDataHappyPath(t *testing.T) {
	fixtures, _, err := LoadKeygenTestFixtures(1)
	if err != nil {
		t.Skipf("skipping: could not load ECDSA keygen fixtures: %v", err)
	}
	key := fixtures[0]
	assert.NoError(t, key.ValidateSaveData(), "valid save data should pass validation")
}

func TestValidateSaveDataRejectsNilXi(t *testing.T) {
	fixtures, _, err := LoadKeygenTestFixtures(1)
	if err != nil {
		t.Skipf("skipping: could not load ECDSA keygen fixtures: %v", err)
	}
	key := copySaveData(fixtures[0])
	key.Xi = nil
	assert.Error(t, key.ValidateSaveData(), "nil Xi should be rejected")
}

func TestValidateSaveDataRejectsTamperedXi(t *testing.T) {
	fixtures, _, err := LoadKeygenTestFixtures(1)
	if err != nil {
		t.Skipf("skipping: could not load ECDSA keygen fixtures: %v", err)
	}
	key := copySaveData(fixtures[0])
	key.Xi = new(big.Int).Add(key.Xi, big.NewInt(1))
	assert.Error(t, key.ValidateSaveData(), "tampered Xi should fail Feldman check")
}

func TestValidateSaveDataRejectsArrayMismatch(t *testing.T) {
	fixtures, _, err := LoadKeygenTestFixtures(1)
	if err != nil {
		t.Skipf("skipping: could not load ECDSA keygen fixtures: %v", err)
	}
	key := copySaveData(fixtures[0])
	key.BigXj = key.BigXj[:len(key.BigXj)-1]
	assert.Error(t, key.ValidateSaveData(), "mismatched array lengths should be rejected")
}

func TestValidateSaveDataRejectsShareIDNotInKs(t *testing.T) {
	fixtures, _, err := LoadKeygenTestFixtures(1)
	if err != nil {
		t.Skipf("skipping: could not load ECDSA keygen fixtures: %v", err)
	}
	key := copySaveData(fixtures[0])
	key.ShareID = big.NewInt(999999)
	assert.Error(t, key.ValidateSaveData(), "ShareID not in Ks should be rejected")
}

func TestValidateSaveDataRejectsNilBigXj(t *testing.T) {
	fixtures, _, err := LoadKeygenTestFixtures(1)
	if err != nil {
		t.Skipf("skipping: could not load ECDSA keygen fixtures: %v", err)
	}
	key := copySaveData(fixtures[0])
	key.BigXj[0] = nil
	assert.Error(t, key.ValidateSaveData(), "nil BigXj element should be rejected")
}
