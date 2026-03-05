// Copyright (c) 2024 Hemi Labs, Inc.
//
// This file is part of the hemi tss-lib fork. See LICENSE for terms.

package resharing

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hemilabs/x/tss-lib/v2/eddsa/keygen"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

// TestEdDSAResharingKeyAtIndexRejectsMismatchedKey verifies the [FORK] Key-at-Index
// check in ValidateMessage: a message whose From has a valid Index but a Key that
// does not match the party registered at that Index must be rejected.
// DGRound1Message is from the old committee, so the lookup uses OldParties().
func TestEdDSAResharingKeyAtIndexRejectsMismatchedKey(t *testing.T) {
	oldN, oldT, newN, newT := 3, 1, 3, 1

	oldPartyIDs := tss.GenerateTestPartyIDs(oldN)
	oldCtx := tss.NewPeerContext(oldPartyIDs)
	newPartyIDs := tss.GenerateTestPartyIDs(newN)
	newCtx := tss.NewPeerContext(newPartyIDs)

	// Use a new-committee party as the local party (avoids BuildLocalSaveDataSubset
	// which requires populated save data for old-committee members).
	params := tss.NewReSharingParameters(tss.Edwards(), oldCtx, newCtx, newPartyIDs[0], oldN, oldT, newN, newT)

	outCh := make(chan tss.Message, 10)
	endCh := make(chan *keygen.LocalPartySaveData, 10)
	save := keygen.NewLocalPartySaveData(newN)
	party := NewLocalParty(params, save, outCh, endCh).(*LocalParty)

	// Build a fake sender with valid Index=1 in old committee but wrong Key.
	fakeKey := big.NewInt(999999)
	fakeFrom := tss.NewPartyID("fake", "fake", fakeKey)
	fakeFrom.Index = 1

	// DGRound1Message is broadcast from old committee.
	content := &DGRound1Message{
		EddsaPubX:   []byte{0x01},
		EddsaPubY:   []byte{0x01},
		VCommitment: []byte{0x01},
	}
	routing := tss.MessageRouting{From: fakeFrom, IsBroadcast: true}
	wire := tss.NewMessageWrapper(routing, content)
	msg := tss.NewMessage(routing, content, wire)

	ok, err := party.ValidateMessage(msg)
	assert.False(t, ok, "ValidateMessage should reject mismatched key")
	assert.NotNil(t, err, "error should be non-nil")
	assert.Contains(t, err.Error(), "sender Key does not match")
}

// TestEdDSAResharingStoreMessageRejectsDuplicate verifies the [FORK] duplicate
// message rejection: storing the same (round, sender) DGRound1Message twice must
// silently drop the second one (return true, nil).
func TestEdDSAResharingStoreMessageRejectsDuplicate(t *testing.T) {
	oldN, oldT, newN, newT := 3, 1, 3, 1

	oldPartyIDs := tss.GenerateTestPartyIDs(oldN)
	oldCtx := tss.NewPeerContext(oldPartyIDs)
	newPartyIDs := tss.GenerateTestPartyIDs(newN)
	newCtx := tss.NewPeerContext(newPartyIDs)

	// Use a new-committee party as the local party.
	params := tss.NewReSharingParameters(tss.Edwards(), oldCtx, newCtx, newPartyIDs[0], oldN, oldT, newN, newT)

	outCh := make(chan tss.Message, 10)
	endCh := make(chan *keygen.LocalPartySaveData, 10)
	save := keygen.NewLocalPartySaveData(newN)
	party := NewLocalParty(params, save, outCh, endCh).(*LocalParty)

	// Build a valid DGRound1Message from old party at index 1 (broadcast).
	from := oldPartyIDs[1]
	content := &DGRound1Message{
		EddsaPubX:   []byte{0x01},
		EddsaPubY:   []byte{0x01},
		VCommitment: []byte{0x01},
	}
	routing := tss.MessageRouting{From: from, IsBroadcast: true}
	wire := tss.NewMessageWrapper(routing, content)
	msg := tss.NewMessage(routing, content, wire)

	// First store: accepted.
	ok, err := party.StoreMessage(msg)
	assert.True(t, ok, "first store should succeed")
	assert.Nil(t, err, "first store should have no error")

	// Second store: duplicate silently dropped.
	ok2, err2 := party.StoreMessage(msg)
	assert.True(t, ok2, "duplicate store should return true (silently dropped)")
	assert.Nil(t, err2, "duplicate store should have no error")
}

// TestEdDSAResharingStoreMessageRejectsWrongBroadcastFlag verifies the [FORK]
// broadcast/P2P flag validation: DGRound3Message1 is P2P, so sending it as
// broadcast must be rejected.
func TestEdDSAResharingStoreMessageRejectsWrongBroadcastFlag(t *testing.T) {
	oldN, oldT, newN, newT := 3, 1, 3, 1

	oldPartyIDs := tss.GenerateTestPartyIDs(oldN)
	oldCtx := tss.NewPeerContext(oldPartyIDs)
	newPartyIDs := tss.GenerateTestPartyIDs(newN)
	newCtx := tss.NewPeerContext(newPartyIDs)

	// Use a new-committee party as the local party.
	params := tss.NewReSharingParameters(tss.Edwards(), oldCtx, newCtx, newPartyIDs[0], oldN, oldT, newN, newT)

	outCh := make(chan tss.Message, 10)
	endCh := make(chan *keygen.LocalPartySaveData, 10)
	save := keygen.NewLocalPartySaveData(newN)
	party := NewLocalParty(params, save, outCh, endCh).(*LocalParty)

	// DGRound3Message1 is P2P from old committee. Send as broadcast (wrong flag).
	from := oldPartyIDs[1]
	content := &DGRound3Message1{
		Share:      []byte{0x01},
		ReceiverId: []byte{0x01, 0x02},
	}
	routing := tss.MessageRouting{From: from, IsBroadcast: true}
	wire := tss.NewMessageWrapper(routing, content)
	msg := tss.NewMessage(routing, content, wire)

	ok, err := party.StoreMessage(msg)
	assert.False(t, ok, "store with wrong broadcast flag should fail")
	assert.NotNil(t, err, "error should be non-nil")
	assert.Contains(t, err.Error(), "expected P2P but got broadcast")
}
