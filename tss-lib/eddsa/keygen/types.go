// Copyright © 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"math/big"

	"github.com/hemilabs/x/tss-lib/v3/crypto"
	"github.com/hemilabs/x/tss-lib/v3/crypto/vss"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// TaskName identifies the EdDSA keygen protocol in error messages.
const TaskName = "eddsa-keygen"

type localTempData struct {
	localMessageStore

	ssidNonce *big.Int
	ssid      []byte

	// round 1 data
	ui     *big.Int
	vs     vss.Vs
	shares vss.Shares

	deCommitPolyG []*big.Int
}

type localMessageStore struct {
	kgRound1Messages  []*tss.Message
	kgRound2Message1s []*tss.Message
	kgRound2Message2s []*tss.Message
}

// LocalSecrets holds the party's secret key material.
type LocalSecrets struct {
	Xi, ShareID *big.Int
}

// LocalPartySaveData holds the complete keygen output for one party.
type LocalPartySaveData struct {
	LocalSecrets

	Ks    []*big.Int
	BigXj []*crypto.ECPoint
	// EDDSAPub is the distributed EdDSA public key.
	EDDSAPub *crypto.ECPoint
}

// NewLocalPartySaveData allocates a LocalPartySaveData with slices sized for partyCount.
func NewLocalPartySaveData(partyCount int) (saveData LocalPartySaveData) {
	saveData.Ks = make([]*big.Int, partyCount)
	saveData.BigXj = make([]*crypto.ECPoint, partyCount)
	return
}
