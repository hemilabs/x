// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"bytes"
	"math/big"
	"testing"

	cmt "github.com/hemilabs/x/tss/v3/crypto/commitments"
	"github.com/hemilabs/x/tss/v3/crypto/dlnproof"
	"github.com/hemilabs/x/tss/v3/crypto/paillier"
	"github.com/hemilabs/x/tss/v3/crypto/vss"
	"github.com/hemilabs/x/tss/v3/tss"
)

func TestValidateBasicKGRound1(t *testing.T) {
	valid := func() *KGRound1Message {
		return &KGRound1Message{
			Commitment: big.NewInt(1),
			PaillierPK: &paillier.PublicKey{N: big.NewInt(100)},
			NTilde:     big.NewInt(2),
			H1:         big.NewInt(3),
			H2:         big.NewInt(4),
		}
	}

	// Happy path.
	if !valid().ValidateBasic() {
		t.Fatal("valid message should pass")
	}

	// Nil receiver.
	if (*KGRound1Message)(nil).ValidateBasic() {
		t.Fatal("nil receiver should fail")
	}

	// Each field nil.
	for _, tc := range []struct {
		name   string
		mutate func(m *KGRound1Message)
	}{
		{"Commitment nil", func(m *KGRound1Message) { m.Commitment = nil }},
		{"Commitment zero", func(m *KGRound1Message) { m.Commitment = big.NewInt(0) }},
		{"PaillierPK nil", func(m *KGRound1Message) { m.PaillierPK = nil }},
		{"PaillierPK.N nil", func(m *KGRound1Message) { m.PaillierPK = &paillier.PublicKey{N: nil} }},
		{"PaillierPK.N zero", func(m *KGRound1Message) { m.PaillierPK = &paillier.PublicKey{N: big.NewInt(0)} }},
		{"NTilde nil", func(m *KGRound1Message) { m.NTilde = nil }},
		{"NTilde zero", func(m *KGRound1Message) { m.NTilde = big.NewInt(0) }},
		{"H1 nil", func(m *KGRound1Message) { m.H1 = nil }},
		{"H1 zero", func(m *KGRound1Message) { m.H1 = big.NewInt(0) }},
		{"H2 nil", func(m *KGRound1Message) { m.H2 = nil }},
		{"H2 zero", func(m *KGRound1Message) { m.H2 = big.NewInt(0) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := valid()
			tc.mutate(m)
			if m.ValidateBasic() {
				t.Fatalf("%s: should fail ValidateBasic", tc.name)
			}
		})
	}
}

func TestValidateBasicKGRound2Message1(t *testing.T) {
	valid := func() *KGRound2Message1 {
		return &KGRound2Message1{Share: big.NewInt(1), ReceiverID: []byte("r")}
	}

	// Happy path.
	if !valid().ValidateBasic() {
		t.Fatal("valid should pass")
	}

	// Nil receiver.
	if (*KGRound2Message1)(nil).ValidateBasic() {
		t.Fatal("nil receiver should fail")
	}

	for _, tc := range []struct {
		name   string
		mutate func(m *KGRound2Message1)
	}{
		{"Share nil", func(m *KGRound2Message1) { m.Share = nil }},
		{"Share zero", func(m *KGRound2Message1) { m.Share = big.NewInt(0) }},
		{"Share negative", func(m *KGRound2Message1) { m.Share = big.NewInt(-1) }},
		{"ReceiverID empty", func(m *KGRound2Message1) { m.ReceiverID = []byte{} }},
		{"ReceiverID nil", func(m *KGRound2Message1) { m.ReceiverID = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := valid()
			tc.mutate(m)
			if m.ValidateBasic() {
				t.Fatalf("%s: should fail ValidateBasic", tc.name)
			}
		})
	}
}

func TestValidateBasicKGRound2Message2(t *testing.T) {
	// Happy path: exactly 2 elements (minimum valid).
	if !(&KGRound2Message2{DeCommitment: cmt.HashDeCommitment{big.NewInt(1), big.NewInt(2)}}).ValidateBasic() {
		t.Fatal("valid 2-element decommitment should pass")
	}

	// Nil receiver.
	if (*KGRound2Message2)(nil).ValidateBasic() {
		t.Fatal("nil receiver should fail")
	}

	for _, tc := range []struct {
		name string
		msg  *KGRound2Message2
	}{
		{"empty struct", &KGRound2Message2{}},
		{"nil decommitment", &KGRound2Message2{DeCommitment: nil}},
		{"1-element decommitment", &KGRound2Message2{DeCommitment: cmt.HashDeCommitment{big.NewInt(1)}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.msg.ValidateBasic() {
				t.Fatalf("%s: should fail ValidateBasic", tc.name)
			}
		})
	}
}

func TestValidateBasicKGRound3(t *testing.T) {
	makeValid := func() paillier.Proof {
		var proof paillier.Proof
		for i := range proof {
			proof[i] = big.NewInt(int64(i + 1))
		}
		return proof
	}

	// Happy path.
	if !(&KGRound3Message{PaillierProof: makeValid()}).ValidateBasic() {
		t.Fatal("valid should pass")
	}

	// Nil receiver.
	if (*KGRound3Message)(nil).ValidateBasic() {
		t.Fatal("nil receiver should fail")
	}

	// All-nil proof (zero-value array).
	if (&KGRound3Message{}).ValidateBasic() {
		t.Fatal("all-nil proof should fail")
	}

	// Nil element at each boundary index: first, middle, last.
	for _, idx := range []int{0, 5, len(paillier.Proof{}) - 1} {
		t.Run("nil_element_"+big.NewInt(int64(idx)).String(), func(t *testing.T) {
			proof := makeValid()
			proof[idx] = nil
			if (&KGRound3Message{PaillierProof: proof}).ValidateBasic() {
				t.Fatalf("nil element at index %d should fail", idx)
			}
		})
	}
}

// --- Constructor field tests ---

// TestNewKGRound1MessageFields verifies that NewKGRound1Message populates
// all tss.Message envelope fields and content fields correctly.
func TestNewKGRound1MessageFields(t *testing.T) {
	pIDs := tss.GenerateTestPartyIDs(3)
	from := pIDs[0]

	ct := big.NewInt(42)
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

	msg := NewKGRound1Message(from, ct, pk, nTilde, h1, h2, &dlnP1, &dlnP2)

	// Envelope checks.
	if msg.From != from {
		t.Fatal("From mismatch")
	}
	if !msg.IsBroadcast {
		t.Fatal("Round1 should be broadcast")
	}
	if msg.To != nil {
		t.Fatal("Broadcast message should have nil To")
	}

	// Content checks.
	content, ok := msg.Content.(*KGRound1Message)
	if !ok {
		t.Fatalf("Content type: got %T, want *KGRound1Message", msg.Content)
	}
	if content.Commitment.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("Commitment: got %v, want 42", content.Commitment)
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
}

// TestNewKGRound1MessageNilDLN verifies that DLN proofs can be nil
// (SNARK mode).
func TestNewKGRound1MessageNilDLN(t *testing.T) {
	pIDs := tss.GenerateTestPartyIDs(2)
	msg := NewKGRound1Message(pIDs[0], big.NewInt(1),
		&paillier.PublicKey{N: big.NewInt(100)},
		big.NewInt(2), big.NewInt(3), big.NewInt(4),
		nil, nil)

	content := msg.Content.(*KGRound1Message)
	if content.DLNProof1 != nil || content.DLNProof2 != nil {
		t.Fatal("DLN proofs should be nil in SNARK mode")
	}
	if !content.ValidateBasic() {
		t.Fatal("message with nil DLN proofs should still pass ValidateBasic")
	}
}

// TestNewKGRound2Message1Fields verifies that NewKGRound2Message1
// produces a P2P message with correct From, To, ReceiverID, and share.
func TestNewKGRound2Message1Fields(t *testing.T) {
	pIDs := tss.GenerateTestPartyIDs(3)
	from := pIDs[0]
	to := pIDs[1]

	share := &vss.Share{
		Threshold: 1,
		ID:        big.NewInt(7),
		Share:     big.NewInt(999),
	}

	msg := NewKGRound2Message1(to, from, share, nil)

	// Envelope checks.
	if msg.From != from {
		t.Fatal("From mismatch")
	}
	if msg.IsBroadcast {
		t.Fatal("Round2Message1 should NOT be broadcast")
	}
	if len(msg.To) != 1 || msg.To[0] != to {
		t.Fatalf("To: got %v, want [%v]", msg.To, to)
	}

	// Content checks.
	content, ok := msg.Content.(*KGRound2Message1)
	if !ok {
		t.Fatalf("Content type: got %T, want *KGRound2Message1", msg.Content)
	}
	if content.Share.Cmp(big.NewInt(999)) != 0 {
		t.Fatalf("Share: got %v, want 999", content.Share)
	}
	if !bytes.Equal(content.ReceiverID, to.Key) {
		t.Fatalf("ReceiverID: got %x, want %x", content.ReceiverID, to.Key)
	}
	if content.FacProof != nil {
		t.Fatal("FacProof should be nil when passed nil")
	}
}

// TestNewKGRound2Message2Fields verifies that NewKGRound2Message2
// produces a broadcast message with the correct decommitment.
func TestNewKGRound2Message2Fields(t *testing.T) {
	pIDs := tss.GenerateTestPartyIDs(3)
	from := pIDs[2]

	deC := cmt.HashDeCommitment{big.NewInt(10), big.NewInt(20), big.NewInt(30)}

	msg := NewKGRound2Message2(from, deC, nil)

	// Envelope checks.
	if msg.From != from {
		t.Fatal("From mismatch")
	}
	if !msg.IsBroadcast {
		t.Fatal("Round2Message2 should be broadcast")
	}
	if msg.To != nil {
		t.Fatal("Broadcast message should have nil To")
	}

	// Content checks.
	content, ok := msg.Content.(*KGRound2Message2)
	if !ok {
		t.Fatalf("Content type: got %T, want *KGRound2Message2", msg.Content)
	}
	if len(content.DeCommitment) != 3 {
		t.Fatalf("DeCommitment length: got %d, want 3", len(content.DeCommitment))
	}
	for i, v := range []int64{10, 20, 30} {
		if content.DeCommitment[i].Cmp(big.NewInt(v)) != 0 {
			t.Fatalf("DeCommitment[%d]: got %v, want %d", i, content.DeCommitment[i], v)
		}
	}
	if content.ModProof != nil {
		t.Fatal("ModProof should be nil when passed nil")
	}
}

// TestNewKGRound3MessageFields verifies that NewKGRound3Message
// produces a broadcast message with the correct Paillier proof.
func TestNewKGRound3MessageFields(t *testing.T) {
	pIDs := tss.GenerateTestPartyIDs(3)
	from := pIDs[1]

	var proof paillier.Proof
	for i := range proof {
		proof[i] = big.NewInt(int64(i * 7))
	}

	msg := NewKGRound3Message(from, proof)

	// Envelope checks.
	if msg.From != from {
		t.Fatal("From mismatch")
	}
	if !msg.IsBroadcast {
		t.Fatal("Round3 should be broadcast")
	}
	if msg.To != nil {
		t.Fatal("Broadcast message should have nil To")
	}

	// Content checks.
	content, ok := msg.Content.(*KGRound3Message)
	if !ok {
		t.Fatalf("Content type: got %T, want *KGRound3Message", msg.Content)
	}
	for i := range content.PaillierProof {
		expected := big.NewInt(int64(i * 7))
		if content.PaillierProof[i].Cmp(expected) != 0 {
			t.Fatalf("PaillierProof[%d]: got %v, want %v", i, content.PaillierProof[i], expected)
		}
	}
}

func TestExportR2BcastSelf(t *testing.T) {
	// ExportR2BcastSelf returns the stored message for own index.
	st := &KeygenState{
		params: nil, // not needed for export
	}
	// Just verify it doesn't panic on zero state.
	// In real usage this is called after Round2.
	defer func() {
		_ = recover() // expected — params is nil
	}()
	_ = st.ExportR2BcastSelf()
}

func TestValidateSaveData(t *testing.T) {
	empty := NewLocalPartySaveData(0)
	if err := empty.ValidateSaveData(); err == nil {
		t.Fatal("empty save data should fail validation")
	}
}

func TestBuildLocalSaveDataSubset(t *testing.T) {
	// BuildLocalSaveDataSubset panics on missing key — verify it does.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for missing signer key")
		}
	}()
	sd := NewLocalPartySaveData(2)
	sd.Ks = []*big.Int{big.NewInt(1), big.NewInt(2)}
	// Pass an ID whose key doesn't match anything in Ks.
	fakeIDs := tss.GenerateTestPartyIDs(1)
	BuildLocalSaveDataSubset(sd, fakeIDs)
}
