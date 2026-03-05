// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package keygen

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/hemilabs/x/tss-lib/v2/crypto"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

type (
	LocalSecrets struct {
		// secret fields (not shared, but stored locally)
		Xi, ShareID *big.Int // xi, kj
	}

	// Everything in LocalPartySaveData is saved locally to user's HD when done
	LocalPartySaveData struct {
		LocalSecrets

		// original indexes (ki in signing preparation phase)
		Ks []*big.Int

		// public keys (Xj = uj*G for each Pj)
		BigXj []*crypto.ECPoint // Xj

		// used for test assertions (may be discarded)
		EDDSAPub *crypto.ECPoint // y
	}
)

func NewLocalPartySaveData(partyCount int) (saveData LocalPartySaveData) {
	saveData.Ks = make([]*big.Int, partyCount)
	saveData.BigXj = make([]*crypto.ECPoint, partyCount)
	return
}

// [FORK] ValidateSaveData performs comprehensive validation of loaded EdDSA save data,
// checking for nil fields, array consistency, curve membership of public keys,
// and the Feldman VSS invariant (Xi·G == BigXj[ownIndex]).
//
// Call this after loading save data from storage and before using it in signing
// or resharing. Without this, corrupted or tampered save data could silently produce
// invalid signatures, which would be indistinguishable from a protocol failure
// at a remote party.
//
// Returns a descriptive error or nil if all checks pass.
func (saveData LocalPartySaveData) ValidateSaveData() error {
	// Secret fields.
	if saveData.Xi == nil || saveData.ShareID == nil {
		return errors.New("ValidateSaveData: Xi or ShareID is nil")
	}
	if saveData.EDDSAPub == nil {
		return errors.New("ValidateSaveData: EDDSAPub is nil")
	}

	// Array consistency.
	n := len(saveData.Ks)
	if n < 2 {
		return fmt.Errorf("ValidateSaveData: party count %d is less than 2", n)
	}
	if len(saveData.BigXj) != n {
		return errors.New("ValidateSaveData: BigXj length does not match Ks")
	}

	// Per-party field nil checks.
	for i := 0; i < n; i++ {
		if saveData.Ks[i] == nil {
			return fmt.Errorf("ValidateSaveData: Ks[%d] is nil", i)
		}
		if saveData.BigXj[i] == nil {
			return fmt.Errorf("ValidateSaveData: BigXj[%d] is nil", i)
		}
		if !saveData.BigXj[i].IsOnCurve() {
			return fmt.Errorf("ValidateSaveData: BigXj[%d] is not on curve", i)
		}
	}

	// Find own index from Ks using ShareID.
	ownIdx := -1
	for i, k := range saveData.Ks {
		if k.Cmp(saveData.ShareID) == 0 {
			ownIdx = i
			break
		}
	}
	if ownIdx == -1 {
		return errors.New("ValidateSaveData: ShareID not found in Ks")
	}

	// Feldman VSS invariant: Xi·G must equal BigXj[ownIndex].
	// [FORK] Guard Xi=0: ScalarBaseMult(0) panics (identity point). A zero Xi means
	// the party's secret share is trivially known.
	if saveData.Xi.Sign() == 0 {
		return errors.New("ValidateSaveData: Xi is zero")
	}
	ec := saveData.BigXj[ownIdx].Curve()
	xiG := crypto.ScalarBaseMult(ec, saveData.Xi)
	if !xiG.Equals(saveData.BigXj[ownIdx]) {
		return errors.New("ValidateSaveData: Feldman VSS check failed: Xi·G != BigXj[ownIndex]")
	}

	return nil
}

// BuildLocalSaveDataSubset re-creates the LocalPartySaveData to contain data for only the list of signing parties.
func BuildLocalSaveDataSubset(sourceData LocalPartySaveData, sortedIDs tss.SortedPartyIDs) LocalPartySaveData {
	keysToIndices := make(map[string]int, len(sourceData.Ks))
	for j, kj := range sourceData.Ks {
		keysToIndices[hex.EncodeToString(kj.Bytes())] = j
	}
	newData := NewLocalPartySaveData(sortedIDs.Len())
	newData.LocalSecrets = sourceData.LocalSecrets
	newData.EDDSAPub = sourceData.EDDSAPub
	for j, id := range sortedIDs {
		savedIdx, ok := keysToIndices[hex.EncodeToString(id.Key)]
		if !ok {
			panic("BuildLocalSaveDataSubset: unable to find a signer party in the local save data")
		}
		newData.Ks[j] = sourceData.Ks[savedIdx]
		newData.BigXj[j] = sourceData.BigXj[savedIdx]
	}
	return newData
}
