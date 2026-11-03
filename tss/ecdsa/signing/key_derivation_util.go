// Copyright (c) 2021 Swingby
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package signing

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"errors"
	"fmt"
	"math/big"

	"github.com/hemilabs/x/tss/v3/crypto"
	"github.com/hemilabs/x/tss/v3/ecdsa/keygen"
)

// UpdatePublicKeyAndAdjustBigXj adjusts the distributed public key and BigXj shares for BIP-32 key derivation.
// It returns an error if delta is zero, or if the child key is not
// ECDSAPub + delta*G for each key.  On an error, the keys do not change.
func UpdatePublicKeyAndAdjustBigXj(keyDerivationDelta *big.Int, keys []keygen.LocalPartySaveData, extendedChildPk *ecdsa.PublicKey, ec elliptic.Curve) error {
	// [FORK] Guard keyDerivationDelta=0: ScalarBaseMult(0) panics (identity point).
	// keyDerivationDelta is a sum of BIP-32 IL values mod q; each is validated non-zero
	// individually, but their sum mod q could be 0 with probability ~2^-256.
	if keyDerivationDelta.Sign() == 0 {
		return errors.New("UpdatePublicKeyAndAdjustBigXj: keyDerivationDelta is zero")
	}
	if extendedChildPk == nil {
		return errors.New("UpdatePublicKeyAndAdjustBigXj: child key is nil")
	}
	gDelta := crypto.ScalarBaseMult(ec, keyDerivationDelta)
	childPk, err := crypto.NewECPoint(ec, extendedChildPk.X, extendedChildPk.Y)
	if err != nil {
		return fmt.Errorf("UpdatePublicKeyAndAdjustBigXj: child key: %w", err)
	}
	// Calculate all the new values before the keys change, so that an
	// error leaves the keys as they were.
	newBigXj := make([][]*crypto.ECPoint, len(keys))
	for k := range keys {
		newBigXj[k], err = adjustBigXj(keys[k], gDelta, childPk)
		if err != nil {
			return fmt.Errorf("UpdatePublicKeyAndAdjustBigXj: key %d: %w", k, err)
		}
	}
	for k := range keys {
		keys[k].ECDSAPub = childPk
		copy(keys[k].BigXj, newBigXj[k])
	}
	return nil
}

// adjustBigXj returns BigXj + delta*G for one key.  The child key must
// be ECDSAPub + delta*G.  Otherwise the adjusted BigXj do not belong to
// the child key.
func adjustBigXj(key keygen.LocalPartySaveData, gDelta, childPk *crypto.ECPoint) ([]*crypto.ECPoint, error) {
	if key.ECDSAPub == nil {
		return nil, errors.New("ECDSAPub is nil")
	}
	want, err := key.ECDSAPub.Add(gDelta)
	if err != nil {
		return nil, fmt.Errorf("ECDSAPub + delta*G: %w", err)
	}
	if !want.Equals(childPk) {
		return nil, errors.New("child key is not ECDSAPub + delta*G")
	}
	// Suppose X_j has shamir shares X_j0,     X_j1,     ..., X_jn
	// So X_j + D has shamir shares  X_j0 + D, X_j1 + D, ..., X_jn + D
	bigXj := make([]*crypto.ECPoint, len(key.BigXj))
	for j, xj := range key.BigXj {
		if xj == nil {
			return nil, fmt.Errorf("BigXj[%d] is nil", j)
		}
		bigXj[j], err = xj.Add(gDelta)
		if err != nil {
			return nil, fmt.Errorf("BigXj[%d] + delta*G: %w", j, err)
		}
	}
	return bigXj, nil
}
