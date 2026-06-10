// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/hemilabs/x/tss/v3/crypto"
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

// BuildLocalSaveDataSubset re-creates the LocalPartySaveData for only
// the given signing parties.
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
