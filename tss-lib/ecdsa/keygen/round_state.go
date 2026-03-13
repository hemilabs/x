// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"math/big"

	"github.com/hemilabs/x/tss-lib/v2/tss"
)

// KeygenState holds all mutable state between keygen rounds.
// Opaque to the caller — pass between round functions without
// reading or modifying.
type KeygenState struct {
	params *tss.Parameters
	save   *LocalPartySaveData
	temp   *localTempData
}

// RoundOutput holds the outbound messages and artifacts produced
// by a single round function.
type RoundOutput struct {
	// Messages to send to other parties.  Broadcast messages
	// have GetTo() == nil; P2P messages have one recipient.
	Messages []tss.Message

	// Save is non-nil only on the final round (Round4).
	// Contains the complete key share data.
	Save *LocalPartySaveData

	// Poly contains the VSS polynomial coefficients for SNARK
	// witness extraction.  Non-nil only on Round1.
	Poly []*big.Int
}

// ExportR2P2PSelf returns this party's own Round2 P2P message (stored
// during Round2 for self-delivery).  Needed by the signing test to
// build the full message matrix without channels.
func (s *KeygenState) ExportR2P2PSelf() tss.ParsedMessage {
	i := s.params.PartyID().Index
	return s.temp.kgRound2Message1s[i]
}

// ExportR2BcastSelf returns this party's own Round2 broadcast message.
func (s *KeygenState) ExportR2BcastSelf() tss.ParsedMessage {
	i := s.params.PartyID().Index
	return s.temp.kgRound2Message2s[i]
}
