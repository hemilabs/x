// Copyright © 2024 Hemi Labs, Inc.
//
// This file is part of the hemi tss-lib fork. See LICENSE for terms.

package keygen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- KGRound1Message ---

func TestKGRound1MessageValidateBasicRejectsNil(t *testing.T) {
	var m *KGRound1Message
	assert.False(t, m.ValidateBasic())
}

func TestKGRound1MessageValidateBasicAcceptsValid(t *testing.T) {
	m := &KGRound1Message{
		Commitment: []byte{0x01},
	}
	assert.True(t, m.ValidateBasic())
}

func TestKGRound1MessageValidateBasicRejectsEmptyCommitment(t *testing.T) {
	m := &KGRound1Message{
		Commitment: []byte{},
	}
	assert.False(t, m.ValidateBasic())
}

func TestKGRound1MessageValidateBasicRejectsOversizedCommitment(t *testing.T) {
	oversized := make([]byte, 33)
	oversized[0] = 0x01
	m := &KGRound1Message{
		Commitment: oversized,
	}
	assert.False(t, m.ValidateBasic())
}

// --- KGRound2Message1 ---

func TestKGRound2Message1ValidateBasicRejectsNil(t *testing.T) {
	var m *KGRound2Message1
	assert.False(t, m.ValidateBasic())
}

func TestKGRound2Message1ValidateBasicAcceptsValid(t *testing.T) {
	m := &KGRound2Message1{
		Share:      []byte{0x01},
		ReceiverId: []byte{0x01},
	}
	assert.True(t, m.ValidateBasic())
}

func TestKGRound2Message1ValidateBasicRejectsEmptyShare(t *testing.T) {
	m := &KGRound2Message1{
		Share:      []byte{},
		ReceiverId: []byte{0x01},
	}
	assert.False(t, m.ValidateBasic())
}

func TestKGRound2Message1ValidateBasicRejectsOversizedShare(t *testing.T) {
	oversized := make([]byte, 33)
	oversized[0] = 0x01
	m := &KGRound2Message1{
		Share:      oversized,
		ReceiverId: []byte{0x01},
	}
	assert.False(t, m.ValidateBasic())
}

func TestKGRound2Message1ValidateBasicRejectsEmptyReceiverId(t *testing.T) {
	m := &KGRound2Message1{
		Share:      []byte{0x01},
		ReceiverId: []byte{},
	}
	assert.False(t, m.ValidateBasic())
}

// --- KGRound2Message2 ---

func TestKGRound2Message2ValidateBasicRejectsNil(t *testing.T) {
	var m *KGRound2Message2
	assert.False(t, m.ValidateBasic())
}

func TestKGRound2Message2ValidateBasicAcceptsValid(t *testing.T) {
	m := &KGRound2Message2{
		DeCommitment: [][]byte{{0x01}, {0x02}},
		ProofAlphaX:  []byte{0x01},
		ProofAlphaY:  []byte{0x01},
		ProofT:       []byte{0x01},
	}
	assert.True(t, m.ValidateBasic())
}

func TestKGRound2Message2ValidateBasicRejectsEmptyProofAlphaX(t *testing.T) {
	m := &KGRound2Message2{
		DeCommitment: [][]byte{{0x01}},
		ProofAlphaX:  []byte{},
		ProofAlphaY:  []byte{0x01},
		ProofT:       []byte{0x01},
	}
	assert.False(t, m.ValidateBasic())
}

func TestKGRound2Message2ValidateBasicRejectsOversizedProofAlphaX(t *testing.T) {
	oversized := make([]byte, 33)
	oversized[0] = 0x01
	m := &KGRound2Message2{
		DeCommitment: [][]byte{{0x01}},
		ProofAlphaX:  oversized,
		ProofAlphaY:  []byte{0x01},
		ProofT:       []byte{0x01},
	}
	assert.False(t, m.ValidateBasic())
}

func TestKGRound2Message2ValidateBasicRejectsOversizedProofT(t *testing.T) {
	oversized := make([]byte, 33)
	oversized[0] = 0x01
	m := &KGRound2Message2{
		DeCommitment: [][]byte{{0x01}},
		ProofAlphaX:  []byte{0x01},
		ProofAlphaY:  []byte{0x01},
		ProofT:       oversized,
	}
	assert.False(t, m.ValidateBasic())
}
