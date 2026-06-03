// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package resharing

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss-lib/v3/crypto"
	cmt "github.com/hemilabs/x/tss-lib/v3/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v3/crypto/dlnproof"
	"github.com/hemilabs/x/tss-lib/v3/crypto/facproof"
	"github.com/hemilabs/x/tss-lib/v3/crypto/modproof"
	"github.com/hemilabs/x/tss-lib/v3/crypto/paillier"
	"github.com/hemilabs/x/tss-lib/v3/crypto/vss"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// TestNewDGRound1MessageFields verifies that NewDGRound1Message populates
// all tss.Message envelope fields and content fields correctly.
func TestNewDGRound1MessageFields(t *testing.T) {
	pIDs := tss.GenerateTestPartyIDs(4)
	from := pIDs[0]
	to := pIDs[1:]

	ecdsaPub := crypto.ScalarBaseMult(tss.S256(), big.NewInt(42))
	vct := big.NewInt(777)
	ssid := []byte("test-ssid-round1")

	msg := NewDGRound1Message(to, from, ecdsaPub, vct, ssid)

	// Envelope checks.
	if msg.From != from {
		t.Fatal("From mismatch")
	}
	if len(msg.To) != len(to) {
		t.Fatalf("To length: got %d, want %d", len(msg.To), len(to))
	}
	for i, p := range to {
		if msg.To[i] != p {
			t.Fatalf("To[%d] mismatch", i)
		}
	}
	if !msg.IsBroadcast {
		t.Fatal("Round1 should be broadcast")
	}
	if msg.IsToOldAndNewCommittees {
		t.Fatal("Round1 should NOT be IsToOldAndNewCommittees")
	}
	if msg.IsToOldCommittee {
		t.Fatal("Round1 should NOT be IsToOldCommittee")
	}

	// Content checks.
	content, ok := msg.Content.(*DGRound1Message)
	if !ok {
		t.Fatalf("Content type: got %T, want *DGRound1Message", msg.Content)
	}
	if content.ECDSAPub != ecdsaPub {
		t.Fatal("ECDSAPub mismatch")
	}
	if content.VCommitment.Cmp(big.NewInt(777)) != 0 {
		t.Fatalf("VCommitment: got %v, want 777", content.VCommitment)
	}
	if !bytes.Equal(content.SSID, ssid) {
		t.Fatalf("SSID: got %x, want %x", content.SSID, ssid)
	}
	if !content.ValidateBasic() {
		t.Fatal("constructed message should pass ValidateBasic")
	}
}

// TestNewDGRound2Message1Fields verifies that NewDGRound2Message1 populates
// all tss.Message envelope fields and content fields correctly.
func TestNewDGRound2Message1Fields(t *testing.T) {
	pIDs := tss.GenerateTestPartyIDs(4)
	from := pIDs[1]
	to := pIDs[2:]

	pk := &paillier.PublicKey{N: big.NewInt(12345)}
	nTilde := big.NewInt(100)
	h1 := big.NewInt(200)
	h2 := big.NewInt(300)

	var dlnP1, dlnP2 dlnproof.Proof
	for i := range dlnP1.Alpha {
		dlnP1.Alpha[i] = big.NewInt(int64(i + 1))
		dlnP1.T[i] = big.NewInt(int64(i + 1))
		dlnP2.Alpha[i] = big.NewInt(int64(i + 100))
		dlnP2.T[i] = big.NewInt(int64(i + 100))
	}

	var mp modproof.ProofMod
	mp.W = big.NewInt(10)
	mp.A = big.NewInt(11)
	mp.B = big.NewInt(12)

	msg := NewDGRound2Message1(to, from, pk, &mp, nTilde, h1, h2, &dlnP1, &dlnP2)

	// Envelope checks.
	if msg.From != from {
		t.Fatal("From mismatch")
	}
	if len(msg.To) != len(to) {
		t.Fatalf("To length: got %d, want %d", len(msg.To), len(to))
	}
	if !msg.IsBroadcast {
		t.Fatal("Round2Message1 should be broadcast")
	}
	if msg.IsToOldCommittee {
		t.Fatal("Round2Message1 should NOT be IsToOldCommittee")
	}
	if msg.IsToOldAndNewCommittees {
		t.Fatal("Round2Message1 should NOT be IsToOldAndNewCommittees")
	}

	// Content checks.
	content, ok := msg.Content.(*DGRound2Message1)
	if !ok {
		t.Fatalf("Content type: got %T, want *DGRound2Message1", msg.Content)
	}
	if content.PaillierPK.N.Cmp(big.NewInt(12345)) != 0 {
		t.Fatalf("PaillierPK.N: got %v, want 12345", content.PaillierPK.N)
	}
	if content.NTilde.Cmp(nTilde) != 0 {
		t.Fatal("NTilde mismatch")
	}
	if content.H1.Cmp(h1) != 0 {
		t.Fatal("H1 mismatch")
	}
	if content.H2.Cmp(h2) != 0 {
		t.Fatal("H2 mismatch")
	}
	if content.DLNProof1 == nil || content.DLNProof2 == nil {
		t.Fatal("DLN proofs should be non-nil")
	}
	if content.ModProof == nil {
		t.Fatal("ModProof should be non-nil")
	}
	if content.ModProof.W.Cmp(big.NewInt(10)) != 0 {
		t.Fatal("ModProof.W mismatch")
	}
	if !content.ValidateBasic() {
		t.Fatal("constructed message should pass ValidateBasic")
	}

	// Nil proofs (SNARK mode).
	msgNil := NewDGRound2Message1(to, from, pk, nil, nTilde, h1, h2, nil, nil)
	contentNil := msgNil.Content.(*DGRound2Message1)
	if contentNil.DLNProof1 != nil || contentNil.DLNProof2 != nil {
		t.Fatal("DLN proofs should be nil in SNARK mode")
	}
	if contentNil.ModProof != nil {
		t.Fatal("ModProof should be nil in SNARK mode")
	}
	if !contentNil.ValidateBasic() {
		t.Fatal("message with nil proofs should still pass ValidateBasic")
	}
}

// TestNewDGRound2Message2Fields verifies that NewDGRound2Message2 produces
// a broadcast ACK directed to the old committee.
func TestNewDGRound2Message2Fields(t *testing.T) {
	pIDs := tss.GenerateTestPartyIDs(4)
	from := pIDs[2]
	to := pIDs[:2]

	msg := NewDGRound2Message2(to, from)

	// Envelope checks.
	if msg.From != from {
		t.Fatal("From mismatch")
	}
	if len(msg.To) != len(to) {
		t.Fatalf("To length: got %d, want %d", len(msg.To), len(to))
	}
	if !msg.IsBroadcast {
		t.Fatal("Round2Message2 should be broadcast")
	}
	if !msg.IsToOldCommittee {
		t.Fatal("Round2Message2 should be IsToOldCommittee")
	}
	if msg.IsToOldAndNewCommittees {
		t.Fatal("Round2Message2 should NOT be IsToOldAndNewCommittees")
	}

	// Content checks.
	content, ok := msg.Content.(*DGRound2Message2)
	if !ok {
		t.Fatalf("Content type: got %T, want *DGRound2Message2", msg.Content)
	}
	if !content.ValidateBasic() {
		t.Fatal("constructed ACK should pass ValidateBasic")
	}
}

// TestNewDGRound3Message1Fields verifies that NewDGRound3Message1 produces
// a P2P message with correct Share and ReceiverID.
func TestNewDGRound3Message1Fields(t *testing.T) {
	pIDs := tss.GenerateTestPartyIDs(4)
	from := pIDs[0]
	to := pIDs[1]

	share := &vss.Share{
		Threshold: 1,
		ID:        big.NewInt(7),
		Share:     big.NewInt(999),
	}

	msg := NewDGRound3Message1(to, from, share)

	// Envelope checks.
	if msg.From != from {
		t.Fatal("From mismatch")
	}
	if len(msg.To) != 1 || msg.To[0] != to {
		t.Fatalf("To: got %v, want [%v]", msg.To, to)
	}
	if msg.IsBroadcast {
		t.Fatal("Round3Message1 should NOT be broadcast (P2P)")
	}
	if msg.IsToOldCommittee {
		t.Fatal("Round3Message1 should NOT be IsToOldCommittee")
	}
	if msg.IsToOldAndNewCommittees {
		t.Fatal("Round3Message1 should NOT be IsToOldAndNewCommittees")
	}

	// Content checks.
	content, ok := msg.Content.(*DGRound3Message1)
	if !ok {
		t.Fatalf("Content type: got %T, want *DGRound3Message1", msg.Content)
	}
	if content.Share.Cmp(big.NewInt(999)) != 0 {
		t.Fatalf("Share: got %v, want 999", content.Share)
	}
	if !bytes.Equal(content.ReceiverID, to.Key) {
		t.Fatalf("ReceiverID: got %x, want %x", content.ReceiverID, to.Key)
	}
	if !content.ValidateBasic() {
		t.Fatal("constructed message should pass ValidateBasic")
	}
}

// TestNewDGRound3Message2Fields verifies that NewDGRound3Message2 produces
// a broadcast message with the correct VDeCommitment.
func TestNewDGRound3Message2Fields(t *testing.T) {
	pIDs := tss.GenerateTestPartyIDs(4)
	from := pIDs[0]
	to := pIDs[1:]

	vdct := cmt.HashDeCommitment{big.NewInt(10), big.NewInt(20), big.NewInt(30)}

	msg := NewDGRound3Message2(to, from, vdct)

	// Envelope checks.
	if msg.From != from {
		t.Fatal("From mismatch")
	}
	if len(msg.To) != len(to) {
		t.Fatalf("To length: got %d, want %d", len(msg.To), len(to))
	}
	if !msg.IsBroadcast {
		t.Fatal("Round3Message2 should be broadcast")
	}
	if msg.IsToOldCommittee {
		t.Fatal("Round3Message2 should NOT be IsToOldCommittee")
	}
	if msg.IsToOldAndNewCommittees {
		t.Fatal("Round3Message2 should NOT be IsToOldAndNewCommittees")
	}

	// Content checks.
	content, ok := msg.Content.(*DGRound3Message2)
	if !ok {
		t.Fatalf("Content type: got %T, want *DGRound3Message2", msg.Content)
	}
	if len(content.VDeCommitment) != 3 {
		t.Fatalf("VDeCommitment length: got %d, want 3", len(content.VDeCommitment))
	}
	for i, v := range []int64{10, 20, 30} {
		if content.VDeCommitment[i].Cmp(big.NewInt(v)) != 0 {
			t.Fatalf("VDeCommitment[%d]: got %v, want %d", i, content.VDeCommitment[i], v)
		}
	}
	if !content.ValidateBasic() {
		t.Fatal("constructed message should pass ValidateBasic")
	}
}

// TestNewDGRound4Message1Fields verifies that NewDGRound4Message1 produces
// a P2P message with correct FacProof and ReceiverID.
func TestNewDGRound4Message1Fields(t *testing.T) {
	pIDs := tss.GenerateTestPartyIDs(4)
	from := pIDs[2]
	to := pIDs[0]

	proof := &facproof.ProofFac{
		P:     big.NewInt(11),
		Q:     big.NewInt(13),
		A:     big.NewInt(17),
		B:     big.NewInt(19),
		T:     big.NewInt(23),
		Sigma: big.NewInt(29),
		Z1:    big.NewInt(31),
		Z2:    big.NewInt(37),
		W1:    big.NewInt(41),
		W2:    big.NewInt(43),
		V:     big.NewInt(47),
	}

	msg := NewDGRound4Message1(to, from, proof)

	// Envelope checks.
	if msg.From != from {
		t.Fatal("From mismatch")
	}
	if len(msg.To) != 1 || msg.To[0] != to {
		t.Fatalf("To: got %v, want [%v]", msg.To, to)
	}
	if msg.IsBroadcast {
		t.Fatal("Round4Message1 should NOT be broadcast (P2P)")
	}
	if msg.IsToOldCommittee {
		t.Fatal("Round4Message1 should NOT be IsToOldCommittee")
	}
	if msg.IsToOldAndNewCommittees {
		t.Fatal("Round4Message1 should NOT be IsToOldAndNewCommittees")
	}

	// Content checks.
	content, ok := msg.Content.(*DGRound4Message1)
	if !ok {
		t.Fatalf("Content type: got %T, want *DGRound4Message1", msg.Content)
	}
	if content.FacProof == nil {
		t.Fatal("FacProof should be non-nil")
	}
	if content.FacProof.P.Cmp(big.NewInt(11)) != 0 {
		t.Fatalf("FacProof.P: got %v, want 11", content.FacProof.P)
	}
	if !bytes.Equal(content.ReceiverID, to.Key) {
		t.Fatalf("ReceiverID: got %x, want %x", content.ReceiverID, to.Key)
	}
	if !content.ValidateBasic() {
		t.Fatal("constructed message should pass ValidateBasic")
	}

	// Nil proof (SNARK mode).
	msgNil := NewDGRound4Message1(to, from, nil)
	contentNil := msgNil.Content.(*DGRound4Message1)
	if contentNil.FacProof != nil {
		t.Fatal("FacProof should be nil in SNARK mode")
	}
	if !contentNil.ValidateBasic() {
		t.Fatal("message with nil FacProof should still pass ValidateBasic")
	}
}

// TestNewDGRound4Message2Fields verifies that NewDGRound4Message2 produces
// a broadcast ACK directed to both old and new committees.
func TestNewDGRound4Message2Fields(t *testing.T) {
	pIDs := tss.GenerateTestPartyIDs(4)
	from := pIDs[3]
	to := pIDs[:3]

	msg := NewDGRound4Message2(to, from)

	// Envelope checks.
	if msg.From != from {
		t.Fatal("From mismatch")
	}
	if len(msg.To) != len(to) {
		t.Fatalf("To length: got %d, want %d", len(msg.To), len(to))
	}
	if !msg.IsBroadcast {
		t.Fatal("Round4Message2 should be broadcast")
	}
	if msg.IsToOldCommittee {
		t.Fatal("Round4Message2 should NOT be IsToOldCommittee (it goes to both)")
	}
	if !msg.IsToOldAndNewCommittees {
		t.Fatal("Round4Message2 should be IsToOldAndNewCommittees")
	}

	// Content checks.
	content, ok := msg.Content.(*DGRound4Message2)
	if !ok {
		t.Fatalf("Content type: got %T, want *DGRound4Message2", msg.Content)
	}
	if !content.ValidateBasic() {
		t.Fatal("constructed ACK should pass ValidateBasic")
	}
}
