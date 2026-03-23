// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package resharing

import (
	"math/big"

	"github.com/hemilabs/x/tss-lib/v3/crypto"
	"github.com/hemilabs/x/tss-lib/v3/crypto/vss"
	"github.com/hemilabs/x/tss-lib/v3/eddsa/keygen"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// TaskName identifies the EdDSA resharing protocol in error messages.
const TaskName = "eddsa-resharing"

type localTempData struct {
	localMessageStore

	ssidNonce *big.Int
	ssid      []byte

	// Round 1 (old committee)
	VD        []*big.Int // decommitment
	NewShares vss.Shares // shares for new committee

	// Round 4 (new committee)
	newXi     *big.Int
	newKs     []*big.Int
	newBigXjs []*crypto.ECPoint
}

type localMessageStore struct {
	dgRound1Messages  []*tss.Message
	dgRound2Messages  []*tss.Message
	dgRound3Message1s []*tss.Message
	dgRound3Message2s []*tss.Message
	dgRound4Messages  []*tss.Message
}

// ReshareState holds mutable state across all resharing rounds.
type ReshareState struct {
	params *tss.ReSharingParameters
	input  *keygen.LocalPartySaveData
	save   *keygen.LocalPartySaveData
	temp   localTempData
}

// ReshareRoundOutput is returned by each round function.
type ReshareRoundOutput struct {
	// Messages to send to other parties.
	Messages []*tss.Message
	// Save is non-nil only after the final round (new committee only).
	Save *keygen.LocalPartySaveData
}
