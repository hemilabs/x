// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package resharing

import (
	"crypto/elliptic"
	"math/big"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/crypto"
	cmt "github.com/hemilabs/x/tss-lib/v2/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v2/crypto/vss"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

// These messages were generated from Protocol Buffers definitions into eddsa-resharing.pb.go

var (
	// Ensure that signing messages implement ValidateBasic
	_ = []tss.MessageContent{
		(*DGRound1Message)(nil),
		(*DGRound2Message)(nil),
		(*DGRound3Message1)(nil),
		(*DGRound3Message2)(nil),
		(*DGRound4Message)(nil),
	}
)

// ----- //

func NewDGRound1Message(
	to []*tss.PartyID,
	from *tss.PartyID,
	eddsaPub *crypto.ECPoint,
	vct cmt.HashCommitment,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:             from,
		To:               to,
		IsBroadcast:      true,
		IsToOldCommittee: false,
	}
	content := &DGRound1Message{
		EddsaPubX:   eddsaPub.X().Bytes(),
		EddsaPubY:   eddsaPub.Y().Bytes(),
		VCommitment: vct.Bytes(),
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

// [FORK] ValidateBasic: upstream checked nil receiver and non-empty fields but not sizes.
// Hardened with upper bounds on Edwards25519 coordinates (32 bytes) and commitment hash
// (32 bytes) to reject oversized payloads before they reach crypto deserialization.
func (m *DGRound1Message) ValidateBasic() bool {
	return m != nil &&
		common.NonEmptyBytes(m.EddsaPubX) &&
		len(m.EddsaPubX) <= 32 && // Edwards25519 coordinate max (32 bytes)
		common.NonEmptyBytes(m.EddsaPubY) &&
		len(m.EddsaPubY) <= 32 &&
		common.NonEmptyBytes(m.VCommitment) &&
		len(m.VCommitment) <= 32 // SHA-512/256 commitment hash
}

func (m *DGRound1Message) UnmarshalEDDSAPub(ec elliptic.Curve) (*crypto.ECPoint, error) {
	return crypto.NewECPoint(
		ec,
		new(big.Int).SetBytes(m.EddsaPubX),
		new(big.Int).SetBytes(m.EddsaPubY))
}

func (m *DGRound1Message) UnmarshalVCommitment() *big.Int {
	return new(big.Int).SetBytes(m.GetVCommitment())
}

// ----- //

func NewDGRound2Message(
	to []*tss.PartyID,
	from *tss.PartyID,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:             from,
		To:               to,
		IsBroadcast:      true,
		IsToOldCommittee: true,
	}
	content := &DGRound2Message{}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

// [FORK] ValidateBasic: upstream returned `true` unconditionally (no nil check).
// Hardened with nil receiver check.
func (m *DGRound2Message) ValidateBasic() bool {
	return m != nil
}

// ----- //

func NewDGRound3Message1(
	to *tss.PartyID,
	from *tss.PartyID,
	share *vss.Share,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:             from,
		To:               []*tss.PartyID{to},
		IsBroadcast:      false,
		IsToOldCommittee: false,
	}
	// [FORK] ReceiverId: upstream did not include the receiver's Key in the message.
	// Adding it allows the receiver to verify the share was intended for them,
	// preventing share redirection attacks where a relay swaps P2P envelopes (SC#2).
	content := &DGRound3Message1{
		Share:      share.Share.Bytes(),
		ReceiverId: to.GetKey(),
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

// [FORK] ValidateBasic: upstream only checked NonEmptyBytes(Share). Hardened with share
// length upper bound (32 bytes for ed25519 scalar) and mandatory ReceiverId presence.
func (m *DGRound3Message1) ValidateBasic() bool {
	return m != nil &&
		common.NonEmptyBytes(m.Share) &&
		len(m.Share) <= 32 && // ed25519 scalar max 32 bytes
		common.NonEmptyBytes(m.GetReceiverId())
}

// [FORK] UnmarshalReceiverId: new method to extract the receiver's Key for verification.
func (m *DGRound3Message1) UnmarshalReceiverId() []byte {
	return m.GetReceiverId()
}

// ----- //

func NewDGRound3Message2(
	to []*tss.PartyID,
	from *tss.PartyID,
	vdct cmt.HashDeCommitment,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:             from,
		To:               to,
		IsBroadcast:      true,
		IsToOldCommittee: false,
	}
	vDctBzs := common.BigIntsToBytes(vdct)
	content := &DGRound3Message2{
		VDecommitment: vDctBzs,
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

func (m *DGRound3Message2) ValidateBasic() bool {
	return m != nil &&
		common.NonEmptyMultiBytes(m.VDecommitment)
}

func (m *DGRound3Message2) UnmarshalVDeCommitment() cmt.HashDeCommitment {
	deComBzs := m.GetVDecommitment()
	return cmt.NewHashDeCommitmentFromBytes(deComBzs)
}

// ----- //

func NewDGRound4Message(
	to []*tss.PartyID,
	from *tss.PartyID,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:                    from,
		To:                      to,
		IsBroadcast:             true,
		IsToOldAndNewCommittees: true,
	}
	content := &DGRound4Message{}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

// [FORK] ValidateBasic: upstream returned `true` unconditionally (no nil check).
// Hardened with nil receiver check.
func (m *DGRound4Message) ValidateBasic() bool {
	return m != nil
}
