// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"github.com/hemilabs/x/tss/v3/crypto/vss"
	"github.com/hemilabs/x/tss/v3/tss"
)

// KeygenState holds mutable state across all keygen rounds for one party.
type KeygenState struct {
	params *tss.Parameters
	save   LocalPartySaveData
	temp   localTempData
}

// RoundOutput is returned by each round function.
type RoundOutput struct {
	// Messages to send to other parties.
	Messages []*tss.Message
	// Save is non-nil only after the final round.
	Save *LocalPartySaveData
	// Poly is the VSS polynomial (available after Round1 for SNARK witness).
	Poly vss.Vs
}

// ExportR2P2PSelf returns the party's own P2P Round 2 message.
func (s *KeygenState) ExportR2P2PSelf() *tss.Message {
	i := s.params.PartyID().Index
	return s.temp.kgRound2Message1s[i]
}

// ExportR2BcastSelf returns the party's own broadcast Round 2 message.
func (s *KeygenState) ExportR2BcastSelf() *tss.Message {
	i := s.params.PartyID().Index
	return s.temp.kgRound2Message2s[i]
}
