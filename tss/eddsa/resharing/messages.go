// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package resharing

import (
	"math/big"

	"github.com/hemilabs/x/tss/v3/crypto"
	cmt "github.com/hemilabs/x/tss/v3/crypto/commitments"
	"github.com/hemilabs/x/tss/v3/crypto/vss"
	"github.com/hemilabs/x/tss/v3/tss"
)

// DGRound1Message is broadcast by old committee: EdDSA pub + VSS commitment.
type DGRound1Message struct {
	EDDSAPub    *crypto.ECPoint
	VCommitment *big.Int
}

// ValidateBasic checks that required fields of DGRound1Message are non-nil.
func (m *DGRound1Message) ValidateBasic() bool {
	return m != nil && m.EDDSAPub != nil &&
		m.VCommitment != nil && m.VCommitment.Sign() > 0
}

// NewDGRound1Message constructs a *tss.Message with the given content.
func NewDGRound1Message(to []*tss.PartyID, from *tss.PartyID, eddsaPub *crypto.ECPoint, vct cmt.HashCommitment) *tss.Message {
	return &tss.Message{
		From:        from,
		To:          to,
		IsBroadcast: true,
		Content: &DGRound1Message{
			EDDSAPub:    eddsaPub,
			VCommitment: vct,
		},
	}
}

// DGRound2Message is an ACK broadcast from new to old committee.
type DGRound2Message struct{}

// ValidateBasic checks that the receiver is non-nil.
func (m *DGRound2Message) ValidateBasic() bool { return m != nil }

// NewDGRound2Message constructs a *tss.Message with the given content.
func NewDGRound2Message(to []*tss.PartyID, from *tss.PartyID) *tss.Message {
	return &tss.Message{
		From:             from,
		To:               to,
		IsBroadcast:      true,
		IsToOldCommittee: true,
		Content:          &DGRound2Message{},
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
func NewDGRound3Message1(to *tss.PartyID, from *tss.PartyID, share *vss.Share) *tss.Message {
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

// ValidateBasic checks that the decommitment has enough elements.
func (m *DGRound3Message2) ValidateBasic() bool {
	return m != nil && len(m.VDeCommitment) >= 2
}

// NewDGRound3Message2 constructs a *tss.Message with the given content.
func NewDGRound3Message2(to []*tss.PartyID, from *tss.PartyID, vdct cmt.HashDeCommitment) *tss.Message {
	return &tss.Message{
		From:        from,
		To:          to,
		IsBroadcast: true,
		Content: &DGRound3Message2{
			VDeCommitment: vdct,
		},
	}
}

// DGRound4Message is an ACK broadcast to both committees.
type DGRound4Message struct{}

// ValidateBasic checks that the receiver is non-nil.
func (m *DGRound4Message) ValidateBasic() bool { return m != nil }

// NewDGRound4Message constructs a *tss.Message with the given content.
func NewDGRound4Message(to []*tss.PartyID, from *tss.PartyID) *tss.Message {
	return &tss.Message{
		From:                    from,
		To:                      to,
		IsBroadcast:             true,
		IsToOldAndNewCommittees: true,
		Content:                 &DGRound4Message{},
	}
}

// DGRound5Message is broadcast by a new party to the old committee
// after its own Round 5 passes.  It confirms the new sharing so that
// old parties know when to retire their shares.
type DGRound5Message struct {
	EDDSAPub     *crypto.ECPoint
	VCommitments []*big.Int // round 1 commitments, indexed by old party
	Digest       []byte     // confirmDigest of the new sharing
}

// ValidateBasic checks that required fields of DGRound5Message are non-nil.
func (m *DGRound5Message) ValidateBasic() bool {
	if m == nil || m.EDDSAPub == nil || len(m.VCommitments) == 0 ||
		len(m.Digest) == 0 {
		return false
	}
	for _, c := range m.VCommitments {
		if c == nil || c.Sign() <= 0 {
			return false
		}
	}
	return true
}

// NewDGRound5Message constructs a *tss.Message with the given content.
func NewDGRound5Message(to []*tss.PartyID, from *tss.PartyID, eddsaPub *crypto.ECPoint, vcts []*big.Int, digest []byte) *tss.Message {
	return &tss.Message{
		From:             from,
		To:               to,
		IsBroadcast:      true,
		IsToOldCommittee: true,
		Content: &DGRound5Message{
			EDDSAPub:     eddsaPub,
			VCommitments: vcts,
			Digest:       digest,
		},
	}
}
