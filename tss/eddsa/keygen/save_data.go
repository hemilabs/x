// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/hemilabs/x/tss/v3/crypto"
	"github.com/hemilabs/x/tss/v3/crypto/vss"
	"github.com/hemilabs/x/tss/v3/tss"
)

// ValidateSaveData checks that the saved keygen data is consistent
// and usable for signing or resharing.
func (saveData LocalPartySaveData) ValidateSaveData() error {
	if saveData.Xi == nil || saveData.ShareID == nil {
		return errors.New("ValidateSaveData: Xi or ShareID is nil")
	}
	if saveData.EDDSAPub == nil {
		return errors.New("ValidateSaveData: EDDSAPub is nil")
	}
	n := len(saveData.Ks)
	if n < 2 {
		return fmt.Errorf("ValidateSaveData: party count %d is less than 2", n)
	}
	if len(saveData.BigXj) != n {
		return errors.New("ValidateSaveData: BigXj length does not match Ks")
	}
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
	ec := saveData.BigXj[ownIdx].Curve()
	if saveData.Xi.Sign() <= 0 || saveData.Xi.Cmp(ec.Params().N) >= 0 {
		return errors.New("ValidateSaveData: Xi is not in [1, q)")
	}
	xiG := crypto.ScalarBaseMult(ec, saveData.Xi)
	if !xiG.Equals(saveData.BigXj[ownIdx]) {
		return errors.New("ValidateSaveData: Feldman VSS check failed: Xi·G != BigXj[ownIndex]")
	}
	// The public shares must interpolate to the group key.
	pub, err := vss.InterpolateAtZero(ec, saveData.Ks, saveData.BigXj)
	if err != nil {
		return fmt.Errorf("ValidateSaveData: %w", err)
	}
	if !pub.Equals(saveData.EDDSAPub) {
		return errors.New("ValidateSaveData: BigXj does not interpolate to EDDSAPub")
	}
	return nil
}

// BuildLocalSaveDataSubset re-creates the LocalPartySaveData for only
// the given signing parties.  It returns an error if a party is not in
// the save data.  Keys are matched by value, so an encoding with
// leading zero bytes matches.  The secrets are copied, so the result
// does not share them with sourceData.
func BuildLocalSaveDataSubset(sourceData LocalPartySaveData, sortedIDs tss.SortedPartyIDs) (LocalPartySaveData, error) {
	keysToIndices := make(map[string]int, len(sourceData.Ks))
	for j, kj := range sourceData.Ks {
		if kj != nil {
			keysToIndices[kj.String()] = j
		}
	}
	newData := NewLocalPartySaveData(sortedIDs.Len())
	newData.LocalSecrets = LocalSecrets{
		Xi:      cloneInt(sourceData.Xi),
		ShareID: cloneInt(sourceData.ShareID),
	}
	newData.EDDSAPub = sourceData.EDDSAPub
	for j, id := range sortedIDs {
		savedIdx, ok := keysToIndices[id.KeyInt().String()]
		if !ok {
			return LocalPartySaveData{}, fmt.Errorf("BuildLocalSaveDataSubset: party %v is not in the local save data", id)
		}
		newData.Ks[j] = sourceData.Ks[savedIdx]
		newData.BigXj[j] = sourceData.BigXj[savedIdx]
	}
	return newData, nil
}

// cloneInt returns a copy of x, or nil for a nil x.
func cloneInt(x *big.Int) *big.Int {
	if x == nil {
		return nil
	}
	return new(big.Int).Set(x)
}
