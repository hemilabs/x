// Copyright (c) 2025 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"

	"github.com/hemilabs/x/tss-lib/v2/crypto/dlnproof"
	"github.com/hemilabs/x/tss-lib/v2/crypto/facproof"
	"github.com/hemilabs/x/tss-lib/v2/crypto/modproof"
	"github.com/hemilabs/x/tss-lib/v2/crypto/paillier"
	"github.com/hemilabs/x/tss-lib/v2/crypto/vss"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

// TestKGRound2Message1ValidateBasicRejectsEmptyShare verifies that ValidateBasic
// rejects a message with an empty share field.
func TestKGRound2Message1ValidateBasicRejectsEmptyShare(t *testing.T) {
	msg := &KGRound2Message1{
		Share:      nil,
		FacProof:   makeDummyFacProof(),
		ReceiverId: []byte{0x01, 0x02, 0x03},
	}
	assert.False(t, msg.ValidateBasic(), "empty share should fail ValidateBasic")
}

// TestKGRound2Message1ValidateBasicAcceptsNilFacProof verifies that ValidateBasic
// accepts a message with a nil facProof field (on-chain SNARK mode).
func TestKGRound2Message1ValidateBasicAcceptsNilFacProof(t *testing.T) {
	msg := &KGRound2Message1{
		Share:      []byte{0x01},
		FacProof:   nil,
		ReceiverId: []byte{0x01, 0x02, 0x03},
	}
	assert.True(t, msg.ValidateBasic(), "nil facProof should pass ValidateBasic (on-chain mode)")
}

// TestKGRound2Message1ValidateBasicRejectsEmptyReceiverId verifies that ValidateBasic
// rejects a message with an empty receiverId field.
func TestKGRound2Message1ValidateBasicRejectsEmptyReceiverId(t *testing.T) {
	msg := &KGRound2Message1{
		Share:      []byte{0x01},
		FacProof:   makeDummyFacProof(),
		ReceiverId: nil,
	}
	assert.False(t, msg.ValidateBasic(), "empty receiverId should fail ValidateBasic")
}

// TestKGRound2Message1ValidateBasicAcceptsValid verifies that ValidateBasic
// accepts a properly populated message.
func TestKGRound2Message1ValidateBasicAcceptsValid(t *testing.T) {
	msg := &KGRound2Message1{
		Share:      []byte{0x01},
		FacProof:   makeDummyFacProof(),
		ReceiverId: []byte{0x01, 0x02, 0x03},
	}
	assert.True(t, msg.ValidateBasic(), "valid message should pass ValidateBasic")
}

// TestKGRound2Message1ValidateBasicRejectsNil verifies nil message.
func TestKGRound2Message1ValidateBasicRejectsNil(t *testing.T) {
	var msg *KGRound2Message1
	assert.False(t, msg.ValidateBasic(), "nil message should fail ValidateBasic")
}

// TestKGRound2Message1UnmarshalReceiverId verifies the UnmarshalReceiverId accessor.
func TestKGRound2Message1UnmarshalReceiverId(t *testing.T) {
	receiverId := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	msg := &KGRound2Message1{
		ReceiverId: receiverId,
	}
	assert.Equal(t, receiverId, msg.UnmarshalReceiverId())
}

// ---------------------------------------------------------------------------
// KGRound2Message2 ValidateBasic tests (MOD proof check was uncommented)
// ---------------------------------------------------------------------------

// TestKGRound2Message2ValidateBasicRejectsEmptyDeCommitment verifies that
// ValidateBasic rejects a message with empty deCommitment.
func TestKGRound2Message2ValidateBasicRejectsEmptyDeCommitment(t *testing.T) {
	msg := &KGRound2Message2{
		DeCommitment: nil,
		ModProof:     makeDummyModProof(),
	}
	assert.False(t, msg.ValidateBasic(), "empty deCommitment should fail ValidateBasic")
}

// TestKGRound2Message2ValidateBasicAcceptsNilModProof verifies that
// ValidateBasic accepts a message with nil modProof (on-chain SNARK mode).
func TestKGRound2Message2ValidateBasicAcceptsNilModProof(t *testing.T) {
	msg := &KGRound2Message2{
		DeCommitment: [][]byte{{0x01}, {0x02}},
		ModProof:     nil,
	}
	assert.True(t, msg.ValidateBasic(), "nil modProof should pass ValidateBasic (on-chain mode)")
}

// TestKGRound2Message2ValidateBasicRejectsNil verifies nil message.
func TestKGRound2Message2ValidateBasicRejectsNil(t *testing.T) {
	var msg *KGRound2Message2
	assert.False(t, msg.ValidateBasic(), "nil message should fail ValidateBasic")
}

// TestKGRound2Message2ValidateBasicAcceptsValid verifies a properly populated message.
func TestKGRound2Message2ValidateBasicAcceptsValid(t *testing.T) {
	msg := &KGRound2Message2{
		DeCommitment: [][]byte{{0x01}, {0x02}},
		ModProof:     makeDummyModProof(),
	}
	assert.True(t, msg.ValidateBasic(), "valid message should pass ValidateBasic")
}

// ---------------------------------------------------------------------------
// NewKGRound2Message1 constructor integration test
// ---------------------------------------------------------------------------

// TestNewKGRound2Message1PopulatesReceiverId verifies that the constructor
// populates receiverId from to.GetKey().
func TestNewKGRound2Message1PopulatesReceiverId(t *testing.T) {
	receiverKey := big.NewInt(0xDEADBEEF)
	to := tss.NewPartyID("receiver", "Receiver", receiverKey)
	from := tss.NewPartyID("sender", "Sender", big.NewInt(0xCAFE))

	share := &vss.Share{
		Threshold: 1,
		ID:        big.NewInt(1),
		Share:     big.NewInt(12345),
	}

	// Create a minimal valid ProofFac for the constructor.
	proof := &facproof.ProofFac{
		P: big.NewInt(1), Q: big.NewInt(2), A: big.NewInt(3),
		B: big.NewInt(4), T: big.NewInt(5), Sigma: big.NewInt(6),
		Z1: big.NewInt(7), Z2: big.NewInt(8),
		W1: big.NewInt(9), W2: big.NewInt(10),
		V: big.NewInt(11),
	}

	parsed := NewKGRound2Message1(to, from, share, proof)
	content := parsed.Content().(*KGRound2Message1)

	// The receiverId should be to.GetKey() = receiverKey.Bytes()
	assert.Equal(t, receiverKey.Bytes(), content.GetReceiverId(),
		"receiverId should match to.GetKey()")
	assert.NotEmpty(t, content.GetReceiverId())
}

// ---------------------------------------------------------------------------
// Protobuf round-trip for receiverId
// ---------------------------------------------------------------------------

// TestKGRound2Message1ProtoRoundTrip verifies that protobuf marshal/unmarshal
// preserves the receiverId field.
func TestKGRound2Message1ProtoRoundTrip(t *testing.T) {
	original := &KGRound2Message1{
		Share:      []byte{0x01, 0x02, 0x03},
		FacProof:   makeDummyFacProof(),
		ReceiverId: []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE},
	}

	data, err := proto.Marshal(original)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	recovered := &KGRound2Message1{}
	err = proto.Unmarshal(data, recovered)
	assert.NoError(t, err)

	assert.Equal(t, original.Share, recovered.Share, "share mismatch")
	assert.Equal(t, original.ReceiverId, recovered.ReceiverId, "receiverId mismatch after proto round-trip")
	assert.Equal(t, len(original.FacProof), len(recovered.FacProof), "facProof length mismatch")
}

// ---------------------------------------------------------------------------
// Wrong-length proof parts tests
// ---------------------------------------------------------------------------

// TestKGRound2Message1ValidateBasicRejectsWrongFacProofLength verifies that
// ValidateBasic rejects facProof with wrong number of parts.
func TestKGRound2Message1ValidateBasicRejectsWrongFacProofLength(t *testing.T) {
	// Too few parts.
	shortProof := make([][]byte, facproof.ProofFacBytesParts-1)
	for i := range shortProof {
		shortProof[i] = []byte{0x01}
	}
	msg := &KGRound2Message1{
		Share:      []byte{0x01},
		FacProof:   shortProof,
		ReceiverId: []byte{0x01, 0x02, 0x03},
	}
	assert.False(t, msg.ValidateBasic(), "short facProof should fail ValidateBasic")

	// Too many parts.
	longProof := make([][]byte, facproof.ProofFacBytesParts+1)
	for i := range longProof {
		longProof[i] = []byte{0x01}
	}
	msg.FacProof = longProof
	assert.False(t, msg.ValidateBasic(), "long facProof should fail ValidateBasic")
}

// TestKGRound2Message2ValidateBasicRejectsWrongModProofLength verifies that
// ValidateBasic rejects modProof with wrong number of parts.
func TestKGRound2Message2ValidateBasicRejectsWrongModProofLength(t *testing.T) {
	// Too few parts.
	shortProof := make([][]byte, modproof.ProofModBytesParts-1)
	for i := range shortProof {
		shortProof[i] = []byte{0x01}
	}
	msg := &KGRound2Message2{
		DeCommitment: [][]byte{{0x01}, {0x02}},
		ModProof:     shortProof,
	}
	assert.False(t, msg.ValidateBasic(), "short modProof should fail ValidateBasic")

	// Too many parts.
	longProof := make([][]byte, modproof.ProofModBytesParts+1)
	for i := range longProof {
		longProof[i] = []byte{0x01}
	}
	msg.ModProof = longProof
	assert.False(t, msg.ValidateBasic(), "long modProof should fail ValidateBasic")
}

// TestKGRound2Message1ProtoRoundTripEmptyReceiverId verifies that proto3
// treats empty bytes and nil identically.
func TestKGRound2Message1ProtoRoundTripEmptyReceiverId(t *testing.T) {
	original := &KGRound2Message1{
		Share:      []byte{0x01},
		FacProof:   makeDummyFacProof(),
		ReceiverId: []byte{}, // explicitly empty
	}

	data, err := proto.Marshal(original)
	assert.NoError(t, err)

	recovered := &KGRound2Message1{}
	err = proto.Unmarshal(data, recovered)
	assert.NoError(t, err)

	// In proto3, empty bytes field is indistinguishable from absent.
	// GetReceiverId returns nil for absent fields.
	assert.False(t, recovered.ValidateBasic(),
		"empty receiverId should fail ValidateBasic after proto round-trip")
}

// ---------------------------------------------------------------------------
// KGRound1Message ValidateBasic tests
// ---------------------------------------------------------------------------

// TestKGRound1MessageValidateBasicAcceptsNilDLNProofs verifies that
// ValidateBasic accepts a message with nil DLN proofs (on-chain SNARK mode).
func TestKGRound1MessageValidateBasicAcceptsNilDLNProofs(t *testing.T) {
	msg := &KGRound1Message{
		Commitment: []byte{0x01},
		PaillierN:  []byte{0x01},
		NTilde:     []byte{0x01},
		H1:         []byte{0x01},
		H2:         []byte{0x01},
		Dlnproof_1: nil,
		Dlnproof_2: nil,
	}
	assert.True(t, msg.ValidateBasic(), "nil DLN proofs should pass ValidateBasic (on-chain mode)")

	// Also test with empty slices.
	msg.Dlnproof_1 = [][]byte{}
	msg.Dlnproof_2 = [][]byte{}
	assert.True(t, msg.ValidateBasic(), "empty DLN proofs should pass ValidateBasic (on-chain mode)")
}

// TestKGRound1MessageValidateBasicRejectsNil verifies nil message.
func TestKGRound1MessageValidateBasicRejectsNil(t *testing.T) {
	var msg *KGRound1Message
	assert.False(t, msg.ValidateBasic(), "nil message should fail ValidateBasic")
}

// TestKGRound1MessageValidateBasicRejectsEmptyCommitment verifies that
// ValidateBasic rejects a message with empty commitment.
func TestKGRound1MessageValidateBasicRejectsEmptyCommitment(t *testing.T) {
	msg := makeDummyKGRound1Message()
	msg.Commitment = nil
	assert.False(t, msg.ValidateBasic(), "empty commitment should fail ValidateBasic")
}

// TestKGRound1MessageValidateBasicRejectsEmptyPaillierN verifies that
// ValidateBasic rejects a message with empty PaillierN.
func TestKGRound1MessageValidateBasicRejectsEmptyPaillierN(t *testing.T) {
	msg := makeDummyKGRound1Message()
	msg.PaillierN = nil
	assert.False(t, msg.ValidateBasic(), "empty PaillierN should fail ValidateBasic")
}

// TestKGRound1MessageValidateBasicRejectsEmptyNTilde verifies that
// ValidateBasic rejects a message with empty NTilde.
func TestKGRound1MessageValidateBasicRejectsEmptyNTilde(t *testing.T) {
	msg := makeDummyKGRound1Message()
	msg.NTilde = nil
	assert.False(t, msg.ValidateBasic(), "empty NTilde should fail ValidateBasic")
}

// TestKGRound1MessageValidateBasicRejectsEmptyH1 verifies that
// ValidateBasic rejects a message with empty H1.
func TestKGRound1MessageValidateBasicRejectsEmptyH1(t *testing.T) {
	msg := makeDummyKGRound1Message()
	msg.H1 = nil
	assert.False(t, msg.ValidateBasic(), "empty H1 should fail ValidateBasic")
}

// TestKGRound1MessageValidateBasicRejectsEmptyH2 verifies that
// ValidateBasic rejects a message with empty H2.
func TestKGRound1MessageValidateBasicRejectsEmptyH2(t *testing.T) {
	msg := makeDummyKGRound1Message()
	msg.H2 = nil
	assert.False(t, msg.ValidateBasic(), "empty H2 should fail ValidateBasic")
}

// TestKGRound1MessageValidateBasicRejectsWrongDLNProof1Length verifies that
// ValidateBasic rejects wrong-length DLN proof 1.
func TestKGRound1MessageValidateBasicRejectsWrongDLNProof1Length(t *testing.T) {
	msg := makeDummyKGRound1Message()
	// Too few parts.
	msg.Dlnproof_1 = make([][]byte, 5)
	for i := range msg.Dlnproof_1 {
		msg.Dlnproof_1[i] = []byte{0x01}
	}
	assert.False(t, msg.ValidateBasic(), "short DLN proof 1 should fail ValidateBasic")
}

// TestKGRound1MessageValidateBasicRejectsWrongDLNProof2Length verifies that
// ValidateBasic rejects wrong-length DLN proof 2.
func TestKGRound1MessageValidateBasicRejectsWrongDLNProof2Length(t *testing.T) {
	msg := makeDummyKGRound1Message()
	// Too many parts.
	msg.Dlnproof_2 = make([][]byte, 2+(dlnproof.Iterations*2)+1)
	for i := range msg.Dlnproof_2 {
		msg.Dlnproof_2[i] = []byte{0x01}
	}
	assert.False(t, msg.ValidateBasic(), "long DLN proof 2 should fail ValidateBasic")
}

// TestKGRound1MessageValidateBasicAcceptsValid verifies a properly populated message.
func TestKGRound1MessageValidateBasicAcceptsValid(t *testing.T) {
	msg := makeDummyKGRound1Message()
	assert.True(t, msg.ValidateBasic(), "valid message should pass ValidateBasic")
}

// ---------------------------------------------------------------------------
// KGRound3Message ValidateBasic tests
// ---------------------------------------------------------------------------

// TestKGRound3MessageValidateBasicRejectsNil verifies nil message.
func TestKGRound3MessageValidateBasicRejectsNil(t *testing.T) {
	var msg *KGRound3Message
	assert.False(t, msg.ValidateBasic(), "nil message should fail ValidateBasic")
}

// TestKGRound3MessageValidateBasicRejectsWrongProofLength verifies that
// ValidateBasic rejects wrong-length Paillier proof.
func TestKGRound3MessageValidateBasicRejectsWrongProofLength(t *testing.T) {
	// Too few parts.
	shortProof := make([][]byte, paillier.ProofIters-1)
	for i := range shortProof {
		shortProof[i] = []byte{0x01}
	}
	msg := &KGRound3Message{PaillierProof: shortProof}
	assert.False(t, msg.ValidateBasic(), "short Paillier proof should fail ValidateBasic")

	// Too many parts.
	longProof := make([][]byte, paillier.ProofIters+1)
	for i := range longProof {
		longProof[i] = []byte{0x01}
	}
	msg.PaillierProof = longProof
	assert.False(t, msg.ValidateBasic(), "long Paillier proof should fail ValidateBasic")
}

// TestKGRound3MessageValidateBasicRejectsEmptyElement verifies that
// ValidateBasic rejects a proof with an empty element.
func TestKGRound3MessageValidateBasicRejectsEmptyElement(t *testing.T) {
	proof := makeDummyPaillierProof()
	proof[5] = nil // one empty element
	msg := &KGRound3Message{PaillierProof: proof}
	assert.False(t, msg.ValidateBasic(), "proof with empty element should fail ValidateBasic")
}

// TestKGRound3MessageValidateBasicAcceptsValid verifies a properly populated message.
func TestKGRound3MessageValidateBasicAcceptsValid(t *testing.T) {
	msg := &KGRound3Message{PaillierProof: makeDummyPaillierProof()}
	assert.True(t, msg.ValidateBasic(), "valid message should pass ValidateBasic")
}

// ---------------------------------------------------------------------------
// ReceiverID verification gap tests
// ---------------------------------------------------------------------------

// TestKGRound2Message1ReceiverIdValidateBasicAcceptsAnyContent verifies that
// ValidateBasic accepts any non-empty receiverId content (it only checks
// non-empty, not semantic correctness). The actual receiverId verification
// happens in round_3.go via bytes.Equal(r2msg1.GetReceiverId(), myKey).
func TestKGRound2Message1ReceiverIdValidateBasicAcceptsAnyContent(t *testing.T) {
	// Create two messages with different receiverId content.
	wrongReceiver := []byte{0xDE, 0xAD}
	rightReceiver := []byte{0xBE, 0xEF}

	msg1 := &KGRound2Message1{
		Share:      []byte{0x01},
		FacProof:   makeDummyFacProof(),
		ReceiverId: wrongReceiver,
	}
	msg2 := &KGRound2Message1{
		Share:      []byte{0x01},
		FacProof:   makeDummyFacProof(),
		ReceiverId: rightReceiver,
	}

	// Both pass ValidateBasic regardless of receiverId content — ValidateBasic
	// only checks non-empty, semantic verification is in round_3.go.
	assert.True(t, msg1.ValidateBasic(), "msg with wrong receiverId should still pass ValidateBasic")
	assert.True(t, msg2.ValidateBasic(), "msg with right receiverId should pass ValidateBasic")
}

// ---------------------------------------------------------------------------
// KGRound3Message UnmarshalProofInts edge case tests
// ---------------------------------------------------------------------------

// TestKGRound3MessageUnmarshalProofIntsShortSlice documents that
// UnmarshalProofInts panics if the PaillierProof slice has fewer than
// ProofIters elements. ValidateBasic guards against this, but the function
// itself has no bounds check.
func TestKGRound3MessageUnmarshalProofIntsShortSlice(t *testing.T) {
	shortProof := make([][]byte, 5) // fewer than paillier.ProofIters (13)
	for i := range shortProof {
		shortProof[i] = []byte{0x01}
	}
	msg := &KGRound3Message{PaillierProof: shortProof}

	// Confirm ValidateBasic rejects it.
	assert.False(t, msg.ValidateBasic(), "short proof should fail ValidateBasic")

	// Confirm UnmarshalProofInts would panic (defense-in-depth gap).
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("UnmarshalProofInts should panic on short slice")
		} else {
			t.Logf("KNOWN GAP: UnmarshalProofInts panics on short slice: %v", r)
		}
	}()
	msg.UnmarshalProofInts()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makeDummyKGRound1Message creates a KGRound1Message with the right number of
// parts in each field for ValidateBasic to accept.
func makeDummyKGRound1Message() *KGRound1Message {
	dlnParts := 2 + (dlnproof.Iterations * 2)
	dlnProof1 := make([][]byte, dlnParts)
	for i := range dlnProof1 {
		dlnProof1[i] = []byte{0x01}
	}
	dlnProof2 := make([][]byte, dlnParts)
	for i := range dlnProof2 {
		dlnProof2[i] = []byte{0x01}
	}
	return &KGRound1Message{
		Commitment: []byte{0x01},
		PaillierN:  []byte{0x01},
		NTilde:     []byte{0x01},
		H1:         []byte{0x01},
		H2:         []byte{0x01},
		Dlnproof_1: dlnProof1,
		Dlnproof_2: dlnProof2,
	}
}

// makeDummyPaillierProof creates a Paillier proof with the right number of
// parts for ValidateBasic to accept.
func makeDummyPaillierProof() [][]byte {
	proof := make([][]byte, paillier.ProofIters)
	for i := range proof {
		proof[i] = []byte{0x01}
	}
	return proof
}

// makeDummyFacProof creates a facProof byte slice with the right number of parts
// for ValidateBasic to accept.
func makeDummyFacProof() [][]byte {
	parts := make([][]byte, facproof.ProofFacBytesParts)
	for i := range parts {
		parts[i] = []byte{0x01}
	}
	return parts
}

// makeDummyModProof creates a modProof byte slice with the right number of parts
// for ValidateBasic to accept.
func makeDummyModProof() [][]byte {
	parts := make([][]byte, modproof.ProofModBytesParts)
	for i := range parts {
		parts[i] = []byte{0x01}
	}
	return parts
}

// ---------------------------------------------------------------------------
// Additional KGRound2Message1 receiver/share edge-case tests
// ---------------------------------------------------------------------------

// TestKGRound2Message1ReceiverIdEmpty verifies that ValidateBasic rejects
// a message where ReceiverId is an explicitly empty (zero-length) byte slice.
func TestKGRound2Message1ReceiverIdEmpty(t *testing.T) {
	msg := &KGRound2Message1{
		Share:      []byte{0x01},
		FacProof:   makeDummyFacProof(),
		ReceiverId: []byte{},
	}
	assert.False(t, msg.ValidateBasic(), "empty ReceiverId should fail ValidateBasic")
}

// TestKGRound2Message1ReceiverIdNil verifies that ValidateBasic rejects
// a message where ReceiverId is nil.
func TestKGRound2Message1ReceiverIdNil(t *testing.T) {
	msg := &KGRound2Message1{
		Share:      []byte{0x01},
		FacProof:   makeDummyFacProof(),
		ReceiverId: nil,
	}
	assert.False(t, msg.ValidateBasic(), "nil ReceiverId should fail ValidateBasic")
}

// TestKGRound2Message1ShareZero verifies that ValidateBasic accepts a message
// whose Share is a single zero byte (non-empty).
func TestKGRound2Message1ShareZero(t *testing.T) {
	msg := &KGRound2Message1{
		Share:      []byte{0x00},
		FacProof:   makeDummyFacProof(),
		ReceiverId: []byte{0x01, 0x02, 0x03},
	}
	assert.True(t, msg.ValidateBasic(), "single zero-byte Share should pass ValidateBasic (non-empty)")
}

// ---------------------------------------------------------------------------
// Additional KGRound2Message2 ValidateBasic tests
// ---------------------------------------------------------------------------

// TestKGRound2Message2ValidateBasicMinimal verifies that ValidateBasic accepts
// a message with the minimum valid fields: a non-empty DeCommitment and a
// ModProof with the correct number of parts.
func TestKGRound2Message2ValidateBasicMinimal(t *testing.T) {
	msg := &KGRound2Message2{
		DeCommitment: [][]byte{{0x01}, {0x02}, {0x03}},
		ModProof:     makeDummyModProof(),
	}
	assert.True(t, msg.ValidateBasic(), "minimal valid KGRound2Message2 should pass ValidateBasic")
}

// TestKGRound2Message2ValidateBasicAcceptsEmptyModProof verifies that ValidateBasic
// accepts a message whose ModProof is an empty (zero-length) slice (on-chain SNARK mode).
func TestKGRound2Message2ValidateBasicAcceptsEmptyModProof(t *testing.T) {
	msg := &KGRound2Message2{
		DeCommitment: [][]byte{{0x01}, {0x02}, {0x03}},
		ModProof:     [][]byte{},
	}
	assert.True(t, msg.ValidateBasic(), "empty ModProof should pass ValidateBasic (on-chain mode)")
}

// ---------------------------------------------------------------------------
// Additional KGRound3Message ValidateBasic and UnmarshalProofInts tests
// ---------------------------------------------------------------------------

// TestKGRound3MessageValidateBasicValid verifies that ValidateBasic accepts
// a message with exactly paillier.ProofIters (13) non-empty byte slices.
func TestKGRound3MessageValidateBasicValid(t *testing.T) {
	proof := make([][]byte, paillier.ProofIters)
	for i := range proof {
		proof[i] = []byte{byte(i + 1)}
	}
	msg := &KGRound3Message{PaillierProof: proof}
	assert.True(t, msg.ValidateBasic(), "valid KGRound3Message should pass ValidateBasic")
}

// TestKGRound3MessageUnmarshalProofIntsValid verifies that UnmarshalProofInts
// returns exactly paillier.ProofIters (13) *big.Int values from valid input.
func TestKGRound3MessageUnmarshalProofIntsValid(t *testing.T) {
	proof := make([][]byte, paillier.ProofIters)
	for i := range proof {
		proof[i] = []byte{byte(i + 1)}
	}
	msg := &KGRound3Message{PaillierProof: proof}
	assert.True(t, msg.ValidateBasic(), "precondition: message must be valid")

	ints := msg.UnmarshalProofInts()
	assert.Equal(t, paillier.ProofIters, len(ints), "UnmarshalProofInts should return %d elements", paillier.ProofIters)
	for i, v := range ints {
		assert.NotNil(t, v, "element %d should not be nil", i)
	}
}

// ---------------------------------------------------------------------------
// KGRound1Message Unmarshal* accessor tests
// ---------------------------------------------------------------------------

// TestKGRound1MessageUnmarshalCommitment verifies the round-trip of Commitment.
func TestKGRound1MessageUnmarshalCommitment(t *testing.T) {
	val := big.NewInt(12345)
	msg := &KGRound1Message{Commitment: val.Bytes()}
	result := msg.UnmarshalCommitment()
	assert.Equal(t, 0, val.Cmp(result), "UnmarshalCommitment should return original value")
}

// TestKGRound1MessageUnmarshalPaillierPK verifies the round-trip of PaillierN.
func TestKGRound1MessageUnmarshalPaillierPK(t *testing.T) {
	n := big.NewInt(999999937) // a large prime-ish number
	msg := &KGRound1Message{PaillierN: n.Bytes()}
	pk := msg.UnmarshalPaillierPK()
	assert.NotNil(t, pk)
	assert.Equal(t, 0, n.Cmp(pk.N), "PaillierPK.N should match original")
}

// TestKGRound1MessageUnmarshalNTilde verifies the round-trip of NTilde.
func TestKGRound1MessageUnmarshalNTilde(t *testing.T) {
	val := big.NewInt(54321)
	msg := &KGRound1Message{NTilde: val.Bytes()}
	result := msg.UnmarshalNTilde()
	assert.Equal(t, 0, val.Cmp(result), "UnmarshalNTilde should return original value")
}

// TestKGRound1MessageUnmarshalH1H2 verifies the round-trip of H1 and H2.
func TestKGRound1MessageUnmarshalH1H2(t *testing.T) {
	h1 := big.NewInt(111)
	h2 := big.NewInt(222)
	msg := &KGRound1Message{H1: h1.Bytes(), H2: h2.Bytes()}
	assert.Equal(t, 0, h1.Cmp(msg.UnmarshalH1()), "UnmarshalH1 mismatch")
	assert.Equal(t, 0, h2.Cmp(msg.UnmarshalH2()), "UnmarshalH2 mismatch")
}

// TestKGRound1MessageUnmarshalDLNProofsInvalidInput verifies that UnmarshalDLNProof1
// and UnmarshalDLNProof2 return errors for malformed input.
func TestKGRound1MessageUnmarshalDLNProofsInvalidInput(t *testing.T) {
	msg := &KGRound1Message{
		Dlnproof_1: [][]byte{{0x01}},        // too few parts
		Dlnproof_2: [][]byte{{0x01}, {0x02}}, // too few parts
	}
	_, err1 := msg.UnmarshalDLNProof1()
	assert.Error(t, err1, "malformed DLN proof 1 should error")
	_, err2 := msg.UnmarshalDLNProof2()
	assert.Error(t, err2, "malformed DLN proof 2 should error")
}

// ---------------------------------------------------------------------------
// UnmarshalShare zero-value round-trip
// ---------------------------------------------------------------------------

// TestKGRound2Message1UnmarshalShareZeroValue documents the zero-value
// round-trip behavior: big.NewInt(0).Bytes() = []byte{}, and
// new(big.Int).SetBytes([]byte{}) = big.NewInt(0) with Sign() == 0.
func TestKGRound2Message1UnmarshalShareZeroValue(t *testing.T) {
	// Share was originally zero.
	msg := &KGRound2Message1{Share: big.NewInt(0).Bytes()} // = []byte{}
	result := msg.UnmarshalShare()
	assert.Equal(t, 0, result.Sign(), "zero share should unmarshal to Sign()==0")
	assert.Equal(t, 0, result.Cmp(big.NewInt(0)), "zero share should equal big.NewInt(0)")

	// Verify that an explicit []byte{0x00} also produces big.NewInt(0).
	msg2 := &KGRound2Message1{Share: []byte{0x00}}
	result2 := msg2.UnmarshalShare()
	assert.Equal(t, 0, result2.Sign(), "single zero byte should also unmarshal to Sign()==0")

	// Both forms produce the same big.Int value.
	assert.Equal(t, 0, result.Cmp(result2),
		"empty bytes and [0x00] should both unmarshal to big.NewInt(0)")
}

// TestKGRound2Message1UnmarshalShareNonZero verifies normal share round-trip.
func TestKGRound2Message1UnmarshalShareNonZero(t *testing.T) {
	original := big.NewInt(9999)
	msg := &KGRound2Message1{Share: original.Bytes()}
	result := msg.UnmarshalShare()
	assert.Equal(t, 0, original.Cmp(result), "non-zero share should round-trip correctly")
}

// ---------------------------------------------------------------------------
// UnmarshalModProof error path
// ---------------------------------------------------------------------------

// TestKGRound2Message2UnmarshalModProofMalformed verifies that UnmarshalModProof
// returns an error for malformed input.
func TestKGRound2Message2UnmarshalModProofMalformed(t *testing.T) {
	msg := &KGRound2Message2{
		ModProof: [][]byte{{0x01}}, // too few parts
	}
	_, err := msg.UnmarshalModProof()
	assert.Error(t, err, "malformed modProof should return error")
}

// TestReceiverIdMismatchDetection verifies the defense-in-depth check from
// round_3.go:102 — bytes.Equal(r2msg1.GetReceiverId(), myKey). This exercises
// the check logic directly on message fields to ensure wrong-receiverId
// messages would be rejected.
func TestReceiverIdMismatchDetection(t *testing.T) {
	myKey := big.NewInt(0xDEADBEEF).Bytes()
	wrongKey := big.NewInt(0xCAFEBABE).Bytes()

	msg := &KGRound2Message1{
		Share:      []byte{0x01},
		FacProof:   makeDummyFacProof(),
		ReceiverId: wrongKey,
	}

	// Verify mismatch detection (same logic as round_3.go:102).
	assert.False(t, bytes.Equal(msg.GetReceiverId(), myKey),
		"wrong receiverId should not match myKey")

	// Verify match detection with correct receiverId.
	msg.ReceiverId = myKey
	assert.True(t, bytes.Equal(msg.GetReceiverId(), myKey),
		"correct receiverId should match myKey")

	// ValidateBasic still passes for both cases — it only checks
	// non-empty, semantic verification is in round_3.go.
	msg.ReceiverId = wrongKey
	assert.True(t, msg.ValidateBasic(),
		"ValidateBasic should pass even with wrong receiverId (it only checks non-empty)")
}

// TestReceiverIdMismatchConstructorPath verifies that NewKGRound2Message1
// populates receiverId from to.GetKey(), and a mismatch with a different
// party's key is detectable via bytes.Equal.
func TestReceiverIdMismatchConstructorPath(t *testing.T) {
	receiverKey := big.NewInt(0xDEADBEEF)
	attackerKey := big.NewInt(0xBAADF00D)
	to := tss.NewPartyID("receiver", "Receiver", receiverKey)
	from := tss.NewPartyID("sender", "Sender", big.NewInt(0xCAFE))

	share := &vss.Share{
		Threshold: 1,
		ID:        big.NewInt(1),
		Share:     big.NewInt(12345),
	}

	proof := &facproof.ProofFac{
		P: big.NewInt(1), Q: big.NewInt(2), A: big.NewInt(3),
		B: big.NewInt(4), T: big.NewInt(5), Sigma: big.NewInt(6),
		Z1: big.NewInt(7), Z2: big.NewInt(8),
		W1: big.NewInt(9), W2: big.NewInt(10),
		V: big.NewInt(11),
	}

	parsed := NewKGRound2Message1(to, from, share, proof)
	content := parsed.Content().(*KGRound2Message1)

	// Should match the intended receiver.
	assert.True(t, bytes.Equal(content.GetReceiverId(), receiverKey.Bytes()),
		"receiverId should match intended receiver")

	// Should NOT match a different party (attacker trying to steal the share).
	assert.False(t, bytes.Equal(content.GetReceiverId(), attackerKey.Bytes()),
		"receiverId should not match attacker's key")
}

// TestKGRound2Message1UnmarshalFacProofMalformed verifies that UnmarshalFacProof
// returns an error for malformed input.
func TestKGRound2Message1UnmarshalFacProofMalformed(t *testing.T) {
	msg := &KGRound2Message1{
		FacProof: [][]byte{{0x01}}, // too few parts
	}
	_, err := msg.UnmarshalFacProof()
	assert.Error(t, err, "malformed facProof should return error")
}

// ---------------------------------------------------------------------------
// Round 2 agent review: additional edge-case tests
// ---------------------------------------------------------------------------

// TestKGRound1MessageAsymmetricDLNProofs verifies ValidateBasic behavior
// when one DLN proof is present and the other is absent. Both configurations
// should pass since each proof is independently optional.
func TestKGRound1MessageAsymmetricDLNProofs(t *testing.T) {
	dlnParts := 2 + (dlnproof.Iterations * 2)
	fullProof := make([][]byte, dlnParts)
	for i := range fullProof {
		fullProof[i] = []byte{0x01}
	}

	// DLN proof 1 present, DLN proof 2 absent.
	msg := &KGRound1Message{
		Commitment: []byte{0x01},
		PaillierN:  []byte{0x01},
		NTilde:     []byte{0x01},
		H1:         []byte{0x01},
		H2:         []byte{0x01},
		Dlnproof_1: fullProof,
		Dlnproof_2: nil,
	}
	assert.True(t, msg.ValidateBasic(),
		"DLN proof 1 present + DLN proof 2 absent should pass ValidateBasic")

	// DLN proof 1 absent, DLN proof 2 present.
	msg.Dlnproof_1 = nil
	msg.Dlnproof_2 = fullProof
	assert.True(t, msg.ValidateBasic(),
		"DLN proof 1 absent + DLN proof 2 present should pass ValidateBasic")
}

// TestKGRound1MessageDLNProofOffByOne verifies that DLN proofs with exactly
// one element too many or too few are rejected, while the exact correct count
// is accepted.
func TestKGRound1MessageDLNProofOffByOne(t *testing.T) {
	correctLen := 2 + (dlnproof.Iterations * 2)

	makeProof := func(n int) [][]byte {
		p := make([][]byte, n)
		for i := range p {
			p[i] = []byte{0x01}
		}
		return p
	}

	base := &KGRound1Message{
		Commitment: []byte{0x01},
		PaillierN:  []byte{0x01},
		NTilde:     []byte{0x01},
		H1:         []byte{0x01},
		H2:         []byte{0x01},
	}

	// Exact correct count: should pass.
	base.Dlnproof_1 = makeProof(correctLen)
	base.Dlnproof_2 = makeProof(correctLen)
	assert.True(t, base.ValidateBasic(), "exact correct DLN proof length should pass")

	// One too few in proof 1: should fail.
	base.Dlnproof_1 = makeProof(correctLen - 1)
	base.Dlnproof_2 = makeProof(correctLen)
	assert.False(t, base.ValidateBasic(), "DLN proof 1 with one too few elements should fail")

	// One too many in proof 2: should fail.
	base.Dlnproof_1 = makeProof(correctLen)
	base.Dlnproof_2 = makeProof(correctLen + 1)
	assert.False(t, base.ValidateBasic(), "DLN proof 2 with one too many elements should fail")
}

// TestKGRound1MessageDLNProofNilElement verifies that a DLN proof with the
// correct number of parts but one nil element fails ValidateBasic.
func TestKGRound1MessageDLNProofNilElement(t *testing.T) {
	dlnParts := 2 + (dlnproof.Iterations * 2)
	proof := make([][]byte, dlnParts)
	for i := range proof {
		proof[i] = []byte{0x01}
	}
	// Set one element to nil.
	proof[5] = nil

	msg := &KGRound1Message{
		Commitment: []byte{0x01},
		PaillierN:  []byte{0x01},
		NTilde:     []byte{0x01},
		H1:         []byte{0x01},
		H2:         []byte{0x01},
		Dlnproof_1: proof,
		Dlnproof_2: makeDummyKGRound1Message().Dlnproof_2,
	}
	assert.False(t, msg.ValidateBasic(),
		"DLN proof with nil element should fail NonEmptyMultiBytes check")
}

// TestKGRound3MessageNilPaillierProof verifies that ValidateBasic rejects
// a KGRound3Message with nil PaillierProof (unlike DLN/MOD/FAC, Paillier
// proof is always required).
func TestKGRound3MessageNilPaillierProof(t *testing.T) {
	msg := &KGRound3Message{PaillierProof: nil}
	assert.False(t, msg.ValidateBasic(),
		"nil PaillierProof should fail ValidateBasic (always required)")

	msg.PaillierProof = [][]byte{}
	assert.False(t, msg.ValidateBasic(),
		"empty PaillierProof should fail ValidateBasic (always required)")
}

// TestKGRound1MessageProtoRoundTrip verifies that KGRound1Message survives
// a protobuf marshal/unmarshal cycle with both DLN proofs present and absent.
func TestKGRound1MessageProtoRoundTrip(t *testing.T) {
	t.Run("with DLN proofs", func(t *testing.T) {
		original := makeDummyKGRound1Message()
		data, err := proto.Marshal(original)
		assert.NoError(t, err)

		recovered := &KGRound1Message{}
		err = proto.Unmarshal(data, recovered)
		assert.NoError(t, err)

		assert.Equal(t, original.Commitment, recovered.Commitment)
		assert.Equal(t, original.PaillierN, recovered.PaillierN)
		assert.Equal(t, original.NTilde, recovered.NTilde)
		assert.Equal(t, original.H1, recovered.H1)
		assert.Equal(t, original.H2, recovered.H2)
		assert.Equal(t, len(original.Dlnproof_1), len(recovered.Dlnproof_1))
		assert.Equal(t, len(original.Dlnproof_2), len(recovered.Dlnproof_2))
		assert.True(t, recovered.ValidateBasic(), "recovered message should pass ValidateBasic")
	})

	t.Run("without DLN proofs (on-chain mode)", func(t *testing.T) {
		original := &KGRound1Message{
			Commitment: []byte{0x01},
			PaillierN:  []byte{0x01},
			NTilde:     []byte{0x01},
			H1:         []byte{0x01},
			H2:         []byte{0x01},
			Dlnproof_1: nil,
			Dlnproof_2: nil,
		}
		data, err := proto.Marshal(original)
		assert.NoError(t, err)

		recovered := &KGRound1Message{}
		err = proto.Unmarshal(data, recovered)
		assert.NoError(t, err)

		// In proto3, nil/empty bytes are indistinguishable.
		assert.True(t, recovered.ValidateBasic(),
			"recovered message without DLN proofs should pass ValidateBasic")
	})
}
