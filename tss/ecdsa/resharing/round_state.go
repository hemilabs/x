// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package resharing

import (
	"math/big"

	"github.com/hemilabs/x/tss-lib/v3/crypto"
	"github.com/hemilabs/x/tss-lib/v3/ecdsa/keygen"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// ReshareState holds all mutable state between resharing rounds.
// A single node may be in both the old and new committee; the
// ReSharingParameters encode which roles this party plays.
type ReshareState struct {
	params *tss.ReSharingParameters
	input  *keygen.LocalPartySaveData // existing key (old committee)
	save   *keygen.LocalPartySaveData // new key being built
	temp   *localTempData
}

// ReshareRoundOutput holds the outbound messages and artifacts
// produced by a single resharing round function.
type ReshareRoundOutput struct {
	// Messages to send.
	Messages []*tss.Message

	// Save is non-nil only after ReshareRound5 (final new-committee round).
	Save *keygen.LocalPartySaveData

	// Poly contains the VSS polynomial coefficients for SNARK
	// witness extraction.  Non-nil only on ReshareRound1 (old committee).
	Poly []*big.Int

	// NewVs contains Feldman VSS commitments for SNARK witness.
	// Non-nil only on ReshareRound1 (old committee).
	NewVs []*crypto.ECPoint
}
