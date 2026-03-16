// Copyright © 2021 Swingby

package signing

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"errors"
	"math/big"

	"github.com/hemilabs/x/tss-lib/v3/common"
	"github.com/hemilabs/x/tss-lib/v3/crypto"
	"github.com/hemilabs/x/tss-lib/v3/ecdsa/keygen"
)

// UpdatePublicKeyAndAdjustBigXj adjusts the distributed public key and BigXj shares for BIP-32 key derivation.
func UpdatePublicKeyAndAdjustBigXj(keyDerivationDelta *big.Int, keys []keygen.LocalPartySaveData, extendedChildPk *ecdsa.PublicKey, ec elliptic.Curve) error {
	// [FORK] Guard keyDerivationDelta=0: ScalarBaseMult(0) panics (identity point).
	// keyDerivationDelta is a sum of BIP-32 IL values mod q; each is validated non-zero
	// individually, but their sum mod q could be 0 with probability ~2^-256.
	if keyDerivationDelta.Sign() == 0 {
		return errors.New("UpdatePublicKeyAndAdjustBigXj: keyDerivationDelta is zero")
	}
	var err error
	gDelta := crypto.ScalarBaseMult(ec, keyDerivationDelta)
	for k := range keys {
		keys[k].ECDSAPub, err = crypto.NewECPoint(ec, extendedChildPk.X, extendedChildPk.Y)
		if err != nil {
			common.Logger.Errorf("error creating new extended child public key")
			return err
		}
		// Suppose X_j has shamir shares X_j0,     X_j1,     ..., X_jn
		// So X_j + D has shamir shares  X_j0 + D, X_j1 + D, ..., X_jn + D
		for j := range keys[k].BigXj {
			keys[k].BigXj[j], err = keys[k].BigXj[j].Add(gDelta)
			if err != nil {
				common.Logger.Errorf("error in delta operation")
				return err
			}
		}
	}
	return nil
}
