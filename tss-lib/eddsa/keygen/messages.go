// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package keygen

import (
	"crypto/elliptic"
	"math/big"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/crypto"
	cmt "github.com/hemilabs/x/tss-lib/v2/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v2/crypto/schnorr"
	"github.com/hemilabs/x/tss-lib/v2/crypto/vss"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

// These messages were generated from Protocol Buffers definitions into eddsa-keygen.pb.go
// The following messages are registered on the Protocol Buffers "wire"

var (
	// Ensure that keygen messages implement ValidateBasic
	_ = []tss.MessageContent{
		(*KGRound1Message)(nil),
		(*KGRound2Message1)(nil),
		(*KGRound2Message2)(nil),
	}
)

// ----- //

func NewKGRound1Message(from *tss.PartyID, ct cmt.HashCommitment) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:        from,
		IsBroadcast: true,
	}
	content := &KGRound1Message{
		Commitment: ct.Bytes(),
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

// [FORK] ValidateBasic: upstream checked nil receiver and NonEmptyBytes. Hardened with
// upper-bound on commitment length to reject oversized payloads before they reach crypto code.
func (m *KGRound1Message) ValidateBasic() bool {
	return m != nil &&
		common.NonEmptyBytes(m.GetCommitment()) &&
		len(m.GetCommitment()) <= 32 // SHA-512/256 commitment hash
}

func (m *KGRound1Message) UnmarshalCommitment() *big.Int {
	return new(big.Int).SetBytes(m.GetCommitment())
}

// ----- //

func NewKGRound2Message1(
	to, from *tss.PartyID,
	share *vss.Share,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:        from,
		To:          []*tss.PartyID{to},
		IsBroadcast: false,
	}
	// [FORK] ReceiverId: upstream did not include the receiver's Key in the message.
	// Adding it allows the receiver to verify the share was intended for them,
	// preventing share redirection attacks where a relay swaps P2P envelopes (SC#2).
	content := &KGRound2Message1{
		Share:      share.Share.Bytes(),
		ReceiverId: to.GetKey(),
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

// [FORK] ValidateBasic: upstream only checked NonEmptyBytes(Share). Hardened with share length
// upper bound (32 bytes for ed25519 scalars) and mandatory ReceiverId presence.
func (m *KGRound2Message1) ValidateBasic() bool {
	return m != nil &&
		common.NonEmptyBytes(m.GetShare()) &&
		len(m.GetShare()) <= 32 &&
		common.NonEmptyBytes(m.GetReceiverId())
}

// [FORK] UnmarshalReceiverId: new method to extract the receiver's Key for verification.
func (m *KGRound2Message1) UnmarshalReceiverId() []byte {
	return m.GetReceiverId()
}

func (m *KGRound2Message1) UnmarshalShare() *big.Int {
	return new(big.Int).SetBytes(m.Share)
}

// ----- //

func NewKGRound2Message2(
	from *tss.PartyID,
	deCommitment cmt.HashDeCommitment,
	proof *schnorr.ZKProof,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:        from,
		IsBroadcast: true,
	}
	dcBzs := common.BigIntsToBytes(deCommitment)
	content := &KGRound2Message2{
		DeCommitment: dcBzs,
		ProofAlphaX:  proof.Alpha.X().Bytes(),
		ProofAlphaY:  proof.Alpha.Y().Bytes(),
		ProofT:       proof.T.Bytes(),
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

// [FORK] ValidateBasic: upstream only checked NonEmptyMultiBytes(decommitment). Hardened with
// upper-bound checks on Schnorr proof fields (32 bytes for Edwards25519 coordinates and scalars)
// to reject oversized payloads before they reach crypto deserialization.
func (m *KGRound2Message2) ValidateBasic() bool {
	return m != nil &&
		common.NonEmptyMultiBytes(m.GetDeCommitment()) &&
		common.NonEmptyBytes(m.GetProofAlphaX()) &&
		len(m.GetProofAlphaX()) <= 32 && // Edwards25519 coordinate max (32 bytes)
		common.NonEmptyBytes(m.GetProofAlphaY()) &&
		len(m.GetProofAlphaY()) <= 32 &&
		common.NonEmptyBytes(m.GetProofT()) &&
		len(m.GetProofT()) <= 32 // scalar max
}

func (m *KGRound2Message2) UnmarshalDeCommitment() []*big.Int {
	deComBzs := m.GetDeCommitment()
	return cmt.NewHashDeCommitmentFromBytes(deComBzs)
}

func (m *KGRound2Message2) UnmarshalZKProof(ec elliptic.Curve) (*schnorr.ZKProof, error) {
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
