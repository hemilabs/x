package resharing

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- DGRound1Message ---

func TestDGRound1MessageValidateBasicRejectsNil(t *testing.T) {
	var msg *DGRound1Message
	assert.False(t, msg.ValidateBasic(), "nil message should fail ValidateBasic")
}

func TestDGRound1MessageValidateBasicAcceptsValid(t *testing.T) {
	msg := &DGRound1Message{
		EddsaPubX:   []byte{0x01},
		EddsaPubY:   []byte{0x01},
		VCommitment: []byte{0x01},
	}
	assert.True(t, msg.ValidateBasic())
}

func TestDGRound1MessageValidateBasicRejectsEmptyEddsaPubX(t *testing.T) {
	msg := &DGRound1Message{
		EddsaPubX:   nil,
		EddsaPubY:   []byte{0x01},
		VCommitment: []byte{0x01},
	}
	assert.False(t, msg.ValidateBasic())
}

func TestDGRound1MessageValidateBasicRejectsOversizedEddsaPubX(t *testing.T) {
	oversized := make([]byte, 33)
	oversized[0] = 0x01
	msg := &DGRound1Message{
		EddsaPubX:   oversized,
		EddsaPubY:   []byte{0x01},
		VCommitment: []byte{0x01},
	}
	assert.False(t, msg.ValidateBasic())
}

// --- DGRound2Message ---

func TestDGRound2MessageValidateBasicRejectsNil(t *testing.T) {
	// KEY: upstream returned true unconditionally
	var msg *DGRound2Message
	assert.False(t, msg.ValidateBasic(), "nil message should fail (was upstream bug)")
}

func TestDGRound2MessageValidateBasicAcceptsValid(t *testing.T) {
	msg := &DGRound2Message{}
	assert.True(t, msg.ValidateBasic())
}

// --- DGRound3Message1 ---

func TestDGRound3Message1ValidateBasicRejectsNil(t *testing.T) {
	var msg *DGRound3Message1
	assert.False(t, msg.ValidateBasic(), "nil message should fail ValidateBasic")
}

func TestDGRound3Message1ValidateBasicAcceptsValid(t *testing.T) {
	msg := &DGRound3Message1{
		Share:      []byte{0x01},
		ReceiverId: []byte{0x01, 0x02},
	}
	assert.True(t, msg.ValidateBasic())
}

func TestDGRound3Message1ValidateBasicRejectsEmptyShare(t *testing.T) {
	msg := &DGRound3Message1{
		Share:      nil,
		ReceiverId: []byte{0x01, 0x02},
	}
	assert.False(t, msg.ValidateBasic())
}

func TestDGRound3Message1ValidateBasicRejectsOversizedShare(t *testing.T) {
	oversized := make([]byte, 33)
	oversized[0] = 0x01
	msg := &DGRound3Message1{
		Share:      oversized,
		ReceiverId: []byte{0x01, 0x02},
	}
	assert.False(t, msg.ValidateBasic())
}

func TestDGRound3Message1ValidateBasicRejectsEmptyReceiverId(t *testing.T) {
	msg := &DGRound3Message1{
		Share:      []byte{0x01},
		ReceiverId: nil,
	}
	assert.False(t, msg.ValidateBasic())
}

// --- DGRound3Message2 ---

func TestDGRound3Message2ValidateBasicRejectsNil(t *testing.T) {
	var msg *DGRound3Message2
	assert.False(t, msg.ValidateBasic(), "nil message should fail ValidateBasic")
}

func TestDGRound3Message2ValidateBasicAcceptsValid(t *testing.T) {
	msg := &DGRound3Message2{
		VDecommitment: [][]byte{{0x01}, {0x02}},
	}
	assert.True(t, msg.ValidateBasic())
}

func TestDGRound3Message2ValidateBasicRejectsEmptyVDecommitment(t *testing.T) {
	msg := &DGRound3Message2{
		VDecommitment: nil,
	}
	assert.False(t, msg.ValidateBasic())
}

// --- DGRound4Message ---

func TestDGRound4MessageValidateBasicRejectsNil(t *testing.T) {
	// KEY: upstream returned true unconditionally
	var msg *DGRound4Message
	assert.False(t, msg.ValidateBasic(), "nil message should fail (was upstream bug)")
}

func TestDGRound4MessageValidateBasicAcceptsValid(t *testing.T) {
	msg := &DGRound4Message{}
	assert.True(t, msg.ValidateBasic())
}
