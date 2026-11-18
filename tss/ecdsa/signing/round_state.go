// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package signing

import (
	"math/big"

	"github.com/hemilabs/x/tss/v3/ecdsa/keygen"
	"github.com/hemilabs/x/tss/v3/tss"
)

// SigningState holds all mutable state between signing rounds.
// Opaque to the caller.
type SigningState struct {
	life tss.Lifecycle // round order and failure latch

	params *tss.Parameters
	key    *keygen.LocalPartySaveData
	data   *SignatureData
	temp   *localTempData
}

// SignRoundOutput holds the outbound messages and artifacts
// produced by a single signing round function.
type SignRoundOutput struct {
	// Messages to send.  Broadcast: GetTo() == nil.
	Messages []*tss.Message

	// Signature is non-nil only after Finalize.
	Signature *SignatureData
}

// wipeNonces clears the nonce secrets after a failed round, so that a
// retry cannot use them again.
func (s *SigningState) wipeNonces() {
	s.temp.k = new(big.Int)
	s.temp.gamma = new(big.Int)
	s.temp.w = new(big.Int)
	s.temp.sigma = new(big.Int)
}
