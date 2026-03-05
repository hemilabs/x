package signing

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- SignRound1Message ---

func TestSignRound1MessageValidateBasicRejectsNil(t *testing.T) {
	// KEY TEST: upstream panicked on nil receiver. Fork fixes this.
	var msg *SignRound1Message
	assert.False(t, msg.ValidateBasic(), "nil message should fail ValidateBasic")
}

func TestSignRound1MessageValidateBasicAcceptsValid(t *testing.T) {
	msg := &SignRound1Message{Commitment: []byte{0x01}}
	assert.True(t, msg.ValidateBasic())
}

func TestSignRound1MessageValidateBasicRejectsEmptyCommitment(t *testing.T) {
	msg := &SignRound1Message{Commitment: nil}
	assert.False(t, msg.ValidateBasic())
}

func TestSignRound1MessageValidateBasicRejectsOversizedCommitment(t *testing.T) {
	oversized := make([]byte, 33)
	oversized[0] = 0x01
	msg := &SignRound1Message{Commitment: oversized}
	assert.False(t, msg.ValidateBasic())
}

// --- SignRound2Message ---

func TestSignRound2MessageValidateBasicRejectsNil(t *testing.T) {
	var msg *SignRound2Message
	assert.False(t, msg.ValidateBasic(), "nil message should fail ValidateBasic")
}

func TestSignRound2MessageValidateBasicAcceptsValid(t *testing.T) {
	msg := &SignRound2Message{
		DeCommitment: [][]byte{{0x01}, {0x02}, {0x03}},
		ProofAlphaX:  []byte{0x01},
		ProofAlphaY:  []byte{0x01},
		ProofT:       []byte{0x01},
	}
	assert.True(t, msg.ValidateBasic())
}

func TestSignRound2MessageValidateBasicRejectsWrongDecommitmentCount(t *testing.T) {
	msg := &SignRound2Message{
		DeCommitment: [][]byte{{0x01}, {0x02}},
		ProofAlphaX:  []byte{0x01},
		ProofAlphaY:  []byte{0x01},
		ProofT:       []byte{0x01},
	}
	assert.False(t, msg.ValidateBasic())
}

func TestSignRound2MessageValidateBasicRejectsEmptyProofAlphaX(t *testing.T) {
	msg := &SignRound2Message{
		DeCommitment: [][]byte{{0x01}, {0x02}, {0x03}},
		ProofAlphaX:  nil,
		ProofAlphaY:  []byte{0x01},
		ProofT:       []byte{0x01},
	}
	assert.False(t, msg.ValidateBasic())
}

func TestSignRound2MessageValidateBasicRejectsOversizedProofAlphaX(t *testing.T) {
	oversized := make([]byte, 33)
	oversized[0] = 0x01
	msg := &SignRound2Message{
		DeCommitment: [][]byte{{0x01}, {0x02}, {0x03}},
		ProofAlphaX:  oversized,
		ProofAlphaY:  []byte{0x01},
		ProofT:       []byte{0x01},
	}
	assert.False(t, msg.ValidateBasic())
}

func TestSignRound2MessageValidateBasicRejectsOversizedProofT(t *testing.T) {
	oversized := make([]byte, 33)
	oversized[0] = 0x01
	msg := &SignRound2Message{
		DeCommitment: [][]byte{{0x01}, {0x02}, {0x03}},
		ProofAlphaX:  []byte{0x01},
		ProofAlphaY:  []byte{0x01},
		ProofT:       oversized,
	}
	assert.False(t, msg.ValidateBasic())
}

// --- SignRound3Message ---

func TestSignRound3MessageValidateBasicRejectsNil(t *testing.T) {
	var msg *SignRound3Message
	assert.False(t, msg.ValidateBasic(), "nil message should fail ValidateBasic")
}

func TestSignRound3MessageValidateBasicAcceptsValid(t *testing.T) {
	msg := &SignRound3Message{S: []byte{0x01}}
	assert.True(t, msg.ValidateBasic())
}

func TestSignRound3MessageValidateBasicRejectsEmptyS(t *testing.T) {
	msg := &SignRound3Message{S: nil}
	assert.False(t, msg.ValidateBasic())
}

func TestSignRound3MessageValidateBasicRejectsOversizedS(t *testing.T) {
	oversized := make([]byte, 33)
	oversized[0] = 0x01
	msg := &SignRound3Message{S: oversized}
	assert.False(t, msg.ValidateBasic())
}
