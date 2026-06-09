// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package signing

import (
	"math/big"

	cmt "github.com/hemilabs/x/tss/v3/crypto/commitments"
	"github.com/hemilabs/x/tss/v3/crypto/mta"
	"github.com/hemilabs/x/tss/v3/crypto/schnorr"
	"github.com/hemilabs/x/tss/v3/tss"
)

// SignRound1Message1 is a P2P message: Paillier ciphertext + range proof.
type SignRound1Message1 struct {
	C               *big.Int
	RangeProofAlice *mta.RangeProofAlice
	ReceiverID      []byte
}

// ValidateBasic checks that required fields of SignRound1Message1 are non-nil.
func (m *SignRound1Message1) ValidateBasic() bool {
	return m != nil && m.C != nil && m.C.Sign() > 0 &&
		m.RangeProofAlice != nil && len(m.ReceiverID) > 0
}

// NewSignRound1Message1 constructs a *tss.Message with the given content.
func NewSignRound1Message1(to, from *tss.PartyID, c *big.Int, proof *mta.RangeProofAlice) *tss.Message {
	return &tss.Message{
		From: from,
		To:   []*tss.PartyID{to},
		Content: &SignRound1Message1{
			C:               c,
			RangeProofAlice: proof,
			ReceiverID:      to.Key,
		},
	}
}

// SignRound1Message2 is broadcast: commitment to gamma share.
type SignRound1Message2 struct {
	Commitment *big.Int
}

// ValidateBasic checks that required fields of SignRound1Message2 are non-nil.
func (m *SignRound1Message2) ValidateBasic() bool {
	return m != nil && m.Commitment != nil && m.Commitment.Sign() > 0
}

// NewSignRound1Message2 constructs a *tss.Message with the given content.
func NewSignRound1Message2(from *tss.PartyID, commitment cmt.HashCommitment) *tss.Message {
	return &tss.Message{
		From:        from,
		IsBroadcast: true,
		Content: &SignRound1Message2{
			Commitment: commitment,
		},
	}
}

// SignRound2Message is P2P: MtA ciphertexts + Bob proofs.
type SignRound2Message struct {
	C1         *big.Int
	C2         *big.Int
	ProofBob   *mta.ProofBob
	ProofBobWC *mta.ProofBobWC
	ReceiverID []byte
}

// ValidateBasic checks that required fields of SignRound2Message are non-nil.
func (m *SignRound2Message) ValidateBasic() bool {
	return m != nil &&
		m.C1 != nil && m.C1.Sign() > 0 &&
		m.C2 != nil && m.C2.Sign() > 0 &&
		m.ProofBob != nil && m.ProofBobWC != nil &&
		len(m.ReceiverID) > 0
}

// NewSignRound2Message constructs a *tss.Message with the given content.
func NewSignRound2Message(to, from *tss.PartyID, c1Ji *big.Int, pi1Ji *mta.ProofBob, c2Ji *big.Int, pi2Ji *mta.ProofBobWC) *tss.Message {
	return &tss.Message{
		From: from,
		To:   []*tss.PartyID{to},
		Content: &SignRound2Message{
			C1:         c1Ji,
			C2:         c2Ji,
			ProofBob:   pi1Ji,
			ProofBobWC: pi2Ji,
			ReceiverID: to.Key,
		},
	}
}

// SignRound3Message is broadcast: theta share.
type SignRound3Message struct {
	Theta *big.Int
}

// ValidateBasic checks that required fields of SignRound3Message are non-nil.
func (m *SignRound3Message) ValidateBasic() bool {
	return m != nil && m.Theta != nil && m.Theta.Sign() > 0
}

// NewSignRound3Message constructs a *tss.Message with the given content.
func NewSignRound3Message(from *tss.PartyID, theta *big.Int) *tss.Message {
	return &tss.Message{
		From:        from,
		IsBroadcast: true,
		Content:     &SignRound3Message{Theta: theta},
	}
}

// SignRound4Message is broadcast: decommitment to gamma + ZK proof.
type SignRound4Message struct {
	DeCommitment cmt.HashDeCommitment
	ZKProof      *schnorr.ZKProof
}

// ValidateBasic checks that required fields of SignRound4Message are non-nil.
func (m *SignRound4Message) ValidateBasic() bool {
	return m != nil && len(m.DeCommitment) >= 2 && m.ZKProof != nil
}

// NewSignRound4Message constructs a *tss.Message with the given content.
func NewSignRound4Message(from *tss.PartyID, deCommitment cmt.HashDeCommitment, proof *schnorr.ZKProof) *tss.Message {
	return &tss.Message{
		From:        from,
		IsBroadcast: true,
		Content: &SignRound4Message{
			DeCommitment: deCommitment,
			ZKProof:      proof,
		},
	}
}

// SignRound5Message is broadcast: commitment to blinding.
type SignRound5Message struct {
	Commitment *big.Int
}

// ValidateBasic checks that required fields of SignRound5Message are non-nil.
func (m *SignRound5Message) ValidateBasic() bool {
	return m != nil && m.Commitment != nil && m.Commitment.Sign() > 0
}

// NewSignRound5Message constructs a *tss.Message with the given content.
func NewSignRound5Message(from *tss.PartyID, commitment cmt.HashCommitment) *tss.Message {
	return &tss.Message{
		From:        from,
		IsBroadcast: true,
		Content: &SignRound5Message{
			Commitment: commitment,
		},
	}
}

// SignRound6Message is broadcast: decommitment + ZK + ZKV proofs.
type SignRound6Message struct {
	DeCommitment cmt.HashDeCommitment
	ZKProof      *schnorr.ZKProof
	ZKVProof     *schnorr.ZKVProof
}

// ValidateBasic checks that required fields of SignRound6Message are non-nil.
func (m *SignRound6Message) ValidateBasic() bool {
	return m != nil && len(m.DeCommitment) >= 2 &&
		m.ZKProof != nil && m.ZKVProof != nil
}

// NewSignRound6Message constructs a *tss.Message with the given content.
func NewSignRound6Message(from *tss.PartyID, deCommitment cmt.HashDeCommitment, proof *schnorr.ZKProof, vProof *schnorr.ZKVProof) *tss.Message {
	return &tss.Message{
		From:        from,
		IsBroadcast: true,
		Content: &SignRound6Message{
			DeCommitment: deCommitment,
			ZKProof:      proof,
			ZKVProof:     vProof,
		},
	}
}

// SignRound7Message is broadcast: commitment to Ui/Ti.
type SignRound7Message struct {
	Commitment *big.Int
}

// ValidateBasic checks that required fields of SignRound7Message are non-nil.
func (m *SignRound7Message) ValidateBasic() bool {
	return m != nil && m.Commitment != nil && m.Commitment.Sign() > 0
}

// NewSignRound7Message constructs a *tss.Message with the given content.
func NewSignRound7Message(from *tss.PartyID, commitment cmt.HashCommitment) *tss.Message {
	return &tss.Message{
		From:        from,
		IsBroadcast: true,
		Content: &SignRound7Message{
			Commitment: commitment,
		},
	}
}

// SignRound8Message is broadcast: decommitment of Ui/Ti.
type SignRound8Message struct {
	DeCommitment cmt.HashDeCommitment
}

// ValidateBasic checks that required fields of SignRound8Message are non-nil.
func (m *SignRound8Message) ValidateBasic() bool {
	return m != nil && len(m.DeCommitment) >= 2
}

// NewSignRound8Message constructs a *tss.Message with the given content.
func NewSignRound8Message(from *tss.PartyID, deCommitment cmt.HashDeCommitment) *tss.Message {
	return &tss.Message{
		From:        from,
		IsBroadcast: true,
		Content: &SignRound8Message{
			DeCommitment: deCommitment,
		},
	}
}

// SignRound9Message is broadcast: partial signature share.
type SignRound9Message struct {
	S *big.Int
}

// ValidateBasic checks that required fields of SignRound9Message are non-nil.
func (m *SignRound9Message) ValidateBasic() bool {
	return m != nil && m.S != nil && m.S.Sign() > 0
}

// NewSignRound9Message constructs a *tss.Message with the given content.
func NewSignRound9Message(from *tss.PartyID, si *big.Int) *tss.Message {
	return &tss.Message{
		From:        from,
		IsBroadcast: true,
		Content:     &SignRound9Message{S: si},
	}
}

// SignatureData holds the final ECDSA signature components.
type SignatureData struct {
	R                 []byte
	S                 []byte
	Signature         []byte // DER-encoded signature (optional)
	SignatureRecovery []byte
	M                 []byte // message hash
}
