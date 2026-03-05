package signing

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/ecdsa/keygen"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

// ----- [FORK] Key-at-Index verification tests ----- //

// TestSigningKeyAtIndexRejectsMismatchedKey verifies that ValidateMessage rejects a
// message whose From PartyID has a valid Index but a Key that does not match
// the party registered at that Index in the PeerContext.
func TestSigningKeyAtIndexRejectsMismatchedKey(t *testing.T) {
	// Load keygen fixtures to create a signing party.
	keys, signPIDs, err := keygen.LoadKeygenTestFixturesRandomSet(testThreshold+1, testParticipants)
	assert.NoError(t, err, "should load keygen fixtures")

	ctx := tss.NewPeerContext(signPIDs)
	params := tss.NewParameters(tss.S256(), ctx, signPIDs[0], len(signPIDs), testThreshold)

	outCh := make(chan tss.Message, 10)
	endCh := make(chan *common.SignatureData, 10)
	party := NewLocalParty(big.NewInt(42), params, keys[0], outCh, endCh).(*LocalParty)

	// Construct a fake sender with Index=1 but a wrong Key.
	fakeKey := big.NewInt(999999)
	fakeFrom := tss.NewPartyID("fake", "fake", fakeKey)
	fakeFrom.Index = 1

	// Build a SignRound1Message2 (broadcast) with valid content.
	content := &SignRound1Message2{
		Commitment: big.NewInt(1).Bytes(),
	}
	meta := tss.MessageRouting{
		From:        fakeFrom,
		IsBroadcast: true,
	}
	wire := tss.NewMessageWrapper(meta, content)
	msg := tss.NewMessage(meta, content, wire)

	ok, tssErr := party.ValidateMessage(msg)
	assert.False(t, ok, "ValidateMessage should reject mismatched key")
	assert.Error(t, tssErr, "should return a tss.Error")
	assert.Contains(t, tssErr.Error(), "sender Key does not match",
		"error should mention key mismatch")
}

// ----- [FORK] Duplicate message rejection tests ----- //

// TestSigningStoreMessageRejectsDuplicate verifies that storing the same
// (round, sender) message twice results in a silent drop.
func TestSigningStoreMessageRejectsDuplicate(t *testing.T) {
	keys, signPIDs, err := keygen.LoadKeygenTestFixturesRandomSet(testThreshold+1, testParticipants)
	assert.NoError(t, err, "should load keygen fixtures")

	ctx := tss.NewPeerContext(signPIDs)
	params := tss.NewParameters(tss.S256(), ctx, signPIDs[0], len(signPIDs), testThreshold)

	outCh := make(chan tss.Message, 10)
	endCh := make(chan *common.SignatureData, 10)
	party := NewLocalParty(big.NewInt(42), params, keys[0], outCh, endCh).(*LocalParty)

	sender := signPIDs[1]

	// Build two distinct SignRound1Message2 (broadcast) from the same sender.
	msg1 := NewSignRound1Message2(sender, big.NewInt(1))
	msg2 := NewSignRound1Message2(sender, big.NewInt(2))

	// First store should succeed.
	ok, tssErr := party.StoreMessage(msg1)
	assert.True(t, ok, "first StoreMessage should succeed")
	assert.Nil(t, tssErr, "first StoreMessage should not error")

	// Second store should be silently dropped (true, nil).
	ok, tssErr = party.StoreMessage(msg2)
	assert.True(t, ok, "duplicate StoreMessage should return true (silent drop)")
	assert.Nil(t, tssErr, "duplicate StoreMessage should not error")

	// Verify the stored message is still the original.
	stored := party.temp.signRound1Message2s[sender.Index]
	assert.NotNil(t, stored)
	content := stored.Content().(*SignRound1Message2)
	assert.Equal(t, big.NewInt(1).Bytes(), content.GetCommitment(),
		"stored message should be the original, not the duplicate")
}

// ----- [FORK] Broadcast/P2P flag validation tests ----- //

// TestSigningStoreMessageRejectsWrongBroadcastFlag verifies that a SignRound1Message1
// (which is a P2P message) is rejected when sent with IsBroadcast=true.
func TestSigningStoreMessageRejectsWrongBroadcastFlag(t *testing.T) {
	keys, signPIDs, err := keygen.LoadKeygenTestFixturesRandomSet(testThreshold+1, testParticipants)
	assert.NoError(t, err, "should load keygen fixtures")

	ctx := tss.NewPeerContext(signPIDs)
	params := tss.NewParameters(tss.S256(), ctx, signPIDs[0], len(signPIDs), testThreshold)

	outCh := make(chan tss.Message, 10)
	endCh := make(chan *common.SignatureData, 10)
	party := NewLocalParty(big.NewInt(42), params, keys[0], outCh, endCh).(*LocalParty)

	sender := signPIDs[1]

	// SignRound3Message is broadcast. Construct it with IsBroadcast=false (wrong).
	broadcastContent := &SignRound3Message{
		Theta: big.NewInt(42).Bytes(),
	}
	meta := tss.MessageRouting{
		From:        sender,
		IsBroadcast: false, // wrong: SignRound3Message is broadcast
	}
	wire := tss.NewMessageWrapper(meta, broadcastContent)
	msg := tss.NewMessage(meta, broadcastContent, wire)

	ok, tssErr := party.StoreMessage(msg)
	assert.False(t, ok, "StoreMessage should reject broadcast msg sent as P2P")
	assert.Error(t, tssErr, "should return an error")
	assert.Contains(t, tssErr.Error(), "expected broadcast but got P2P",
		"error should mention broadcast/P2P mismatch")
}

// TestSigningNilMsgPanics verifies that NewLocalParty panics when given a nil message.
func TestSigningNilMsgPanics(t *testing.T) {
	keys, signPIDs, err := keygen.LoadKeygenTestFixturesRandomSet(testThreshold+1, testParticipants)
	assert.NoError(t, err, "should load keygen fixtures")

	ctx := tss.NewPeerContext(signPIDs)
	params := tss.NewParameters(tss.S256(), ctx, signPIDs[0], len(signPIDs), testThreshold)

	outCh := make(chan tss.Message, 10)
	endCh := make(chan *common.SignatureData, 10)

	assert.Panics(t, func() {
		NewLocalParty(nil, params, keys[0], outCh, endCh)
	}, "NewLocalParty with nil msg should panic")
}
