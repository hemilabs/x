// Copyright (c) 2024 Hemi Labs, Inc.
//
// This file is part of the hemi tss-lib fork. See LICENSE for terms.

package keygen

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hemilabs/x/tss-lib/v2/tss"
)

// TestEdDSAKeygenKeyAtIndexRejectsMismatchedKey verifies the [FORK] Key-at-Index
// check in ValidateMessage: a message whose From has a valid Index but a Key that
// does not match the party registered at that Index must be rejected.
func TestEdDSAKeygenKeyAtIndexRejectsMismatchedKey(t *testing.T) {
	partyIDs := tss.GenerateTestPartyIDs(3)
	ctx := tss.NewPeerContext(partyIDs)
	params := tss.NewParameters(tss.Edwards(), ctx, partyIDs[0], 3, 1)

	outCh := make(chan tss.Message, 10)
	endCh := make(chan *LocalPartySaveData, 10)
	party := NewLocalParty(params, outCh, endCh).(*LocalParty)

	// Build a fake sender with valid Index=1 but wrong Key.
	fakeKey := big.NewInt(999999)
	fakeFrom := tss.NewPartyID("fake", "fake", fakeKey)
	fakeFrom.Index = 1

	// Construct a valid KGRound1Message from the fake sender.
	content := &KGRound1Message{Commitment: []byte{0x01}}
	routing := tss.MessageRouting{From: fakeFrom, IsBroadcast: true}
	wire := tss.NewMessageWrapper(routing, content)
	msg := tss.NewMessage(routing, content, wire)

	ok, err := party.ValidateMessage(msg)
	assert.False(t, ok, "ValidateMessage should reject mismatched key")
	assert.NotNil(t, err, "error should be non-nil")
	assert.Contains(t, err.Error(), "sender Key does not match")
}

// TestEdDSAKeygenStoreMessageRejectsDuplicate verifies the [FORK] duplicate
// message rejection: storing the same (round, sender) message twice must silently
// drop the second one (return true, nil).
func TestEdDSAKeygenStoreMessageRejectsDuplicate(t *testing.T) {
	partyIDs := tss.GenerateTestPartyIDs(3)
	ctx := tss.NewPeerContext(partyIDs)
	params := tss.NewParameters(tss.Edwards(), ctx, partyIDs[0], 3, 1)

	outCh := make(chan tss.Message, 10)
	endCh := make(chan *LocalPartySaveData, 10)
	party := NewLocalParty(params, outCh, endCh).(*LocalParty)

	// Build a valid KGRound1Message from party 1 (broadcast).
	from := partyIDs[1]
	content := &KGRound1Message{Commitment: []byte{0x01}}
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

// TestEdDSAKeygenStoreMessageRejectsWrongBroadcastFlag verifies the [FORK]
// broadcast/P2P flag validation: KGRound1Message is broadcast, so sending it
// as P2P must be rejected.
func TestEdDSAKeygenStoreMessageRejectsWrongBroadcastFlag(t *testing.T) {
	partyIDs := tss.GenerateTestPartyIDs(3)
	ctx := tss.NewPeerContext(partyIDs)
	params := tss.NewParameters(tss.Edwards(), ctx, partyIDs[0], 3, 1)

	outCh := make(chan tss.Message, 10)
	endCh := make(chan *LocalPartySaveData, 10)
	party := NewLocalParty(params, outCh, endCh).(*LocalParty)

	// Build a KGRound1Message but mark as P2P (wrong flag).
	from := partyIDs[1]
	content := &KGRound1Message{Commitment: []byte{0x01}}
	routing := tss.MessageRouting{From: from, IsBroadcast: false}
	wire := tss.NewMessageWrapper(routing, content)
	msg := tss.NewMessage(routing, content, wire)

	ok, err := party.StoreMessage(msg)
	assert.False(t, ok, "store with wrong broadcast flag should fail")
	assert.NotNil(t, err, "error should be non-nil")
	assert.Contains(t, err.Error(), "expected broadcast but got P2P")
}
