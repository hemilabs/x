// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package signing

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hemilabs/x/tss-lib/v2/crypto/mta"
)

// --- helpers ---

func makeDummyRangeProofAlice() [][]byte {
	parts := make([][]byte, mta.RangeProofAliceBytesParts)
	for i := range parts {
		parts[i] = []byte{0x01}
	}
	return parts
}

func makeDummyProofBob() [][]byte {
	parts := make([][]byte, mta.ProofBobBytesParts)
	for i := range parts {
		parts[i] = []byte{0x01}
	}
	return parts
}

func makeDummyProofBobWC() [][]byte {
	parts := make([][]byte, mta.ProofBobWCBytesParts)
	for i := range parts {
		parts[i] = []byte{0x01}
	}
	return parts
}

func makeOversized(limit int) []byte {
	b := make([]byte, limit+1)
	b[0] = 0x01
	return b
}

// ============================================================
// SignRound1Message1 (P2P)
// ============================================================

func TestSignRound1Message1ValidateBasicRejectsNil(t *testing.T) {
	var msg *SignRound1Message1
	assert.False(t, msg.ValidateBasic())
}

func TestSignRound1Message1ValidateBasicAcceptsValid(t *testing.T) {
	msg := &SignRound1Message1{
		C:               []byte{0x01},
		RangeProofAlice: makeDummyRangeProofAlice(),
		ReceiverId:      []byte{0x01},
	}
	assert.True(t, msg.ValidateBasic())
}

func TestSignRound1Message1ValidateBasicRejectsEmptyC(t *testing.T) {
	msg := &SignRound1Message1{
		C:               []byte{},
		RangeProofAlice: makeDummyRangeProofAlice(),
		ReceiverId:      []byte{0x01},
	}
	assert.False(t, msg.ValidateBasic())
}

func TestSignRound1Message1ValidateBasicRejectsOversizedC(t *testing.T) {
	msg := &SignRound1Message1{
		C:               makeOversized(1024),
		RangeProofAlice: makeDummyRangeProofAlice(),
		ReceiverId:      []byte{0x01},
	}
	assert.False(t, msg.ValidateBasic())
}

func TestSignRound1Message1ValidateBasicRejectsEmptyReceiverId(t *testing.T) {
	msg := &SignRound1Message1{
		C:               []byte{0x01},
		RangeProofAlice: makeDummyRangeProofAlice(),
		ReceiverId:      []byte{},
	}
	assert.False(t, msg.ValidateBasic())
}

func TestSignRound1Message1ValidateBasicRejectsWrongProofLength(t *testing.T) {
	// Provide wrong number of proof parts (5 instead of 6).
	badProof := make([][]byte, mta.RangeProofAliceBytesParts-1)
	for i := range badProof {
		badProof[i] = []byte{0x01}
	}
	msg := &SignRound1Message1{
		C:               []byte{0x01},
		RangeProofAlice: badProof,
		ReceiverId:      []byte{0x01},
	}
	assert.False(t, msg.ValidateBasic())
}

// ============================================================
// SignRound1Message2 (broadcast)
// ============================================================

func TestSignRound1Message2ValidateBasicRejectsNil(t *testing.T) {
	var msg *SignRound1Message2
	assert.False(t, msg.ValidateBasic())
}

func TestSignRound1Message2ValidateBasicAcceptsValid(t *testing.T) {
	msg := &SignRound1Message2{
		Commitment: []byte{0x01},
	}
	assert.True(t, msg.ValidateBasic())
}

func TestSignRound1Message2ValidateBasicRejectsEmptyCommitment(t *testing.T) {
	msg := &SignRound1Message2{
		Commitment: []byte{},
	}
	assert.False(t, msg.ValidateBasic())
}

func TestSignRound1Message2ValidateBasicRejectsOversizedCommitment(t *testing.T) {
	msg := &SignRound1Message2{
		Commitment: makeOversized(32),
	}
	assert.False(t, msg.ValidateBasic())
}

// ============================================================
// SignRound2Message (P2P)
// ============================================================

func TestSignRound2MessageValidateBasicRejectsNil(t *testing.T) {
	var msg *SignRound2Message
	assert.False(t, msg.ValidateBasic())
}

func TestSignRound2MessageValidateBasicAcceptsValid(t *testing.T) {
	msg := &SignRound2Message{
		C1:         []byte{0x01},
		C2:         []byte{0x01},
		ProofBob:   makeDummyProofBob(),
		ProofBobWc: makeDummyProofBobWC(),
		ReceiverId: []byte{0x01},
	}
	assert.True(t, msg.ValidateBasic())
}

func TestSignRound2MessageValidateBasicRejectsEmptyC1(t *testing.T) {
	msg := &SignRound2Message{
		C1:         []byte{},
		C2:         []byte{0x01},
		ProofBob:   makeDummyProofBob(),
		ProofBobWc: makeDummyProofBobWC(),
		ReceiverId: []byte{0x01},
	}
	assert.False(t, msg.ValidateBasic())
}

func TestSignRound2MessageValidateBasicRejectsOversizedC1(t *testing.T) {
	msg := &SignRound2Message{
		C1:         makeOversized(1024),
		C2:         []byte{0x01},
		ProofBob:   makeDummyProofBob(),
		ProofBobWc: makeDummyProofBobWC(),
		ReceiverId: []byte{0x01},
	}
	assert.False(t, msg.ValidateBasic())
}

func TestSignRound2MessageValidateBasicRejectsEmptyReceiverId(t *testing.T) {
	msg := &SignRound2Message{
		C1:         []byte{0x01},
		C2:         []byte{0x01},
		ProofBob:   makeDummyProofBob(),
		ProofBobWc: makeDummyProofBobWC(),
		ReceiverId: []byte{},
	}
	assert.False(t, msg.ValidateBasic())
}

// ============================================================
// SignRound3Message (broadcast)
// ============================================================

func TestSignRound3MessageValidateBasicRejectsNil(t *testing.T) {
	var msg *SignRound3Message
	assert.False(t, msg.ValidateBasic())
}

func TestSignRound3MessageValidateBasicAcceptsValid(t *testing.T) {
	msg := &SignRound3Message{
		Theta: []byte{0x01},
	}
	assert.True(t, msg.ValidateBasic())
}

func TestSignRound3MessageValidateBasicRejectsEmptyTheta(t *testing.T) {
	msg := &SignRound3Message{
		Theta: []byte{},
	}
	assert.False(t, msg.ValidateBasic())
}

func TestSignRound3MessageValidateBasicRejectsOversizedTheta(t *testing.T) {
	msg := &SignRound3Message{
		Theta: makeOversized(32),
	}
	assert.False(t, msg.ValidateBasic())
}

// ============================================================
// SignRound4Message (broadcast)
// ============================================================

func TestSignRound4MessageValidateBasicRejectsNil(t *testing.T) {
	var msg *SignRound4Message
	assert.False(t, msg.ValidateBasic())
}

func TestSignRound4MessageValidateBasicAcceptsValid(t *testing.T) {
	msg := &SignRound4Message{
		DeCommitment: [][]byte{{0x01}, {0x02}, {0x03}},
		ProofAlphaX:  []byte{0x01},
		ProofAlphaY:  []byte{0x01},
		ProofT:       []byte{0x01},
	}
	assert.True(t, msg.ValidateBasic())
}

func TestSignRound4MessageValidateBasicRejectsWrongDecommitmentCount(t *testing.T) {
	// Only 2 elements instead of required 3.
	msg := &SignRound4Message{
		DeCommitment: [][]byte{{0x01}, {0x02}},
		ProofAlphaX:  []byte{0x01},
		ProofAlphaY:  []byte{0x01},
		ProofT:       []byte{0x01},
	}
	assert.False(t, msg.ValidateBasic())
}

func TestSignRound4MessageValidateBasicRejectsOversizedDecommitmentElement(t *testing.T) {
	// One element is 34 bytes (> 33 limit).
	msg := &SignRound4Message{
		DeCommitment: [][]byte{{0x01}, makeOversized(33), {0x03}},
		ProofAlphaX:  []byte{0x01},
		ProofAlphaY:  []byte{0x01},
		ProofT:       []byte{0x01},
	}
	assert.False(t, msg.ValidateBasic())
}

// ============================================================
// SignRound5Message (broadcast)
// ============================================================

func TestSignRound5MessageValidateBasicRejectsNil(t *testing.T) {
	var msg *SignRound5Message
	assert.False(t, msg.ValidateBasic())
}

func TestSignRound5MessageValidateBasicAcceptsValid(t *testing.T) {
	msg := &SignRound5Message{
		Commitment: []byte{0x01},
	}
	assert.True(t, msg.ValidateBasic())
}

// ============================================================
// SignRound6Message (broadcast)
// ============================================================

func TestSignRound6MessageValidateBasicRejectsNil(t *testing.T) {
	var msg *SignRound6Message
	assert.False(t, msg.ValidateBasic())
}

func TestSignRound6MessageValidateBasicAcceptsValid(t *testing.T) {
	msg := &SignRound6Message{
		DeCommitment: [][]byte{{0x01}, {0x02}, {0x03}, {0x04}, {0x05}},
		ProofAlphaX:  []byte{0x01},
		ProofAlphaY:  []byte{0x01},
		ProofT:       []byte{0x01},
		VProofAlphaX: []byte{0x01},
		VProofAlphaY: []byte{0x01},
		VProofT:      []byte{0x01},
		VProofU:      []byte{0x01},
	}
	assert.True(t, msg.ValidateBasic())
}

func TestSignRound6MessageValidateBasicRejectsEmptyVProofT(t *testing.T) {
	msg := &SignRound6Message{
		DeCommitment: [][]byte{{0x01}, {0x02}, {0x03}, {0x04}, {0x05}},
		ProofAlphaX:  []byte{0x01},
		ProofAlphaY:  []byte{0x01},
		ProofT:       []byte{0x01},
		VProofAlphaX: []byte{0x01},
		VProofAlphaY: []byte{0x01},
		VProofT:      []byte{},
		VProofU:      []byte{0x01},
	}
	assert.False(t, msg.ValidateBasic())
}

// ============================================================
// SignRound7Message (broadcast)
// ============================================================

func TestSignRound7MessageValidateBasicRejectsNil(t *testing.T) {
	var msg *SignRound7Message
	assert.False(t, msg.ValidateBasic())
}

func TestSignRound7MessageValidateBasicAcceptsValid(t *testing.T) {
	msg := &SignRound7Message{
		Commitment: []byte{0x01},
	}
	assert.True(t, msg.ValidateBasic())
}

// ============================================================
// SignRound8Message (broadcast)
// ============================================================

func TestSignRound8MessageValidateBasicRejectsNil(t *testing.T) {
	var msg *SignRound8Message
	assert.False(t, msg.ValidateBasic())
}

func TestSignRound8MessageValidateBasicAcceptsValid(t *testing.T) {
	msg := &SignRound8Message{
		DeCommitment: [][]byte{{0x01}, {0x02}, {0x03}, {0x04}, {0x05}},
	}
	assert.True(t, msg.ValidateBasic())
}

func TestSignRound8MessageValidateBasicRejectsOversizedElement(t *testing.T) {
	// One element is 34 bytes (> 33 limit).
	msg := &SignRound8Message{
		DeCommitment: [][]byte{{0x01}, {0x02}, makeOversized(33), {0x04}, {0x05}},
	}
	assert.False(t, msg.ValidateBasic())
}

// ============================================================
// SignRound9Message (broadcast)
// ============================================================

func TestSignRound9MessageValidateBasicRejectsNil(t *testing.T) {
	var msg *SignRound9Message
	assert.False(t, msg.ValidateBasic())
}

func TestSignRound9MessageValidateBasicAcceptsValid(t *testing.T) {
	msg := &SignRound9Message{
		S: []byte{0x01},
	}
	assert.True(t, msg.ValidateBasic())
}

func TestSignRound9MessageValidateBasicRejectsEmptyS(t *testing.T) {
	msg := &SignRound9Message{
		S: []byte{},
	}
	assert.False(t, msg.ValidateBasic())
}

func TestSignRound9MessageValidateBasicRejectsOversizedS(t *testing.T) {
	msg := &SignRound9Message{
		S: makeOversized(32),
	}
	assert.False(t, msg.ValidateBasic())
}
