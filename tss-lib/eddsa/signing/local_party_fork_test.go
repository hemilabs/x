// Copyright (c) 2024 Hemi Labs, Inc.
//
// This file is part of the hemi tss-lib fork. See LICENSE for terms.

package signing

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/eddsa/keygen"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

// TestEdDSASigningKeyAtIndexRejectsMismatchedKey verifies the [FORK] Key-at-Index
// check in ValidateMessage: a message whose From has a valid Index but a Key that
// does not match the party registered at that Index must be rejected.
func TestEdDSASigningKeyAtIndexRejectsMismatchedKey(t *testing.T) {
	// Load EdDSA keygen fixtures to get valid save data.
	keys, signPIDs, err := keygen.LoadKeygenTestFixturesRandomSet(testThreshold+1, testParticipants)
	assert.NoError(t, err, "should load keygen fixtures")

	ctx := tss.NewPeerContext(signPIDs)
	params := tss.NewParameters(tss.Edwards(), ctx, signPIDs[0], len(signPIDs), testThreshold)

	outCh := make(chan tss.Message, 10)
	endCh := make(chan *common.SignatureData, 10)
	party := NewLocalParty(big.NewInt(42), params, keys[0], outCh, endCh).(*LocalParty)

	// Build a fake sender with valid Index=1 but wrong Key.
	fakeKey := big.NewInt(999999)
	fakeFrom := tss.NewPartyID("fake", "fake", fakeKey)
	fakeFrom.Index = 1

	// Construct a valid SignRound1Message from the fake sender.
	content := &SignRound1Message{Commitment: []byte{0x01}}
	routing := tss.MessageRouting{From: fakeFrom, IsBroadcast: true}
	wire := tss.NewMessageWrapper(routing, content)
	msg := tss.NewMessage(routing, content, wire)

	ok, vErr := party.ValidateMessage(msg)
	assert.False(t, ok, "ValidateMessage should reject mismatched key")
	assert.NotNil(t, vErr, "error should be non-nil")
	assert.Contains(t, vErr.Error(), "sender Key does not match")
}

// TestEdDSASigningStoreMessageRejectsDuplicate verifies the [FORK] duplicate
// message rejection: storing the same (round, sender) message twice must silently
// drop the second one (return true, nil).
func TestEdDSASigningStoreMessageRejectsDuplicate(t *testing.T) {
	keys, signPIDs, err := keygen.LoadKeygenTestFixturesRandomSet(testThreshold+1, testParticipants)
	assert.NoError(t, err, "should load keygen fixtures")

	ctx := tss.NewPeerContext(signPIDs)
	params := tss.NewParameters(tss.Edwards(), ctx, signPIDs[0], len(signPIDs), testThreshold)

	outCh := make(chan tss.Message, 10)
	endCh := make(chan *common.SignatureData, 10)
	party := NewLocalParty(big.NewInt(42), params, keys[0], outCh, endCh).(*LocalParty)

	// Build a valid SignRound1Message from party at index 1 (broadcast).
	from := signPIDs[1]
	content := &SignRound1Message{Commitment: []byte{0x01}}
	routing := tss.MessageRouting{From: from, IsBroadcast: true}
	wire := tss.NewMessageWrapper(routing, content)
	msg := tss.NewMessage(routing, content, wire)

	// First store: accepted.
	ok, sErr := party.StoreMessage(msg)
	assert.True(t, ok, "first store should succeed")
	assert.Nil(t, sErr, "first store should have no error")

	// Second store: duplicate silently dropped.
	ok2, sErr2 := party.StoreMessage(msg)
	assert.True(t, ok2, "duplicate store should return true (silently dropped)")
	assert.Nil(t, sErr2, "duplicate store should have no error")
}

// TestEdDSASigningStoreMessageRejectsWrongBroadcastFlag verifies the [FORK]
// broadcast/P2P flag validation: SignRound1Message is broadcast, so sending it
// as P2P must be rejected.
func TestEdDSASigningStoreMessageRejectsWrongBroadcastFlag(t *testing.T) {
	keys, signPIDs, err := keygen.LoadKeygenTestFixturesRandomSet(testThreshold+1, testParticipants)
	assert.NoError(t, err, "should load keygen fixtures")

	ctx := tss.NewPeerContext(signPIDs)
	params := tss.NewParameters(tss.Edwards(), ctx, signPIDs[0], len(signPIDs), testThreshold)

	outCh := make(chan tss.Message, 10)
	endCh := make(chan *common.SignatureData, 10)
	party := NewLocalParty(big.NewInt(42), params, keys[0], outCh, endCh).(*LocalParty)

	// Build a SignRound1Message but mark as P2P (wrong flag).
	from := signPIDs[1]
	content := &SignRound1Message{Commitment: []byte{0x01}}
	routing := tss.MessageRouting{From: from, IsBroadcast: false}
	wire := tss.NewMessageWrapper(routing, content)
	msg := tss.NewMessage(routing, content, wire)

	ok, sErr := party.StoreMessage(msg)
	assert.False(t, ok, "store with wrong broadcast flag should fail")
	assert.NotNil(t, sErr, "error should be non-nil")
	assert.Contains(t, sErr.Error(), "expected broadcast but got P2P")
}

// TestEdDSASigningNilMsgPanics verifies the [FORK] nil msg guard in
// NewLocalParty: passing nil as the message must panic immediately.
func TestEdDSASigningNilMsgPanics(t *testing.T) {
	keys, signPIDs, err := keygen.LoadKeygenTestFixturesRandomSet(testThreshold+1, testParticipants)
	assert.NoError(t, err, "should load keygen fixtures")

	ctx := tss.NewPeerContext(signPIDs)
	params := tss.NewParameters(tss.Edwards(), ctx, signPIDs[0], len(signPIDs), testThreshold)

	outCh := make(chan tss.Message, 10)
	endCh := make(chan *common.SignatureData, 10)

	assert.Panics(t, func() {
		NewLocalParty(nil, params, keys[0], outCh, endCh)
	}, "NewLocalParty with nil msg should panic")
}
