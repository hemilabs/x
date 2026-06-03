// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"math/big"

	cmt "github.com/hemilabs/x/tss-lib/v3/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v3/crypto/schnorr"
	"github.com/hemilabs/x/tss-lib/v3/crypto/vss"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// KGRound1Message is broadcast: VSS commitment hash.
type KGRound1Message struct {
	Commitment *big.Int
}

// ValidateBasic checks that required fields of KGRound1Message are non-nil.
func (m *KGRound1Message) ValidateBasic() bool {
	return m != nil && m.Commitment != nil && m.Commitment.Sign() > 0
}

// NewKGRound1Message constructs a *tss.Message with the given content.
func NewKGRound1Message(from *tss.PartyID, ct cmt.HashCommitment) *tss.Message {
	return &tss.Message{
		From:        from,
		IsBroadcast: true,
		Content: &KGRound1Message{
			Commitment: ct,
		},
	}
}

// KGRound2Message1 is P2P: VSS share + receiver binding.
type KGRound2Message1 struct {
	Share      *big.Int
	ReceiverID []byte
}

// ValidateBasic checks that required fields of KGRound2Message1 are non-nil.
func (m *KGRound2Message1) ValidateBasic() bool {
	return m != nil && m.Share != nil && m.Share.Sign() > 0 &&
		len(m.ReceiverID) > 0
}

// NewKGRound2Message1 constructs a *tss.Message with the given content.
func NewKGRound2Message1(to, from *tss.PartyID, share *vss.Share) *tss.Message {
	return &tss.Message{
		From: from,
		To:   []*tss.PartyID{to},
		Content: &KGRound2Message1{
			Share:      share.Share,
			ReceiverID: to.Key,
		},
	}
}

// KGRound2Message2 is broadcast: decommitment + Schnorr proof.
type KGRound2Message2 struct {
	DeCommitment cmt.HashDeCommitment
	ZKProof      *schnorr.ZKProof
}

// ValidateBasic checks that required fields of KGRound2Message2 are non-nil.
func (m *KGRound2Message2) ValidateBasic() bool {
	return m != nil && len(m.DeCommitment) >= 2 && m.ZKProof != nil
}

// NewKGRound2Message2 constructs a *tss.Message with the given content.
func NewKGRound2Message2(from *tss.PartyID, deCommitment cmt.HashDeCommitment, proof *schnorr.ZKProof) *tss.Message {
	return &tss.Message{
		From:        from,
		IsBroadcast: true,
		Content: &KGRound2Message2{
			DeCommitment: deCommitment,
			ZKProof:      proof,
		},
	}
}
