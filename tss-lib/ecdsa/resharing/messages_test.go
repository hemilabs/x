// Copyright © 2024 Hemi Labs, Inc.
//
// This file is part of the hemi tss-lib fork. See LICENSE for terms.

package resharing

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hemilabs/x/tss-lib/v2/crypto/dlnproof"
	"github.com/hemilabs/x/tss-lib/v2/crypto/facproof"
	"github.com/hemilabs/x/tss-lib/v2/crypto/modproof"
)

// --- helpers ---

func makeDummyDLNProof() [][]byte {
	parts := make([][]byte, 2+(dlnproof.Iterations*2))
	for i := range parts {
		parts[i] = []byte{0x01}
	}
	return parts
}

func makeDummyModProof() [][]byte {
	parts := make([][]byte, modproof.ProofModBytesParts)
	for i := range parts {
		parts[i] = []byte{0x01}
	}
	return parts
}

func makeDummyFacProof() [][]byte {
	parts := make([][]byte, facproof.ProofFacBytesParts)
	for i := range parts {
		parts[i] = []byte{0x01}
	}
	return parts
}

// --- DGRound1Message ---

func TestDGRound1MessageValidateBasicRejectsNil(t *testing.T) {
	var m *DGRound1Message
	assert.False(t, m.ValidateBasic())
}

func TestDGRound1MessageValidateBasicAcceptsValid(t *testing.T) {
	m := &DGRound1Message{
		EcdsaPubX:   make([]byte, 32),
		EcdsaPubY:   make([]byte, 32),
		VCommitment: make([]byte, 32),
		Ssid:        make([]byte, 64),
	}
	m.EcdsaPubX[0] = 0x01
	m.EcdsaPubY[0] = 0x01
	m.VCommitment[0] = 0x01
	m.Ssid[0] = 0x01
	assert.True(t, m.ValidateBasic())
}

func TestDGRound1MessageValidateBasicRejectsEmptyEcdsaPubX(t *testing.T) {
	m := &DGRound1Message{
		EcdsaPubX:   []byte{},
		EcdsaPubY:   []byte{0x01},
		VCommitment: []byte{0x01},
		Ssid:        []byte{0x01},
	}
	assert.False(t, m.ValidateBasic())
}

func TestDGRound1MessageValidateBasicRejectsOversizedEcdsaPubX(t *testing.T) {
	m := &DGRound1Message{
		EcdsaPubX:   make([]byte, 34),
		EcdsaPubY:   []byte{0x01},
		VCommitment: []byte{0x01},
		Ssid:        []byte{0x01},
	}
	m.EcdsaPubX[0] = 0x01
	assert.False(t, m.ValidateBasic())
}

func TestDGRound1MessageValidateBasicRejectsEmptySsid(t *testing.T) {
	m := &DGRound1Message{
		EcdsaPubX:   []byte{0x01},
		EcdsaPubY:   []byte{0x01},
		VCommitment: []byte{0x01},
		Ssid:        []byte{},
	}
	assert.False(t, m.ValidateBasic())
}

func TestDGRound1MessageValidateBasicRejectsOversizedSsid(t *testing.T) {
	oversized := make([]byte, 257)
	oversized[0] = 0x01
	m := &DGRound1Message{
		EcdsaPubX:   []byte{0x01},
		EcdsaPubY:   []byte{0x01},
		VCommitment: []byte{0x01},
		Ssid:        oversized,
	}
	assert.False(t, m.ValidateBasic())
}

// --- DGRound2Message1 ---

func TestDGRound2Message1ValidateBasicRejectsNil(t *testing.T) {
	var m *DGRound2Message1
	assert.False(t, m.ValidateBasic())
}

func TestDGRound2Message1ValidateBasicAcceptsValid(t *testing.T) {
	m := &DGRound2Message1{
		PaillierN:  []byte{0x01},
		NTilde:     []byte{0x01},
		H1:         []byte{0x01},
		H2:         []byte{0x01},
		ModProof:   makeDummyModProof(),
		Dlnproof_1: makeDummyDLNProof(),
		Dlnproof_2: makeDummyDLNProof(),
	}
	assert.True(t, m.ValidateBasic())
}

func TestDGRound2Message1ValidateBasicAcceptsNilProofs(t *testing.T) {
	m := &DGRound2Message1{
		PaillierN:  []byte{0x01},
		NTilde:     []byte{0x01},
		H1:         []byte{0x01},
		H2:         []byte{0x01},
		ModProof:   nil,
		Dlnproof_1: nil,
		Dlnproof_2: nil,
	}
	assert.True(t, m.ValidateBasic())
}

func TestDGRound2Message1ValidateBasicRejectsEmptyPaillierN(t *testing.T) {
	m := &DGRound2Message1{
		PaillierN: []byte{},
		NTilde:    []byte{0x01},
		H1:        []byte{0x01},
		H2:        []byte{0x01},
	}
	assert.False(t, m.ValidateBasic())
}

func TestDGRound2Message1ValidateBasicRejectsWrongModProofLength(t *testing.T) {
	badProof := make([][]byte, modproof.ProofModBytesParts+1)
	for i := range badProof {
		badProof[i] = []byte{0x01}
	}
	m := &DGRound2Message1{
		PaillierN: []byte{0x01},
		NTilde:    []byte{0x01},
		H1:        []byte{0x01},
		H2:        []byte{0x01},
		ModProof:  badProof,
	}
	assert.False(t, m.ValidateBasic())
}

// --- DGRound2Message2 ---

func TestDGRound2Message2ValidateBasicRejectsNil(t *testing.T) {
	// KEY: upstream returned true unconditionally; fork requires m != nil
	var m *DGRound2Message2
	assert.False(t, m.ValidateBasic())
}

func TestDGRound2Message2ValidateBasicAcceptsValid(t *testing.T) {
	m := &DGRound2Message2{}
	assert.True(t, m.ValidateBasic())
}

// --- DGRound3Message1 ---

func TestDGRound3Message1ValidateBasicRejectsNil(t *testing.T) {
	var m *DGRound3Message1
	assert.False(t, m.ValidateBasic())
}

func TestDGRound3Message1ValidateBasicAcceptsValid(t *testing.T) {
	m := &DGRound3Message1{
		Share:      []byte{0x01},
		ReceiverId: []byte{0x01},
	}
	assert.True(t, m.ValidateBasic())
}

func TestDGRound3Message1ValidateBasicRejectsEmptyShare(t *testing.T) {
	m := &DGRound3Message1{
		Share:      []byte{},
		ReceiverId: []byte{0x01},
	}
	assert.False(t, m.ValidateBasic())
}

func TestDGRound3Message1ValidateBasicRejectsOversizedShare(t *testing.T) {
	oversized := make([]byte, 33)
	oversized[0] = 0x01
	m := &DGRound3Message1{
		Share:      oversized,
		ReceiverId: []byte{0x01},
	}
	assert.False(t, m.ValidateBasic())
}

func TestDGRound3Message1ValidateBasicRejectsEmptyReceiverId(t *testing.T) {
	m := &DGRound3Message1{
		Share:      []byte{0x01},
		ReceiverId: []byte{},
	}
	assert.False(t, m.ValidateBasic())
}

// --- DGRound3Message2 ---

func TestDGRound3Message2ValidateBasicRejectsNil(t *testing.T) {
	var m *DGRound3Message2
	assert.False(t, m.ValidateBasic())
}

func TestDGRound3Message2ValidateBasicAcceptsValid(t *testing.T) {
	m := &DGRound3Message2{
		VDecommitment: [][]byte{
			{0x01},
			{0x02},
		},
	}
	assert.True(t, m.ValidateBasic())
}

func TestDGRound3Message2ValidateBasicRejectsTooManyElements(t *testing.T) {
	parts := make([][]byte, 601)
	for i := range parts {
		parts[i] = []byte{0x01}
	}
	m := &DGRound3Message2{
		VDecommitment: parts,
	}
	assert.False(t, m.ValidateBasic())
}

func TestDGRound3Message2ValidateBasicRejectsOversizedElement(t *testing.T) {
	oversized := make([]byte, 34)
	oversized[0] = 0x01
	m := &DGRound3Message2{
		VDecommitment: [][]byte{
			{0x01},
			oversized,
		},
	}
	assert.False(t, m.ValidateBasic())
}

// --- DGRound4Message1 ---

func TestDGRound4Message1ValidateBasicRejectsNil(t *testing.T) {
	var m *DGRound4Message1
	assert.False(t, m.ValidateBasic())
}

func TestDGRound4Message1ValidateBasicAcceptsValid(t *testing.T) {
	m := &DGRound4Message1{
		FacProof:   makeDummyFacProof(),
		ReceiverId: []byte{0x01},
	}
	assert.True(t, m.ValidateBasic())
}

func TestDGRound4Message1ValidateBasicAcceptsNilFacProof(t *testing.T) {
	m := &DGRound4Message1{
		FacProof:   nil,
		ReceiverId: []byte{0x01},
	}
	assert.True(t, m.ValidateBasic())
}

func TestDGRound4Message1ValidateBasicRejectsEmptyReceiverId(t *testing.T) {
	m := &DGRound4Message1{
		FacProof:   nil,
		ReceiverId: []byte{},
	}
	assert.False(t, m.ValidateBasic())
}

func TestDGRound4Message1ValidateBasicRejectsWrongFacProofLength(t *testing.T) {
	badProof := make([][]byte, facproof.ProofFacBytesParts+1)
	for i := range badProof {
		badProof[i] = []byte{0x01}
	}
	m := &DGRound4Message1{
		FacProof:   badProof,
		ReceiverId: []byte{0x01},
	}
	assert.False(t, m.ValidateBasic())
}

// --- DGRound4Message2 ---

func TestDGRound4Message2ValidateBasicRejectsNil(t *testing.T) {
	// KEY: upstream returned true unconditionally; fork requires m != nil
	var m *DGRound4Message2
	assert.False(t, m.ValidateBasic())
}

func TestDGRound4Message2ValidateBasicAcceptsValid(t *testing.T) {
	m := &DGRound4Message2{}
	assert.True(t, m.ValidateBasic())
}
