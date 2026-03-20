// Copyright © 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.
package resharing

import (
	"math/big"

	"github.com/hemilabs/x/tss-lib/v3/crypto"
	cmt "github.com/hemilabs/x/tss-lib/v3/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v3/crypto/dlnproof"
	"github.com/hemilabs/x/tss-lib/v3/crypto/facproof"
	"github.com/hemilabs/x/tss-lib/v3/crypto/modproof"
	"github.com/hemilabs/x/tss-lib/v3/crypto/paillier"
	"github.com/hemilabs/x/tss-lib/v3/crypto/vss"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// DGRound1Message is broadcast by old committee: ECDSA pub + VSS commitment + SSID.
type DGRound1Message struct {
	ECDSAPub    *crypto.ECPoint
	VCommitment *big.Int
	SSID        []byte
}

// ValidateBasic checks that required fields of DGRound1Message are non-nil.
func (m *DGRound1Message) ValidateBasic() bool {
	return m != nil && m.ECDSAPub != nil &&
		m.VCommitment != nil && m.VCommitment.Sign() > 0 &&
		len(m.SSID) > 0
}

// NewDGRound1Message constructs a *tss.Message with the given content.
func NewDGRound1Message(
	to []*tss.PartyID,
	from *tss.PartyID,
	ecdsaPub *crypto.ECPoint,
	vct cmt.HashCommitment,
	ssid []byte,
) *tss.Message {
	return &tss.Message{
		From:        from,
		To:          to,
		IsBroadcast: true,
		Content: &DGRound1Message{
			ECDSAPub:    ecdsaPub,
			VCommitment: vct,
			SSID:        ssid,
		},
	}
}

// DGRound2Message1 is broadcast by new committee: Paillier key + Pedersen params + proofs.
type DGRound2Message1 struct {
	PaillierPK *paillier.PublicKey
	NTilde     *big.Int
	H1         *big.Int
	H2         *big.Int
	ModProof   *modproof.ProofMod // nil in SNARK mode
	DLNProof1  *dlnproof.Proof    // nil in SNARK mode
	DLNProof2  *dlnproof.Proof    // nil in SNARK mode
}

// ValidateBasic checks that required fields of DGRound2Message1 are non-nil.
func (m *DGRound2Message1) ValidateBasic() bool {
	return m != nil &&
		m.PaillierPK != nil && m.PaillierPK.N != nil && m.PaillierPK.N.Sign() > 0 &&
		m.NTilde != nil && m.NTilde.Sign() > 0 &&
		m.H1 != nil && m.H1.Sign() > 0 &&
		m.H2 != nil && m.H2.Sign() > 0
}

// NewDGRound2Message1 constructs a *tss.Message with the given content.
func NewDGRound2Message1(
	to []*tss.PartyID,
	from *tss.PartyID,
	paillierPK *paillier.PublicKey,
	modProof *modproof.ProofMod,
	NTildei, H1i, H2i *big.Int,
	dlnProof1, dlnProof2 *dlnproof.Proof,
) *tss.Message {
	return &tss.Message{
		From:        from,
		To:          to,
		IsBroadcast: true,
		Content: &DGRound2Message1{
			PaillierPK: paillierPK,
			NTilde:     NTildei,
			H1:         H1i,
			H2:         H2i,
			ModProof:   modProof,
			DLNProof1:  dlnProof1,
			DLNProof2:  dlnProof2,
		},
	}
}

// DGRound2Message2 is an ACK broadcast from new to old committee.
type DGRound2Message2 struct{}

// ValidateBasic checks that required fields of DGRound2Message2 are non-nil.
func (m *DGRound2Message2) ValidateBasic() bool { return m != nil }

// NewDGRound2Message2 constructs a *tss.Message with the given content.
func NewDGRound2Message2(
	to []*tss.PartyID,
	from *tss.PartyID,
) *tss.Message {
	return &tss.Message{
		From:             from,
		To:               to,
		IsBroadcast:      true,
		IsToOldCommittee: true,
		Content:          &DGRound2Message2{},
	}
}

// DGRound3Message1 is P2P from old to new: VSS share.
type DGRound3Message1 struct {
	Share      *big.Int
	ReceiverID []byte
}

// ValidateBasic checks that required fields of DGRound3Message1 are non-nil.
func (m *DGRound3Message1) ValidateBasic() bool {
	return m != nil && m.Share != nil && m.Share.Sign() > 0 &&
		len(m.ReceiverID) > 0
}

// NewDGRound3Message1 constructs a *tss.Message with the given content.
func NewDGRound3Message1(
	to *tss.PartyID,
	from *tss.PartyID,
	share *vss.Share,
) *tss.Message {
	return &tss.Message{
		From: from,
		To:   []*tss.PartyID{to},
		Content: &DGRound3Message1{
			Share:      share.Share,
			ReceiverID: to.Key,
		},
	}
}

// DGRound3Message2 is broadcast by old committee: VSS decommitment.
type DGRound3Message2 struct {
	VDeCommitment cmt.HashDeCommitment
}

// ValidateBasic checks that required fields of DGRound3Message2 are non-nil.
func (m *DGRound3Message2) ValidateBasic() bool {
	return m != nil && len(m.VDeCommitment) >= 2
}

// NewDGRound3Message2 constructs a *tss.Message with the given content.
func NewDGRound3Message2(
	to []*tss.PartyID,
	from *tss.PartyID,
	vdct cmt.HashDeCommitment,
) *tss.Message {
	return &tss.Message{
		From:        from,
		To:          to,
		IsBroadcast: true,
		Content: &DGRound3Message2{
			VDeCommitment: vdct,
		},
	}
}

// DGRound4Message1 is P2P from new to new: FacProof.
type DGRound4Message1 struct {
	FacProof   *facproof.ProofFac // nil in SNARK mode
	ReceiverID []byte
}

// ValidateBasic checks that required fields of DGRound4Message1 are non-nil.
func (m *DGRound4Message1) ValidateBasic() bool {
	return m != nil && len(m.ReceiverID) > 0
}

// NewDGRound4Message1 constructs a *tss.Message with the given content.
func NewDGRound4Message1(
	to *tss.PartyID,
	from *tss.PartyID,
	proof *facproof.ProofFac,
) *tss.Message {
	return &tss.Message{
		From: from,
		To:   []*tss.PartyID{to},
		Content: &DGRound4Message1{
			FacProof:   proof,
			ReceiverID: to.Key,
		},
	}
}

// DGRound4Message2 is an ACK broadcast to both committees.
type DGRound4Message2 struct{}

// ValidateBasic checks that required fields of DGRound4Message2 are non-nil.
func (m *DGRound4Message2) ValidateBasic() bool { return m != nil }

// NewDGRound4Message2 constructs a *tss.Message with the given content.
func NewDGRound4Message2(
	to []*tss.PartyID,
	from *tss.PartyID,
) *tss.Message {
	return &tss.Message{
		From:                    from,
		To:                      to,
		IsBroadcast:             true,
		IsToOldAndNewCommittees: true,
		Content:                 &DGRound4Message2{},
	}
}
