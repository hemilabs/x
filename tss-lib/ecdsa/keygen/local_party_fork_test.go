package keygen

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hemilabs/x/tss-lib/v2/crypto/paillier"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

// ----- [FORK] Key-at-Index verification tests ----- //

// TestKeyAtIndexRejectsMismatchedKey verifies that ValidateMessage rejects a
// message whose From PartyID has a valid Index but a Key that does not match
// the party registered at that Index in the PeerContext.
func TestKeyAtIndexRejectsMismatchedKey(t *testing.T) {
	pIDs := tss.GenerateTestPartyIDs(3)
	ctx := tss.NewPeerContext(pIDs)
	params := tss.NewParameters(tss.S256(), ctx, pIDs[0], 3, 1)

	outCh := make(chan tss.Message, 10)
	endCh := make(chan *LocalPartySaveData, 10)
	party := NewLocalParty(params, outCh, endCh).(*LocalParty)

	// Construct a valid KGRound1Message from a fake sender that has Index=1
	// but a different Key than what is registered at index 1.
	fakeKey := big.NewInt(999999)
	fakeFrom := tss.NewPartyID("fake", "fake", fakeKey)
	fakeFrom.Index = 1 // valid index, wrong key

	// Build a message with valid content that passes ValidateBasic.
	msg, err := NewKGRound1Message(
		fakeFrom,
		big.NewInt(1), // commitment (non-empty)
		&paillier.PublicKey{N: big.NewInt(12345)},
		big.NewInt(100), // NTilde
		big.NewInt(200), // H1
		big.NewInt(300), // H2
		nil,             // no DLN proof (SNARK mode)
		nil,
	)
	assert.NoError(t, err)

	ok, tssErr := party.ValidateMessage(msg)
	assert.False(t, ok, "ValidateMessage should reject mismatched key")
	assert.Error(t, tssErr, "should return a tss.Error")
	assert.Contains(t, tssErr.Error(), "sender Key does not match",
		"error should mention key mismatch")
}

// ----- [FORK] Duplicate message rejection tests ----- //

// TestStoreMessageRejectsDuplicate verifies that storing the same (round, sender)
// message twice results in a silent drop (returns true, nil) without overwriting
// the original stored message.
func TestStoreMessageRejectsDuplicate(t *testing.T) {
	pIDs := tss.GenerateTestPartyIDs(3)
	ctx := tss.NewPeerContext(pIDs)
	params := tss.NewParameters(tss.S256(), ctx, pIDs[0], 3, 1)

	outCh := make(chan tss.Message, 10)
	endCh := make(chan *LocalPartySaveData, 10)
	party := NewLocalParty(params, outCh, endCh).(*LocalParty)

	// Build two distinct KGRound1Messages from party index 1.
	sender := pIDs[1]
	msg1, err := NewKGRound1Message(
		sender,
		big.NewInt(1),
		&paillier.PublicKey{N: big.NewInt(12345)},
		big.NewInt(100),
		big.NewInt(200),
		big.NewInt(300),
		nil,
		nil,
	)
	assert.NoError(t, err)

	msg2, err := NewKGRound1Message(
		sender,
		big.NewInt(2), // different commitment
		&paillier.PublicKey{N: big.NewInt(12345)},
		big.NewInt(100),
		big.NewInt(200),
		big.NewInt(300),
		nil,
		nil,
	)
	assert.NoError(t, err)

	// First store should succeed.
	ok, tssErr := party.StoreMessage(msg1)
	assert.True(t, ok, "first StoreMessage should succeed")
	assert.Nil(t, tssErr, "first StoreMessage should not error")

	// Second store should be silently dropped (true, nil).
	ok, tssErr = party.StoreMessage(msg2)
	assert.True(t, ok, "duplicate StoreMessage should return true (silent drop)")
	assert.Nil(t, tssErr, "duplicate StoreMessage should not error")

	// Verify the stored message is still the original (commitment=1), not the duplicate (commitment=2).
	stored := party.temp.kgRound1Messages[sender.Index]
	assert.NotNil(t, stored)
	content := stored.Content().(*KGRound1Message)
	assert.Equal(t, big.NewInt(1).Bytes(), content.GetCommitment(),
		"stored message should be the original, not the duplicate")
}

// ----- [FORK] Broadcast/P2P flag validation tests ----- //

// TestStoreMessageRejectsWrongBroadcastFlag verifies that a KGRound1Message
// (which is a broadcast message) is rejected when sent with IsBroadcast=false.
func TestStoreMessageRejectsWrongBroadcastFlag(t *testing.T) {
	pIDs := tss.GenerateTestPartyIDs(3)
	ctx := tss.NewPeerContext(pIDs)
	params := tss.NewParameters(tss.S256(), ctx, pIDs[0], 3, 1)

	outCh := make(chan tss.Message, 10)
	endCh := make(chan *LocalPartySaveData, 10)
	party := NewLocalParty(params, outCh, endCh).(*LocalParty)

	sender := pIDs[1]
	content := &KGRound1Message{
		Commitment: big.NewInt(1).Bytes(),
		PaillierN:  big.NewInt(12345).Bytes(),
		NTilde:     big.NewInt(100).Bytes(),
		H1:         big.NewInt(200).Bytes(),
		H2:         big.NewInt(300).Bytes(),
	}
	// Construct with IsBroadcast=false (wrong for KGRound1Message).
	meta := tss.MessageRouting{
		From:        sender,
		IsBroadcast: false,
	}
	wire := tss.NewMessageWrapper(meta, content)
	msg := tss.NewMessage(meta, content, wire)

	ok, tssErr := party.StoreMessage(msg)
	assert.False(t, ok, "StoreMessage should reject broadcast msg sent as P2P")
	assert.Error(t, tssErr, "should return an error")
	assert.Contains(t, tssErr.Error(), "expected broadcast but got P2P",
		"error should mention broadcast/P2P mismatch")
}

// TestStoreMessageRejectsP2PAsBroadcast verifies that a KGRound2Message1
// (which is a P2P message) is rejected when sent with IsBroadcast=true.
func TestStoreMessageRejectsP2PAsBroadcast(t *testing.T) {
	pIDs := tss.GenerateTestPartyIDs(3)
	ctx := tss.NewPeerContext(pIDs)
	params := tss.NewParameters(tss.S256(), ctx, pIDs[0], 3, 1)

	outCh := make(chan tss.Message, 10)
	endCh := make(chan *LocalPartySaveData, 10)
	party := NewLocalParty(params, outCh, endCh).(*LocalParty)

	sender := pIDs[1]
	content := &KGRound2Message1{
		Share:      big.NewInt(42).Bytes(),
		ReceiverId: pIDs[0].GetKey(),
	}
	// Construct with IsBroadcast=true (wrong for KGRound2Message1).
	meta := tss.MessageRouting{
		From:        sender,
		IsBroadcast: true,
	}
	wire := tss.NewMessageWrapper(meta, content)
	msg := tss.NewMessage(meta, content, wire)

	ok, tssErr := party.StoreMessage(msg)
	assert.False(t, ok, "StoreMessage should reject P2P msg sent as broadcast")
	assert.Error(t, tssErr, "should return an error")
	assert.Contains(t, tssErr.Error(), "expected P2P but got broadcast",
		"error should mention P2P/broadcast mismatch")
}
