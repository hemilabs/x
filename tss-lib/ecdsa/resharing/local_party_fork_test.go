package resharing

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hemilabs/x/tss-lib/v2/crypto"
	"github.com/hemilabs/x/tss-lib/v2/ecdsa/keygen"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

// helper builds a minimal new-committee resharing party for fork tests.
// Uses 3 old parties and 3 new parties with threshold=1.
// The party is placed in the NEW committee to avoid BuildLocalSaveDataSubset
// (which requires populated Ks in the save data).
func newTestResharingParty(t *testing.T) (*LocalParty, tss.SortedPartyIDs, tss.SortedPartyIDs) {
	t.Helper()

	oldPIDs := tss.GenerateTestPartyIDs(3)
	newPIDs := tss.GenerateTestPartyIDs(3)
	oldCtx := tss.NewPeerContext(oldPIDs)
	newCtx := tss.NewPeerContext(newPIDs)

	// Use newPIDs[0] as the party ID so IsOldCommittee() returns false,
	// avoiding the BuildLocalSaveDataSubset call on empty save data.
	params := tss.NewReSharingParameters(tss.S256(), oldCtx, newCtx, newPIDs[0], 3, 1, 3, 1)

	outCh := make(chan tss.Message, 10)
	endCh := make(chan *keygen.LocalPartySaveData, 10)
	save := keygen.NewLocalPartySaveData(3)
	party := NewLocalParty(params, save, outCh, endCh).(*LocalParty)
	return party, oldPIDs, newPIDs
}

// ----- [FORK] Key-at-Index verification tests ----- //

// TestResharingKeyAtIndexRejectsMismatchedKey verifies that ValidateMessage rejects
// a DGRound1Message whose From PartyID has a valid Index but a Key that does not
// match the party registered at that Index in the old committee PeerContext.
func TestResharingKeyAtIndexRejectsMismatchedKey(t *testing.T) {
	party, oldPIDs, _ := newTestResharingParty(t)

	// Construct a fake sender with Index=1 (old committee) but wrong Key.
	fakeKey := big.NewInt(999999)
	fakeFrom := tss.NewPartyID("fake", "fake", fakeKey)
	fakeFrom.Index = 1

	// Build a DGRound1Message (broadcast, from old committee).
	// Need a valid ECDSAPub point on the curve.
	ec := tss.S256()
	ecdsaPub := crypto.ScalarBaseMult(ec, big.NewInt(42))

	content := &DGRound1Message{
		EcdsaPubX:   ecdsaPub.X().Bytes(),
		EcdsaPubY:   ecdsaPub.Y().Bytes(),
		VCommitment: big.NewInt(1).Bytes(),
		Ssid:        []byte("test-ssid"),
	}
	meta := tss.MessageRouting{
		From:        fakeFrom,
		To:          oldPIDs,
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

// TestResharingStoreMessageRejectsDuplicate verifies that storing the same
// (round, sender) DGRound1Message twice results in a silent drop.
func TestResharingStoreMessageRejectsDuplicate(t *testing.T) {
	party, _, _ := newTestResharingParty(t)

	ec := tss.S256()
	ecdsaPub := crypto.ScalarBaseMult(ec, big.NewInt(42))

	// Use the actual old party at index 1 as the sender.
	sender := party.params.OldParties().IDs()[1]

	buildMsg := func(commitVal int64) tss.ParsedMessage {
		content := &DGRound1Message{
			EcdsaPubX:   ecdsaPub.X().Bytes(),
			EcdsaPubY:   ecdsaPub.Y().Bytes(),
			VCommitment: big.NewInt(commitVal).Bytes(),
			Ssid:        []byte("test-ssid"),
		}
		meta := tss.MessageRouting{
			From:        sender,
			IsBroadcast: true,
		}
		wire := tss.NewMessageWrapper(meta, content)
		return tss.NewMessage(meta, content, wire)
	}

	msg1 := buildMsg(1)
	msg2 := buildMsg(2)

	// First store should succeed.
	ok, tssErr := party.StoreMessage(msg1)
	assert.True(t, ok, "first StoreMessage should succeed")
	assert.Nil(t, tssErr, "first StoreMessage should not error")

	// Second store should be silently dropped (true, nil).
	ok, tssErr = party.StoreMessage(msg2)
	assert.True(t, ok, "duplicate StoreMessage should return true (silent drop)")
	assert.Nil(t, tssErr, "duplicate StoreMessage should not error")

	// Verify the stored message is still the original.
	stored := party.temp.dgRound1Messages[sender.Index]
	assert.NotNil(t, stored)
	content := stored.Content().(*DGRound1Message)
	assert.Equal(t, big.NewInt(1).Bytes(), content.GetVCommitment(),
		"stored message should be the original, not the duplicate")
}

// ----- [FORK] Broadcast/P2P flag validation tests ----- //

// TestResharingStoreMessageRejectsWrongBroadcastFlag verifies that a DGRound3Message1
// (which is a P2P message from old committee) is rejected when sent with IsBroadcast=true.
func TestResharingStoreMessageRejectsWrongBroadcastFlag(t *testing.T) {
	party, _, _ := newTestResharingParty(t)

	// Use the actual old party at index 1 as the sender.
	sender := party.params.OldParties().IDs()[1]

	// DGRound3Message1 is P2P. Construct it with IsBroadcast=true.
	content := &DGRound3Message1{
		Share:      big.NewInt(42).Bytes(),
		ReceiverId: party.params.NewParties().IDs()[0].GetKey(),
	}
	meta := tss.MessageRouting{
		From:        sender,
		IsBroadcast: true, // wrong: DGRound3Message1 is P2P
	}
	wire := tss.NewMessageWrapper(meta, content)
	msg := tss.NewMessage(meta, content, wire)

	ok, tssErr := party.StoreMessage(msg)
	assert.False(t, ok, "StoreMessage should reject P2P msg sent as broadcast")
	assert.Error(t, tssErr, "should return an error")
	assert.Contains(t, tssErr.Error(), "expected P2P but got broadcast",
		"error should mention P2P/broadcast mismatch")
}
