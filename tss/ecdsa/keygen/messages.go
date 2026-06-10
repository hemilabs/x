// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"math/big"

	cmt "github.com/hemilabs/x/tss/v3/crypto/commitments"
	"github.com/hemilabs/x/tss/v3/crypto/dlnproof"
	"github.com/hemilabs/x/tss/v3/crypto/facproof"
	"github.com/hemilabs/x/tss/v3/crypto/modproof"
	"github.com/hemilabs/x/tss/v3/crypto/paillier"
	"github.com/hemilabs/x/tss/v3/crypto/vss"
	"github.com/hemilabs/x/tss/v3/tss"
)

// KGRound1Message is broadcast by each party in keygen round 1.
// Contains the commitment hash, Paillier public key, Pedersen
// parameters (NTilde, H1, H2) and optional DLN proofs.
type KGRound1Message struct {
	Commitment *big.Int
	PaillierPK *paillier.PublicKey
	NTilde     *big.Int
	H1         *big.Int
	H2         *big.Int
	DLNProof1  *dlnproof.Proof // nil in on-chain SNARK mode
	DLNProof2  *dlnproof.Proof // nil in on-chain SNARK mode
}

// ValidateBasic checks that all required fields are non-nil and
// within expected bounds.
func (m *KGRound1Message) ValidateBasic() bool {
	if m == nil {
		return false
	}
	if m.Commitment == nil || m.Commitment.Sign() == 0 {
		return false
	}
	if m.PaillierPK == nil || m.PaillierPK.N == nil || m.PaillierPK.N.Sign() == 0 {
		return false
	}
	if m.NTilde == nil || m.NTilde.Sign() == 0 {
		return false
	}
	if m.H1 == nil || m.H1.Sign() == 0 {
		return false
	}
	if m.H2 == nil || m.H2.Sign() == 0 {
		return false
	}
	// DLN proofs optional (SNARK mode)
	return true
}

// NewKGRound1Message constructs a round 1 broadcast message.
func NewKGRound1Message(from *tss.PartyID, ct cmt.HashCommitment, paillierPK *paillier.PublicKey, nTildeI, h1I, h2I *big.Int, dlnProof1, dlnProof2 *dlnproof.Proof) *tss.Message {
	return &tss.Message{
		From:        from,
		IsBroadcast: true,
		Content: &KGRound1Message{
			Commitment: ct,
			PaillierPK: paillierPK,
			NTilde:     nTildeI,
			H1:         h1I,
			H2:         h2I,
			DLNProof1:  dlnProof1,
			DLNProof2:  dlnProof2,
		},
	}
}

// KGRound2Message1 is a P2P message sent to each other party in
// keygen round 2.  Contains the VSS share and optional FacProof.
type KGRound2Message1 struct {
	Share      *big.Int
	FacProof   *facproof.ProofFac // nil in on-chain SNARK mode
	ReceiverID []byte
}

// ValidateBasic checks that the share is non-nil and the receiver
// ID is present.
func (m *KGRound2Message1) ValidateBasic() bool {
	return m != nil &&
		m.Share != nil && m.Share.Sign() > 0 &&
		len(m.ReceiverID) > 0
}

// NewKGRound2Message1 constructs a round 2 P2P message.
func NewKGRound2Message1(to, from *tss.PartyID, share *vss.Share, proof *facproof.ProofFac) *tss.Message {
	return &tss.Message{
		From: from,
		To:   []*tss.PartyID{to},
		Content: &KGRound2Message1{
			Share:      share.Share,
			FacProof:   proof,
			ReceiverID: to.Key,
		},
	}
}

// KGRound2Message2 is broadcast by each party in keygen round 2.
// Contains the decommitment and optional ModProof.
type KGRound2Message2 struct {
	DeCommitment cmt.HashDeCommitment
	ModProof     *modproof.ProofMod // nil in on-chain SNARK mode
}

// ValidateBasic checks that the decommitment has at least 2
// elements (blinding factor + payload).
func (m *KGRound2Message2) ValidateBasic() bool {
	return m != nil && len(m.DeCommitment) >= 2
}

// NewKGRound2Message2 constructs a round 2 broadcast message.
func NewKGRound2Message2(from *tss.PartyID, deCommitment cmt.HashDeCommitment, proof *modproof.ProofMod) *tss.Message {
	return &tss.Message{
		From:        from,
		IsBroadcast: true,
		Content: &KGRound2Message2{
			DeCommitment: deCommitment,
			ModProof:     proof,
		},
	}
}

// KGRound3Message is broadcast by each party in keygen round 3.
// Contains the Paillier proof (array of big.Int).
type KGRound3Message struct {
	PaillierProof paillier.Proof
}

// ValidateBasic checks that the proof has the correct number of
// iterations and all elements are non-nil.
func (m *KGRound3Message) ValidateBasic() bool {
	if m == nil {
		return false
	}
	for _, pi := range m.PaillierProof {
		if pi == nil {
			return false
		}
	}
	return true
}

// NewKGRound3Message constructs a round 3 broadcast message.
func NewKGRound3Message(from *tss.PartyID, proof paillier.Proof) *tss.Message {
	return &tss.Message{
		From:        from,
		IsBroadcast: true,
		Content: &KGRound3Message{
			PaillierProof: proof,
		},
	}
}
