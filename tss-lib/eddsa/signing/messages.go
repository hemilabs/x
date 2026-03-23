// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package signing

import (
	"math/big"

	cmt "github.com/hemilabs/x/tss-lib/v3/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v3/crypto/schnorr"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// SignRound1Message is broadcast: commitment to Ri.
type SignRound1Message struct {
	Commitment *big.Int
}

// ValidateBasic checks that required fields of SignRound1Message are non-nil.
func (m *SignRound1Message) ValidateBasic() bool {
	return m != nil && m.Commitment != nil && m.Commitment.Sign() > 0
}

// NewSignRound1Message constructs a *tss.Message with the given content.
func NewSignRound1Message(from *tss.PartyID, commitment cmt.HashCommitment) *tss.Message {
	return &tss.Message{
		From:        from,
		IsBroadcast: true,
		Content: &SignRound1Message{
			Commitment: commitment,
		},
	}
}

// SignRound2Message is broadcast: decommitment + Schnorr proof for ri.
type SignRound2Message struct {
	DeCommitment cmt.HashDeCommitment
	ZKProof      *schnorr.ZKProof
}

// ValidateBasic checks that required fields of SignRound2Message are non-nil.
func (m *SignRound2Message) ValidateBasic() bool {
	return m != nil && len(m.DeCommitment) >= 2 && m.ZKProof != nil
}

// NewSignRound2Message constructs a *tss.Message with the given content.
func NewSignRound2Message(from *tss.PartyID, deCommitment cmt.HashDeCommitment, proof *schnorr.ZKProof) *tss.Message {
	return &tss.Message{
		From:        from,
		IsBroadcast: true,
		Content: &SignRound2Message{
			DeCommitment: deCommitment,
			ZKProof:      proof,
		},
	}
}

// SignRound3Message is broadcast: partial signature si.
type SignRound3Message struct {
	S *big.Int
}

// ValidateBasic checks that required fields of SignRound3Message are non-nil.
func (m *SignRound3Message) ValidateBasic() bool {
	return m != nil && m.S != nil && m.S.Sign() > 0
}

// NewSignRound3Message constructs a *tss.Message with the given content.
func NewSignRound3Message(from *tss.PartyID, si *big.Int) *tss.Message {
	return &tss.Message{
		From:        from,
		IsBroadcast: true,
		Content: &SignRound3Message{
			S: si,
		},
	}
}
