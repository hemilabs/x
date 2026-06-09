// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package tss

type (
	PeerContext struct {
		partyIDs SortedPartyIDs
	}
)

// NewPeerContext creates a peer context from sorted party IDs.
func NewPeerContext(parties SortedPartyIDs) *PeerContext {
	return &PeerContext{partyIDs: parties}
}

// IDs returns the sorted party IDs in this peer context.
func (p2pCtx *PeerContext) IDs() SortedPartyIDs {
	return p2pCtx.partyIDs
}

// SetIDs replaces the party IDs in this peer context.
func (p2pCtx *PeerContext) SetIDs(ids SortedPartyIDs) {
	p2pCtx.partyIDs = ids
}
