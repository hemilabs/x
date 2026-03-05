// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package keygen

import (
	"github.com/hemilabs/x/tss-lib/v2/crypto/facproof"
	"github.com/hemilabs/x/tss-lib/v2/crypto/modproof"
	"math/big"

	"github.com/hemilabs/x/tss-lib/v2/common"
	cmt "github.com/hemilabs/x/tss-lib/v2/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v2/crypto/dlnproof"
	"github.com/hemilabs/x/tss-lib/v2/crypto/paillier"
	"github.com/hemilabs/x/tss-lib/v2/crypto/vss"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

// These messages were generated from Protocol Buffers definitions into ecdsa-keygen.pb.go
// The following messages are registered on the Protocol Buffers "wire"

var (
	// Ensure that keygen messages implement ValidateBasic
	_ = []tss.MessageContent{
		(*KGRound1Message)(nil),
		(*KGRound2Message1)(nil),
		(*KGRound2Message2)(nil),
		(*KGRound3Message)(nil),
	}
)

// ----- //

func NewKGRound1Message(
	from *tss.PartyID,
	ct cmt.HashCommitment,
	paillierPK *paillier.PublicKey,
	nTildeI, h1I, h2I *big.Int,
	dlnProof1, dlnProof2 *dlnproof.Proof,
) (tss.ParsedMessage, error) {
	meta := tss.MessageRouting{
		From:        from,
		IsBroadcast: true,
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
	content := &KGRound1Message{
		Commitment: ct.Bytes(),
		PaillierN:  paillierPK.N.Bytes(),
		NTilde:     nTildeI.Bytes(),
		H1:         h1I.Bytes(),
		H2:         h2I.Bytes(),
		Dlnproof_1: dlnProof1Bz,
		Dlnproof_2: dlnProof2Bz,
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg), nil
}

// [FORK] ValidateBasic: upstream checks non-nil and non-empty on all fields, plus DLN proof
// size validation. We additionally add upper-bound length checks on each field to prevent
// memory exhaustion from adversarially oversized values, and make DLN proofs optional
// (absent in on-chain SNARK mode where per-participant SNARKs replace classical proofs).
func (m *KGRound1Message) ValidateBasic() bool {
	return m != nil &&
		common.NonEmptyBytes(m.GetCommitment()) &&
		len(m.GetCommitment()) <= 32 && // SHA-512/256 commitment hash
		common.NonEmptyBytes(m.GetPaillierN()) &&
		len(m.GetPaillierN()) <= 512 && // 4096-bit N max (512 bytes)
		common.NonEmptyBytes(m.GetNTilde()) &&
		len(m.GetNTilde()) <= 512 && // 4096-bit NTilde max
		common.NonEmptyBytes(m.GetH1()) &&
		len(m.GetH1()) <= 512 && // bounded by NTilde
		common.NonEmptyBytes(m.GetH2()) &&
		len(m.GetH2()) <= 512 && // bounded by NTilde
		// DLN proofs: absent (on-chain SNARK mode) OR correct size
		(len(m.GetDlnproof_1()) == 0 || common.NonEmptyMultiBytes(m.GetDlnproof_1(), 2+(dlnproof.Iterations*2))) &&
		(len(m.GetDlnproof_2()) == 0 || common.NonEmptyMultiBytes(m.GetDlnproof_2(), 2+(dlnproof.Iterations*2)))
}

func (m *KGRound1Message) UnmarshalCommitment() *big.Int {
	return new(big.Int).SetBytes(m.GetCommitment())
}

func (m *KGRound1Message) UnmarshalPaillierPK() *paillier.PublicKey {
	return &paillier.PublicKey{N: new(big.Int).SetBytes(m.GetPaillierN())}
}

func (m *KGRound1Message) UnmarshalNTilde() *big.Int {
	return new(big.Int).SetBytes(m.GetNTilde())
}

func (m *KGRound1Message) UnmarshalH1() *big.Int {
	return new(big.Int).SetBytes(m.GetH1())
}

func (m *KGRound1Message) UnmarshalH2() *big.Int {
	return new(big.Int).SetBytes(m.GetH2())
}

func (m *KGRound1Message) UnmarshalDLNProof1() (*dlnproof.Proof, error) {
	return dlnproof.UnmarshalDLNProof(m.GetDlnproof_1())
}

func (m *KGRound1Message) UnmarshalDLNProof2() (*dlnproof.Proof, error) {
	return dlnproof.UnmarshalDLNProof(m.GetDlnproof_2())
}

// ----- //

func NewKGRound2Message1(
	to, from *tss.PartyID,
	share *vss.Share,
	proof *facproof.ProofFac,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:        from,
		To:          []*tss.PartyID{to},
		IsBroadcast: false,
	}
	var proofBzs [][]byte
	if proof != nil {
		b := proof.Bytes()
		proofBzs = b[:]
	}
	// [FORK] ReceiverId: upstream did not include the receiver's Key in the message.
	// We bind the intended receiver's identity into the P2P message so that round 3
	// can verify the share was addressed to this party, preventing share misdirection.
	content := &KGRound2Message1{
		Share:      share.Share.Bytes(),
		FacProof:   proofBzs,
		ReceiverId: to.GetKey(),
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

// [FORK] ValidateBasic: upstream checks m != nil and non-empty share (FacProof check
// commented out for backward compatibility). We add share length bound, FacProof structure
// check (optional for SNARK mode), and ReceiverId non-empty check (fork addition for share binding).
func (m *KGRound2Message1) ValidateBasic() bool {
	return m != nil &&
		common.NonEmptyBytes(m.GetShare()) &&
		len(m.GetShare()) <= 32 && // secp256k1 scalar max 32 bytes
		// FacProof: absent (on-chain SNARK mode) OR correct size
		(len(m.GetFacProof()) == 0 || common.NonEmptyMultiBytes(m.GetFacProof(), facproof.ProofFacBytesParts)) &&
		common.NonEmptyBytes(m.GetReceiverId())
}

func (m *KGRound2Message1) UnmarshalShare() *big.Int {
	return new(big.Int).SetBytes(m.Share)
}

func (m *KGRound2Message1) UnmarshalFacProof() (*facproof.ProofFac, error) {
	return facproof.NewProofFromBytes(m.GetFacProof())
}

func (m *KGRound2Message1) UnmarshalReceiverId() []byte {
	return m.GetReceiverId()
}

// ----- //

func NewKGRound2Message2(
	from *tss.PartyID,
	deCommitment cmt.HashDeCommitment,
	proof *modproof.ProofMod,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:        from,
		IsBroadcast: true,
	}
	dcBzs := common.BigIntsToBytes(deCommitment)
	var proofBzs [][]byte
	if proof != nil {
		b := proof.Bytes()
		proofBzs = b[:]
	}
	content := &KGRound2Message2{
		DeCommitment: dcBzs,
		ModProof:     proofBzs,
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

// [FORK] ValidateBasic: upstream checks m != nil and non-empty decommitment (ModProof
// check commented out for backward compatibility). We add element count and per-element
// byte length bounds to prevent memory exhaustion, plus ModProof structure check (optional
// for SNARK mode).
func (m *KGRound2Message2) ValidateBasic() bool {
	if m == nil {
		return false
	}
	dc := m.GetDeCommitment()
	if len(dc) > 600 {
		return false
	}
	for _, bz := range dc {
		if len(bz) > 512 {
			return false
		}
	}
	return common.NonEmptyMultiBytes(dc) &&
		// ModProof: absent (on-chain SNARK mode) OR correct size
		(len(m.GetModProof()) == 0 || common.NonEmptyMultiBytes(m.GetModProof(), modproof.ProofModBytesParts))
}

func (m *KGRound2Message2) UnmarshalDeCommitment() []*big.Int {
	deComBzs := m.GetDeCommitment()
	return cmt.NewHashDeCommitmentFromBytes(deComBzs)
}

func (m *KGRound2Message2) UnmarshalModProof() (*modproof.ProofMod, error) {
	return modproof.NewProofFromBytes(m.GetModProof())
}

// ----- //

func NewKGRound3Message(
	from *tss.PartyID,
	proof paillier.Proof,
) tss.ParsedMessage {
	meta := tss.MessageRouting{
		From:        from,
		IsBroadcast: true,
	}
	pfBzs := make([][]byte, len(proof))
	for i := range pfBzs {
		if proof[i] == nil {
			continue
		}
		pfBzs[i] = proof[i].Bytes()
	}
	content := &KGRound3Message{
		PaillierProof: pfBzs,
	}
	msg := tss.NewMessageWrapper(meta, content)
	return tss.NewMessage(meta, content, msg)
}

// ValidateBasic checks Paillier proof has the correct number of iterations (ProofIters)
// and all proof bytes are non-empty. Same as upstream.
func (m *KGRound3Message) ValidateBasic() bool {
	return m != nil &&
		common.NonEmptyMultiBytes(m.GetPaillierProof(), paillier.ProofIters)
}

func (m *KGRound3Message) UnmarshalProofInts() paillier.Proof {
	var pf paillier.Proof
	proofBzs := m.GetPaillierProof()
	for i := range pf {
		pf[i] = new(big.Int).SetBytes(proofBzs[i])
	}
	return pf
}
