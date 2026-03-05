// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package signing

import (
	"crypto/elliptic"
	"math/big"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/crypto"
	cmt "github.com/hemilabs/x/tss-lib/v2/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v2/crypto/schnorr"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

// These messages were generated from Protocol Buffers definitions into eddsa-signing.pb.go
// The following messages are registered on the Protocol Buffers "wire"

var (
	// Ensure that signing messages implement ValidateBasic
	_ = []tss.MessageContent{
		(*SignRound1Message)(nil),
		(*SignRound2Message)(nil),
		(*SignRound3Message)(nil),
	}
)

// ----- //

func NewSignRound1Message(
	from *tss.PartyID,
	commitment cmt.HashCommitment,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:        from,
		IsBroadcast: true,
	}
	content := &SignRound1Message{
		Commitment: commitment.Bytes(),
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

// [FORK] ValidateBasic: upstream checked `m.Commitment != nil` (field-level, panics on nil
// receiver) and NonEmptyBytes. Changed to `m != nil` (receiver-level) and added upper-bound
// on commitment length to reject oversized payloads.
func (m *SignRound1Message) ValidateBasic() bool {
	return m != nil &&
		common.NonEmptyBytes(m.GetCommitment()) &&
		len(m.GetCommitment()) <= 32 // SHA-512/256 commitment hash
}

func (m *SignRound1Message) UnmarshalCommitment() *big.Int {
	return new(big.Int).SetBytes(m.GetCommitment())
}

// ----- //

func NewSignRound2Message(
	from *tss.PartyID,
	deCommitment cmt.HashDeCommitment,
	proof *schnorr.ZKProof,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:        from,
		IsBroadcast: true,
	}
	dcBzs := common.BigIntsToBytes(deCommitment)
	content := &SignRound2Message{
		DeCommitment: dcBzs,
		ProofAlphaX:  proof.Alpha.X().Bytes(),
		ProofAlphaY:  proof.Alpha.Y().Bytes(),
		ProofT:       proof.T.Bytes(),
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

// [FORK] ValidateBasic: upstream checked NonEmptyBytes on proof fields but not size bounds.
// Hardened with upper-bound checks on Schnorr proof fields (32 bytes for Edwards25519
// coordinates and scalars) to reject oversized payloads before they reach crypto deserialization.
func (m *SignRound2Message) ValidateBasic() bool {
	return m != nil &&
		common.NonEmptyMultiBytes(m.DeCommitment, 3) &&
		common.NonEmptyBytes(m.ProofAlphaX) &&
		len(m.ProofAlphaX) <= 32 && // Edwards25519 coordinate max (32 bytes)
		common.NonEmptyBytes(m.ProofAlphaY) &&
		len(m.ProofAlphaY) <= 32 &&
		common.NonEmptyBytes(m.ProofT) &&
		len(m.ProofT) <= 32 // scalar max
}

func (m *SignRound2Message) UnmarshalDeCommitment() []*big.Int {
	deComBzs := m.GetDeCommitment()
	return cmt.NewHashDeCommitmentFromBytes(deComBzs)
}

func (m *SignRound2Message) UnmarshalZKProof(ec elliptic.Curve) (*schnorr.ZKProof, error) {
	point, err := crypto.NewECPoint(
		ec,
		new(big.Int).SetBytes(m.GetProofAlphaX()),
		new(big.Int).SetBytes(m.GetProofAlphaY()))
	if err != nil {
		return nil, err
	}
	return &schnorr.ZKProof{
		Alpha: point,
		T:     new(big.Int).SetBytes(m.GetProofT()),
	}, nil
}

// ----- //

func NewSignRound3Message(
	from *tss.PartyID,
	si *big.Int,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:        from,
		IsBroadcast: true,
	}
	content := &SignRound3Message{
		S: si.Bytes(),
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

// [FORK] ValidateBasic: upstream did not bound S length. Hardened with 32-byte upper bound
// matching the ed25519 scalar size to prevent oversized payloads from reaching ScMulAdd.
func (m *SignRound3Message) ValidateBasic() bool {
	return m != nil &&
		common.NonEmptyBytes(m.S) &&
		len(m.S) <= 32
}

func (m *SignRound3Message) UnmarshalS() *big.Int {
	return new(big.Int).SetBytes(m.S)
}
