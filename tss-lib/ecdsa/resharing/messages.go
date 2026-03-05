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
	"github.com/hemilabs/x/tss-lib/v2/crypto/dlnproof"
	"github.com/hemilabs/x/tss-lib/v2/crypto/facproof"
	"github.com/hemilabs/x/tss-lib/v2/crypto/modproof"
	"github.com/hemilabs/x/tss-lib/v2/crypto/paillier"
	"github.com/hemilabs/x/tss-lib/v2/crypto/vss"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

// These messages were generated from Protocol Buffers definitions into ecdsa-resharing.pb.go

var (
	// Ensure that signing messages implement ValidateBasic
	_ = []tss.MessageContent{
		(*DGRound1Message)(nil),
		(*DGRound2Message1)(nil),
		(*DGRound2Message2)(nil),
		(*DGRound3Message1)(nil),
		(*DGRound3Message2)(nil),
		(*DGRound4Message1)(nil),
		(*DGRound4Message2)(nil),
	}
)

// ----- //

func NewDGRound1Message(
	to []*tss.PartyID,
	from *tss.PartyID,
	ecdsaPub *crypto.ECPoint,
	vct cmt.HashCommitment,
	ssid []byte,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:             from,
		To:               to,
		IsBroadcast:      true,
		IsToOldCommittee: false,
	}
	content := &DGRound1Message{
		EcdsaPubX:   ecdsaPub.X().Bytes(),
		EcdsaPubY:   ecdsaPub.Y().Bytes(),
		VCommitment: vct.Bytes(),
		Ssid:        ssid,
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

// [FORK] ValidateBasic: upstream checks non-nil and non-empty on EcdsaPubX, EcdsaPubY,
// and VCommitment but does not validate the SSID field. We add upper-bound length checks
// on all fields and require the SSID to be non-empty with a bounded length.
func (m *DGRound1Message) ValidateBasic() bool {
	return m != nil &&
		common.NonEmptyBytes(m.EcdsaPubX) &&
		len(m.EcdsaPubX) <= 33 && // raw secp256k1 field element max 32B, with 1B safety margin
		common.NonEmptyBytes(m.EcdsaPubY) &&
		len(m.EcdsaPubY) <= 33 &&
		common.NonEmptyBytes(m.VCommitment) &&
		len(m.VCommitment) <= 32 && // SHA-512/256 commitment hash
		common.NonEmptyBytes(m.GetSsid()) &&
		len(m.GetSsid()) <= 256 // SSID is a hash chain, bounded
}

func (m *DGRound1Message) UnmarshalECDSAPub(ec elliptic.Curve) (*crypto.ECPoint, error) {
	return crypto.NewECPoint(
		ec,
		new(big.Int).SetBytes(m.EcdsaPubX),
		new(big.Int).SetBytes(m.EcdsaPubY))
}

func (m *DGRound1Message) UnmarshalVCommitment() *big.Int {
	return new(big.Int).SetBytes(m.GetVCommitment())
}

func (m *DGRound1Message) UnmarshalSSID() []byte {
	return m.GetSsid()
}

// ----- //

func NewDGRound2Message1(
	to []*tss.PartyID,
	from *tss.PartyID,
	paillierPK *paillier.PublicKey,
	modProof *modproof.ProofMod,
	NTildei, H1i, H2i *big.Int,
	dlnProof1, dlnProof2 *dlnproof.Proof,
) (tss.ParsedMessage, error) {
	meta := tss.MessageRouting{
		From:             from,
		To:               to,
		IsBroadcast:      true,
		IsToOldCommittee: false,
	}
	var modPfBzs [][]byte
	if modProof != nil {
		bz := modProof.Bytes()
		modPfBzs = bz[:]
	}
	var dlnProof1Bz, dlnProof2Bz [][]byte
	if dlnProof1 != nil {
		var err error
		dlnProof1Bz, err = dlnProof1.Serialize()
		if err != nil {
			return nil, err
		}
	}
	if dlnProof2 != nil {
		var err error
		dlnProof2Bz, err = dlnProof2.Serialize()
		if err != nil {
			return nil, err
		}
	}
	content := &DGRound2Message1{
		PaillierN:  paillierPK.N.Bytes(),
		ModProof:   modPfBzs,
		NTilde:     NTildei.Bytes(),
		H1:         H1i.Bytes(),
		H2:         H2i.Bytes(),
		Dlnproof_1: dlnProof1Bz,
		Dlnproof_2: dlnProof2Bz,
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg), nil
}

// [FORK] ValidateBasic: upstream checks non-nil and non-empty on PaillierN, NTilde, H1,
// H2, plus DLN proof size validation. We add upper-bound length checks on all fields and
// make ModProof and DLN proofs optional (absent in on-chain SNARK mode).
func (m *DGRound2Message1) ValidateBasic() bool {
	return m != nil &&
		// ModProof: absent (on-chain SNARK mode) OR correct size
		(len(m.GetModProof()) == 0 || common.NonEmptyMultiBytes(m.GetModProof(), modproof.ProofModBytesParts)) &&
		common.NonEmptyBytes(m.PaillierN) &&
		len(m.PaillierN) <= 512 && // 4096-bit N max (512 bytes)
		common.NonEmptyBytes(m.NTilde) &&
		len(m.NTilde) <= 512 && // 4096-bit NTilde max
		common.NonEmptyBytes(m.H1) &&
		len(m.H1) <= 512 && // bounded by NTilde
		common.NonEmptyBytes(m.H2) &&
		len(m.H2) <= 512 && // bounded by NTilde
		// DLN proofs: absent (on-chain SNARK mode) OR correct size
		(len(m.GetDlnproof_1()) == 0 || common.NonEmptyMultiBytes(m.GetDlnproof_1(), 2+(dlnproof.Iterations*2))) &&
		(len(m.GetDlnproof_2()) == 0 || common.NonEmptyMultiBytes(m.GetDlnproof_2(), 2+(dlnproof.Iterations*2)))
}

func (m *DGRound2Message1) UnmarshalPaillierPK() *paillier.PublicKey {
	return &paillier.PublicKey{
		N: new(big.Int).SetBytes(m.PaillierN),
	}
}

func (m *DGRound2Message1) UnmarshalNTilde() *big.Int {
	return new(big.Int).SetBytes(m.GetNTilde())
}

func (m *DGRound2Message1) UnmarshalH1() *big.Int {
	return new(big.Int).SetBytes(m.GetH1())
}

func (m *DGRound2Message1) UnmarshalH2() *big.Int {
	return new(big.Int).SetBytes(m.GetH2())
}

func (m *DGRound2Message1) UnmarshalModProof() (*modproof.ProofMod, error) {
	return modproof.NewProofFromBytes(m.GetModProof())
}

func (m *DGRound2Message1) UnmarshalDLNProof1() (*dlnproof.Proof, error) {
	return dlnproof.UnmarshalDLNProof(m.GetDlnproof_1())
}

func (m *DGRound2Message1) UnmarshalDLNProof2() (*dlnproof.Proof, error) {
	return dlnproof.UnmarshalDLNProof(m.GetDlnproof_2())
}

// ----- //

func NewDGRound2Message2(
	to []*tss.PartyID,
	from *tss.PartyID,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:             from,
		To:               to,
		IsBroadcast:      true,
		IsToOldCommittee: true,
	}
	content := &DGRound2Message2{}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

// [FORK] ValidateBasic: upstream returned `true` unconditionally (no nil check).
// Hardened with nil receiver check.
func (m *DGRound2Message2) ValidateBasic() bool {
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
	// We bind the intended receiver's identity into the P2P message so that round 4
	// can verify the share was addressed to this party, preventing share misdirection.
	content := &DGRound3Message1{
		Share:      share.Share.Bytes(),
		ReceiverId: to.GetKey(),
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

// [FORK] ValidateBasic: upstream checks m != nil and non-empty share. We add share
// length bound and ReceiverId non-empty check (ReceiverId field is a fork addition).
func (m *DGRound3Message1) ValidateBasic() bool {
	return m != nil &&
		common.NonEmptyBytes(m.Share) &&
		len(m.Share) <= 32 && // secp256k1 scalar max 32 bytes
		common.NonEmptyBytes(m.GetReceiverId())
}

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

// [FORK] ValidateBasic: upstream checks m != nil and non-empty decommitment. We add
// element count and per-element byte length bounds to prevent memory exhaustion from
// malicious oversized decommitments.
func (m *DGRound3Message2) ValidateBasic() bool {
	if m == nil {
		return false
	}
	vd := m.GetVDecommitment()
	if len(vd) > 600 {
		return false
	}
	for _, bz := range vd {
		if len(bz) > 33 {
			return false
		}
	}
	return common.NonEmptyMultiBytes(vd)
}

func (m *DGRound3Message2) UnmarshalVDeCommitment() cmt.HashDeCommitment {
	deComBzs := m.GetVDecommitment()
	return cmt.NewHashDeCommitmentFromBytes(deComBzs)
}

// ----- //

func NewDGRound4Message2(
	to []*tss.PartyID,
	from *tss.PartyID,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:                    from,
		To:                      to,
		IsBroadcast:             true,
		IsToOldAndNewCommittees: true,
	}
	content := &DGRound4Message2{}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

// [FORK] ValidateBasic: upstream returned `true` unconditionally (no nil check).
// Hardened with nil receiver check.
func (m *DGRound4Message2) ValidateBasic() bool {
	return m != nil
}

func NewDGRound4Message1(
	to *tss.PartyID,
	from *tss.PartyID,
	proof *facproof.ProofFac,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:             from,
		To:               []*tss.PartyID{to},
		IsBroadcast:      false,
		IsToOldCommittee: false,
	}
	var pfBzs [][]byte
	if proof != nil {
		bz := proof.Bytes()
		pfBzs = bz[:]
	}
	// [FORK] ReceiverId: upstream did not bind the receiver's Key. We include it so round 5
	// can verify the fac proof was intended for this party, preventing proof redirection.
	content := &DGRound4Message1{
		FacProof:   pfBzs,
		ReceiverId: to.GetKey(),
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

// [FORK] ValidateBasic: upstream checks m != nil (FacProof check commented out for backward
// compatibility). We add optional FacProof structure check (for SNARK mode) and ReceiverId
// non-empty check (ReceiverId field is a fork addition for share binding).
func (m *DGRound4Message1) ValidateBasic() bool {
	return m != nil &&
		// FacProof: absent (on-chain SNARK mode) OR correct size
		(len(m.GetFacProof()) == 0 || common.NonEmptyMultiBytes(m.GetFacProof(), facproof.ProofFacBytesParts)) &&
		common.NonEmptyBytes(m.GetReceiverId())
}

func (m *DGRound4Message1) UnmarshalFacProof() (*facproof.ProofFac, error) {
	return facproof.NewProofFromBytes(m.GetFacProof())
}

func (m *DGRound4Message1) UnmarshalReceiverId() []byte {
	return m.GetReceiverId()
}
