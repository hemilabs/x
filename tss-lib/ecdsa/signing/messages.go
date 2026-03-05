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
	"github.com/hemilabs/x/tss-lib/v2/crypto/mta"
	"github.com/hemilabs/x/tss-lib/v2/crypto/schnorr"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

// These messages were generated from Protocol Buffers definitions into ecdsa-signing.pb.go
// The following messages are registered on the Protocol Buffers "wire"
//
// [FORK] ValidateBasic hardening: upstream ValidateBasic() on all signing message types
// already checks non-nil and non-empty fields (with proof size validation where applicable).
// We add upper-bound length checks, ReceiverId non-empty checks on P2P messages, and
// per-element byte limits on decommitments. This catches oversized/malformed messages early.

var (
	// Ensure that signing messages implement ValidateBasic
	_ = []tss.MessageContent{
		(*SignRound1Message1)(nil),
		(*SignRound1Message2)(nil),
		(*SignRound2Message)(nil),
		(*SignRound3Message)(nil),
		(*SignRound4Message)(nil),
		(*SignRound5Message)(nil),
		(*SignRound6Message)(nil),
		(*SignRound7Message)(nil),
		(*SignRound8Message)(nil),
		(*SignRound9Message)(nil),
	}
)

// ----- //

func NewSignRound1Message1(
	to, from *tss.PartyID,
	c *big.Int,
	proof *mta.RangeProofAlice,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:        from,
		To:          []*tss.PartyID{to},
		IsBroadcast: false,
	}
	pfBz := proof.Bytes()
	// [FORK] ReceiverId field added to P2P messages. Upstream does not bind the intended
	// recipient into the message, allowing relay/reflection attacks where a P2P message
	// meant for party A is delivered to party B.
	content := &SignRound1Message1{
		C:               c.Bytes(),
		RangeProofAlice: pfBz[:],
		ReceiverId:      to.GetKey(),
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

func (m *SignRound1Message1) ValidateBasic() bool {
	return m != nil &&
		common.NonEmptyBytes(m.GetC()) &&
		len(m.GetC()) <= 1024 && // Paillier ciphertext is at most N^2 bytes (2*2048/8 = 512, with margin)
		common.NonEmptyMultiBytes(m.GetRangeProofAlice(), mta.RangeProofAliceBytesParts) &&
		common.NonEmptyBytes(m.GetReceiverId())
}

func (m *SignRound1Message1) UnmarshalC() *big.Int {
	return new(big.Int).SetBytes(m.GetC())
}

func (m *SignRound1Message1) UnmarshalRangeProofAlice() (*mta.RangeProofAlice, error) {
	return mta.RangeProofAliceFromBytes(m.GetRangeProofAlice())
}

// ----- //

func NewSignRound1Message2(
	from *tss.PartyID,
	commitment cmt.HashCommitment,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:        from,
		IsBroadcast: true,
	}
	content := &SignRound1Message2{
		Commitment: commitment.Bytes(),
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

func (m *SignRound1Message2) ValidateBasic() bool {
	return m != nil &&
		common.NonEmptyBytes(m.GetCommitment()) &&
		len(m.GetCommitment()) <= 32 // SHA-512/256 commitment hash
}

func (m *SignRound1Message2) UnmarshalCommitment() *big.Int {
	return new(big.Int).SetBytes(m.GetCommitment())
}

// ----- //

func NewSignRound2Message(
	to, from *tss.PartyID,
	c1Ji *big.Int,
	pi1Ji *mta.ProofBob,
	c2Ji *big.Int,
	pi2Ji *mta.ProofBobWC,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:        from,
		To:          []*tss.PartyID{to},
		IsBroadcast: false,
	}
	pfBob := pi1Ji.Bytes()
	pfBobWC := pi2Ji.Bytes()
	// [FORK] ReceiverId field added (same rationale as SignRound1Message1).
	content := &SignRound2Message{
		C1:         c1Ji.Bytes(),
		C2:         c2Ji.Bytes(),
		ProofBob:   pfBob[:],
		ProofBobWc: pfBobWC[:],
		ReceiverId: to.GetKey(),
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

func (m *SignRound2Message) ValidateBasic() bool {
	return m != nil &&
		common.NonEmptyBytes(m.C1) &&
		len(m.C1) <= 1024 && // Paillier ciphertext upper bound
		common.NonEmptyBytes(m.C2) &&
		len(m.C2) <= 1024 && // Paillier ciphertext upper bound
		common.NonEmptyMultiBytes(m.ProofBob, mta.ProofBobBytesParts) &&
		common.NonEmptyMultiBytes(m.ProofBobWc, mta.ProofBobWCBytesParts) &&
		common.NonEmptyBytes(m.GetReceiverId())
}

func (m *SignRound2Message) UnmarshalProofBob() (*mta.ProofBob, error) {
	return mta.ProofBobFromBytes(m.ProofBob)
}

func (m *SignRound2Message) UnmarshalProofBobWC(ec elliptic.Curve) (*mta.ProofBobWC, error) {
	return mta.ProofBobWCFromBytes(ec, m.ProofBobWc)
}

// ----- //

func NewSignRound3Message(
	from *tss.PartyID,
	theta *big.Int,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:        from,
		IsBroadcast: true,
	}
	content := &SignRound3Message{
		Theta: theta.Bytes(),
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

func (m *SignRound3Message) ValidateBasic() bool {
	return m != nil &&
		common.NonEmptyBytes(m.Theta) &&
		len(m.Theta) <= 32
}

// ----- //

func NewSignRound4Message(
	from *tss.PartyID,
	deCommitment cmt.HashDeCommitment,
	proof *schnorr.ZKProof,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:        from,
		IsBroadcast: true,
	}
	dcBzs := common.BigIntsToBytes(deCommitment)
	content := &SignRound4Message{
		DeCommitment: dcBzs,
		ProofAlphaX:  proof.Alpha.X().Bytes(),
		ProofAlphaY:  proof.Alpha.Y().Bytes(),
		ProofT:       proof.T.Bytes(),
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

func (m *SignRound4Message) ValidateBasic() bool {
	if m == nil {
		return false
	}
	// Bound decommitment element sizes: randomness (32) + x (33) + y (33).
	dc := m.DeCommitment
	for _, bz := range dc {
		if len(bz) > 33 {
			return false
		}
	}
	return common.NonEmptyMultiBytes(dc, 3) &&
		common.NonEmptyBytes(m.ProofAlphaX) &&
		len(m.ProofAlphaX) <= 33 && // EC point coordinate max
		common.NonEmptyBytes(m.ProofAlphaY) &&
		len(m.ProofAlphaY) <= 33 &&
		common.NonEmptyBytes(m.ProofT) &&
		len(m.ProofT) <= 32 // scalar max
}

func (m *SignRound4Message) UnmarshalDeCommitment() []*big.Int {
	deComBzs := m.GetDeCommitment()
	return cmt.NewHashDeCommitmentFromBytes(deComBzs)
}

func (m *SignRound4Message) UnmarshalZKProof(ec elliptic.Curve) (*schnorr.ZKProof, error) {
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

func NewSignRound5Message(
	from *tss.PartyID,
	commitment cmt.HashCommitment,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:        from,
		IsBroadcast: true,
	}
	content := &SignRound5Message{
		Commitment: commitment.Bytes(),
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

func (m *SignRound5Message) ValidateBasic() bool {
	return m != nil &&
		common.NonEmptyBytes(m.Commitment) &&
		len(m.Commitment) <= 32 // SHA-512/256 commitment hash
}

func (m *SignRound5Message) UnmarshalCommitment() *big.Int {
	return new(big.Int).SetBytes(m.GetCommitment())
}

// ----- //

func NewSignRound6Message(
	from *tss.PartyID,
	deCommitment cmt.HashDeCommitment,
	proof *schnorr.ZKProof,
	vProof *schnorr.ZKVProof,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:        from,
		IsBroadcast: true,
	}
	dcBzs := common.BigIntsToBytes(deCommitment)
	content := &SignRound6Message{
		DeCommitment: dcBzs,
		ProofAlphaX:  proof.Alpha.X().Bytes(),
		ProofAlphaY:  proof.Alpha.Y().Bytes(),
		ProofT:       proof.T.Bytes(),
		VProofAlphaX: vProof.Alpha.X().Bytes(),
		VProofAlphaY: vProof.Alpha.Y().Bytes(),
		VProofT:      vProof.T.Bytes(),
		VProofU:      vProof.U.Bytes(),
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

func (m *SignRound6Message) ValidateBasic() bool {
	if m == nil {
		return false
	}
	for _, d := range m.GetDeCommitment() {
		if len(d) > 33 { // EC coordinate or randomness max
			return false
		}
	}
	return common.NonEmptyMultiBytes(m.DeCommitment, 5) &&
		common.NonEmptyBytes(m.ProofAlphaX) &&
		len(m.ProofAlphaX) <= 33 && // EC point coordinate max
		common.NonEmptyBytes(m.ProofAlphaY) &&
		len(m.ProofAlphaY) <= 33 &&
		common.NonEmptyBytes(m.ProofT) &&
		len(m.ProofT) <= 32 && // scalar max
		common.NonEmptyBytes(m.VProofAlphaX) &&
		len(m.VProofAlphaX) <= 33 &&
		common.NonEmptyBytes(m.VProofAlphaY) &&
		len(m.VProofAlphaY) <= 33 &&
		common.NonEmptyBytes(m.VProofT) &&
		len(m.VProofT) <= 32 &&
		common.NonEmptyBytes(m.VProofU) &&
		len(m.VProofU) <= 32
}

func (m *SignRound6Message) UnmarshalDeCommitment() []*big.Int {
	deComBzs := m.GetDeCommitment()
	return cmt.NewHashDeCommitmentFromBytes(deComBzs)
}

func (m *SignRound6Message) UnmarshalZKProof(ec elliptic.Curve) (*schnorr.ZKProof, error) {
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

func (m *SignRound6Message) UnmarshalZKVProof(ec elliptic.Curve) (*schnorr.ZKVProof, error) {
	point, err := crypto.NewECPoint(
		ec,
		new(big.Int).SetBytes(m.GetVProofAlphaX()),
		new(big.Int).SetBytes(m.GetVProofAlphaY()))
	if err != nil {
		return nil, err
	}
	return &schnorr.ZKVProof{
		Alpha: point,
		T:     new(big.Int).SetBytes(m.GetVProofT()),
		U:     new(big.Int).SetBytes(m.GetVProofU()),
	}, nil
}

// ----- //

func NewSignRound7Message(
	from *tss.PartyID,
	commitment cmt.HashCommitment,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:        from,
		IsBroadcast: true,
	}
	content := &SignRound7Message{
		Commitment: commitment.Bytes(),
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

func (m *SignRound7Message) ValidateBasic() bool {
	return m != nil &&
		common.NonEmptyBytes(m.Commitment) &&
		len(m.Commitment) <= 32 // SHA-512/256 commitment hash
}

func (m *SignRound7Message) UnmarshalCommitment() *big.Int {
	return new(big.Int).SetBytes(m.GetCommitment())
}

// ----- //

func NewSignRound8Message(
	from *tss.PartyID,
	deCommitment cmt.HashDeCommitment,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:        from,
		IsBroadcast: true,
	}
	dcBzs := common.BigIntsToBytes(deCommitment)
	content := &SignRound8Message{
		DeCommitment: dcBzs,
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

func (m *SignRound8Message) ValidateBasic() bool {
	if m == nil || !common.NonEmptyMultiBytes(m.DeCommitment, 5) {
		return false
	}
	for _, d := range m.DeCommitment {
		if len(d) > 33 { // EC coordinate or randomness max
			return false
		}
	}
	return true
}

func (m *SignRound8Message) UnmarshalDeCommitment() []*big.Int {
	deComBzs := m.GetDeCommitment()
	return cmt.NewHashDeCommitmentFromBytes(deComBzs)
}

// ----- //

func NewSignRound9Message(
	from *tss.PartyID,
	si *big.Int,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:        from,
		IsBroadcast: true,
	}
	content := &SignRound9Message{
		S: si.Bytes(),
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

func (m *SignRound9Message) ValidateBasic() bool {
	return m != nil &&
		common.NonEmptyBytes(m.S) &&
		len(m.S) <= 32
}

func (m *SignRound9Message) UnmarshalS() *big.Int {
	return new(big.Int).SetBytes(m.S)
}
