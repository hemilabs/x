// Copyright © 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package resharing

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"

	"github.com/hemilabs/x/tss-lib/v3/common"
	"github.com/hemilabs/x/tss-lib/v3/crypto"
	"github.com/hemilabs/x/tss-lib/v3/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v3/crypto/vss"
	"github.com/hemilabs/x/tss-lib/v3/eddsa/keygen"
	"github.com/hemilabs/x/tss-lib/v3/eddsa/signing"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// oldIndex returns this party's index in the old committee, or -1.
func oldIndex(params *tss.ReSharingParameters) int {
	key := params.PartyID().KeyInt()
	for i, pid := range params.OldParties().IDs() {
		if pid.KeyInt().Cmp(key) == 0 {
			return i
		}
	}
	return -1
}

// newIndex returns this party's index in the new committee, or -1.
func newIndex(params *tss.ReSharingParameters) int {
	key := params.PartyID().KeyInt()
	for i, pid := range params.NewParties().IDs() {
		if pid.KeyInt().Cmp(key) == 0 {
			return i
		}
	}
	return -1
}

// getReshareSSID computes the session ID for domain separation.
func getReshareSSID(params *tss.ReSharingParameters, temp *localTempData) ([]byte, error) {
	ssidList := []*big.Int{
		new(big.Int).SetBytes([]byte("eddsa-resharing")),
		params.EC().Params().P, params.EC().Params().N,
		params.EC().Params().B,
		params.EC().Params().Gx, params.EC().Params().Gy,
	}
	ssidList = append(ssidList, params.OldParties().IDs().Keys()...)
	ssidList = append(ssidList, params.NewParties().IDs().Keys()...)
	ssidList = append(ssidList, big.NewInt(int64(params.PartyCount())))
	ssidList = append(ssidList, big.NewInt(int64(params.Threshold())))
	ssidList = append(ssidList, big.NewInt(int64(params.NewThreshold())))
	ssidList = append(ssidList, temp.ssidNonce)
	if cid := params.CeremonyID(); len(cid) > 0 {
		ssidList = append(ssidList, new(big.Int).SetBytes(cid))
	}
	return common.SHA512_256i(ssidList...).Bytes(), nil
}

// ReshareRound1 is executed by the OLD committee.  It computes
// Lagrange-interpolated wi, creates VSS shares for the new committee,
// and broadcasts a commitment.
//
// New committee parties call this too but get a no-op (nil messages).
func ReshareRound1(
	params *tss.ReSharingParameters,
	input *keygen.LocalPartySaveData,
) (*ReshareState, *ReshareRoundOutput, error) {
	oldPC := params.OldPartyCount()
	newPC := params.NewPartyCount()

	temp := &localTempData{
		localMessageStore: localMessageStore{
			dgRound1Messages:  make([]*tss.Message, oldPC),
			dgRound2Messages:  make([]*tss.Message, newPC),
			dgRound3Message1s: make([]*tss.Message, oldPC),
			dgRound3Message2s: make([]*tss.Message, oldPC),
			dgRound4Messages:  make([]*tss.Message, newPC),
		},
	}
	save := keygen.NewLocalPartySaveData(newPC)

	temp.ssidNonce = new(big.Int).SetUint64(uint64(params.SSIDNonce()))

	state := &ReshareState{params: params, input: input, save: &save, temp: *temp}

	if !params.IsOldCommittee() {
		return state, &ReshareRoundOutput{}, nil
	}

	ssid, err := getReshareSSID(params, &state.temp)
	if err != nil {
		return nil, nil, fmt.Errorf("round 1 SSID: %w", err)
	}
	state.temp.ssid = ssid

	Pi := params.PartyID()
	i := oldIndex(params)

	// Lagrange interpolation.
	xi, ks := input.Xi, input.Ks
	if params.Threshold()+1 > len(ks) {
		return nil, nil, fmt.Errorf("t+1=%d not satisfied by key count %d", params.Threshold()+1, len(ks))
	}
	wi := signing.PrepareForSigning(params.EC(), i, len(params.OldParties().IDs()), xi, ks)

	// VSS create for new committee.
	newKs := params.NewParties().IDs().Keys()
	vi, shares, _, err := vss.Create(params.EC(), params.NewThreshold(), wi, newKs, params.Rand())
	if err != nil {
		return nil, nil, fmt.Errorf("round 1 vss create: %w", err)
	}

	// Commitment.
	flatVis, err := crypto.FlattenECPoints(vi)
	if err != nil {
		return nil, nil, fmt.Errorf("round 1 flatten: %w", err)
	}
	vCmt := commitments.NewHashCommitment(params.Rand(), flatVis...)

	state.temp.VD = vCmt.D
	state.temp.NewShares = shares

	r1msg := NewDGRound1Message(
		params.NewParties().IDs().Exclude(Pi), Pi,
		input.EDDSAPub, vCmt.C)
	state.temp.dgRound1Messages[i] = r1msg

	return state, &ReshareRoundOutput{Messages: []*tss.Message{r1msg}}, nil
}

// ReshareRound2 is executed by the NEW committee.  It validates that
// all old parties agree on the EdDSA public key, then sends an ACK
// to the old committee.
//
// Old committee parties call this too but get a no-op.
func ReshareRound2(state *ReshareState, r1Msgs []*tss.Message) (*ReshareRoundOutput, error) {
	params := state.params

	if !params.IsNewCommittee() {
		return &ReshareRoundOutput{}, nil
	}

	oldPC := len(params.OldParties().IDs())
	if len(r1Msgs) != oldPC {
		return nil, fmt.Errorf("expected %d round 1 messages, got %d", oldPC, len(r1Msgs))
	}

	// Validate all old parties agree on the same EdDSA pub key.
	for j, msg := range r1Msgs {
		if msg == nil {
			return nil, tss.NewError(errors.New("missing round 1 message"), TaskName, 2, params.PartyID(), params.OldParties().IDs()[j])
		}
		r1msg, ok := msg.Content.(*DGRound1Message)
		if !ok || !r1msg.ValidateBasic() {
			return nil, tss.NewError(errors.New("invalid round 1 message"), TaskName, 2,
				params.PartyID(), msg.From)
		}
		candidate := r1msg.EDDSAPub
		if state.save.EDDSAPub != nil && !candidate.Equals(state.save.EDDSAPub) {
			return nil, tss.NewError(errors.New("eddsa pub key mismatch"), TaskName, 2,
				params.PartyID(), msg.From)
		}
		state.save.EDDSAPub = candidate
		state.temp.dgRound1Messages[j] = msg
	}

	Pi := params.PartyID()
	i := newIndex(params)

	r2msg := NewDGRound2Message(params.OldParties().IDs(), Pi)
	state.temp.dgRound2Messages[i] = r2msg

	return &ReshareRoundOutput{Messages: []*tss.Message{r2msg}}, nil
}

// ReshareRound3 is executed by the OLD committee.  It sends P2P
// VSS shares and broadcasts the decommitment to the new committee.
//
// New committee parties call this too but get a no-op.
func ReshareRound3(state *ReshareState, r2AckMsgs []*tss.Message) (*ReshareRoundOutput, error) {
	params := state.params

	if !params.IsOldCommittee() {
		return &ReshareRoundOutput{}, nil
	}

	newPC := len(params.NewParties().IDs())
	if len(r2AckMsgs) != newPC {
		return nil, fmt.Errorf("expected %d round 2 ack messages, got %d", newPC, len(r2AckMsgs))
	}

	Pi := params.PartyID()
	i := oldIndex(params)

	// P2P shares to new committee.
	msgs := make([]*tss.Message, 0, params.NewPartyCount()+1)
	for j, Pj := range params.NewParties().IDs() {
		share := state.temp.NewShares[j]
		r3msg1 := NewDGRound3Message1(Pj, Pi, share)
		state.temp.dgRound3Message1s[i] = r3msg1
		msgs = append(msgs, r3msg1)
	}

	// Broadcast decommitment.
	r3msg2 := NewDGRound3Message2(
		params.NewParties().IDs().Exclude(Pi), Pi,
		state.temp.VD)
	state.temp.dgRound3Message2s[i] = r3msg2
	msgs = append(msgs, r3msg2)

	return &ReshareRoundOutput{Messages: msgs}, nil
}

// ReshareRound4 is executed by the NEW committee.  It verifies all
// decommitments and VSS shares, computes the new key share and
// BigXj, and sends an ACK to both committees.
//
// Old committee parties call this too but get a no-op.
func ReshareRound4(
	state *ReshareState,
	r1Msgs []*tss.Message,
	r3p2p []*tss.Message,
	r3bcast []*tss.Message,
) (*ReshareRoundOutput, error) {
	params := state.params

	if !params.IsNewCommittee() {
		return &ReshareRoundOutput{}, nil
	}

	oldPC := params.OldPartyCount()
	if len(r1Msgs) != oldPC {
		return nil, fmt.Errorf("expected %d round 1 messages, got %d", oldPC, len(r1Msgs))
	}
	if len(r3p2p) != oldPC {
		return nil, fmt.Errorf("expected %d round 3 P2P messages, got %d", oldPC, len(r3p2p))
	}
	if len(r3bcast) != oldPC {
		return nil, fmt.Errorf("expected %d round 3 broadcast messages, got %d", oldPC, len(r3bcast))
	}

	Pi := params.PartyID()
	i := newIndex(params)
	modQ := common.ModInt(params.EC().Params().N)

	// Verify decommitments, shares, accumulate newXi.
	newXi := big.NewInt(0)
	vjc := make([][]*crypto.ECPoint, oldPC)

	for j := 0; j < oldPC; j++ {
		if r1Msgs[j] == nil {
			return nil, tss.NewError(errors.New("missing round 1 message"), TaskName, 4, Pi, params.OldParties().IDs()[j])
		}
		r1msg, ok1 := r1Msgs[j].Content.(*DGRound1Message)
		if !ok1 {
			return nil, tss.NewError(errors.New("invalid round 1 message type"), TaskName, 4, Pi, params.OldParties().IDs()[j])
		}
		if r3bcast[j] == nil {
			return nil, tss.NewError(errors.New("missing round 3 broadcast message"), TaskName, 4, Pi, params.OldParties().IDs()[j])
		}
		r3msg2, ok2 := r3bcast[j].Content.(*DGRound3Message2)
		if !ok2 || !r3msg2.ValidateBasic() {
			return nil, tss.NewError(errors.New("invalid round 3 broadcast message"), TaskName, 4, Pi, params.OldParties().IDs()[j])
		}

		vCmtDeCmt := commitments.HashCommitDecommit{C: r1msg.VCommitment, D: r3msg2.VDeCommitment}
		ok, flatVs := vCmtDeCmt.DeCommit()
		if !ok || len(flatVs) != (params.NewThreshold()+1)*2 {
			return nil, tss.NewError(errors.New("de-commitment of v_j0..v_jt failed"),
				TaskName, 4, Pi, params.OldParties().IDs()[j])
		}
		vj, err := crypto.UnFlattenECPoints(params.EC(), flatVs)
		if err != nil {
			return nil, tss.NewError(fmt.Errorf("unflatten: %w", err),
				TaskName, 4, Pi, params.OldParties().IDs()[j])
		}
		// Cofactor clearing for Edwards curve.
		for k := range vj {
			vj[k] = vj[k].EightInvEight()
		}
		vjc[j] = vj

		// Verify receiver binding + share.
		if r3p2p[j] == nil {
			return nil, tss.NewError(errors.New("missing round 3 P2P message"), TaskName, 4, Pi, params.OldParties().IDs()[j])
		}
		r3msg1, ok3 := r3p2p[j].Content.(*DGRound3Message1)
		if !ok3 || !r3msg1.ValidateBasic() {
			return nil, tss.NewError(errors.New("invalid round 3 P2P message"), TaskName, 4, Pi, params.OldParties().IDs()[j])
		}
		if !bytes.Equal(r3msg1.ReceiverID, Pi.Key) {
			return nil, tss.NewError(errors.New("receiverId mismatch"),
				TaskName, 4, Pi, params.OldParties().IDs()[j])
		}
		sharej := &vss.Share{
			Threshold: params.NewThreshold(),
			ID:        Pi.KeyInt(),
			Share:     r3msg1.Share,
		}
		if !sharej.Verify(params.EC(), params.NewThreshold(), vj) {
			return nil, tss.NewError(errors.New("share verify failed"),
				TaskName, 4, Pi, params.OldParties().IDs()[j])
		}
		newXi = new(big.Int).Add(newXi, sharej.Share)
	}

	newXi = new(big.Int).Mod(newXi, params.EC().Params().N)
	if newXi.Sign() == 0 {
		return nil, tss.NewError(errors.New("newXi is zero"), TaskName, 4, Pi)
	}

	// Compute Vc = sum of vjc columns.
	Vc := make([]*crypto.ECPoint, params.NewThreshold()+1)
	for c := 0; c <= params.NewThreshold(); c++ {
		Vc[c] = vjc[0][c]
		for j := 1; j < oldPC; j++ {
			var err error
			Vc[c], err = Vc[c].Add(vjc[j][c])
			if err != nil {
				return nil, tss.NewError(fmt.Errorf("Vc[c].Add: %w", err), TaskName, 4, Pi)
			}
		}
	}

	// Verify V_0 == EdDSA pub key.
	if !Vc[0].Equals(state.save.EDDSAPub) {
		return nil, tss.NewError(errors.New("V_0 != EdDSA pub key"), TaskName, 4, Pi)
	}

	// Compute new BigXj for each new party.
	newKs := make([]*big.Int, 0, params.NewPartyCount())
	newBigXjs := make([]*crypto.ECPoint, params.NewPartyCount())
	for j := 0; j < params.NewPartyCount(); j++ {
		Pj := params.NewParties().IDs()[j]
		kj := Pj.KeyInt()
		newKs = append(newKs, kj)
		BigXj := Vc[0]
		z := new(big.Int).SetInt64(1)
		for c := 1; c <= params.NewThreshold(); c++ {
			z = modQ.Mul(z, kj)
			var err error
			BigXj, err = BigXj.Add(Vc[c].ScalarMult(z))
			if err != nil {
				return nil, tss.NewError(fmt.Errorf("BigXj computation failed: %w", err),
					TaskName, 4, Pi, Pj)
			}
		}
		if BigXj.IsIdentity() {
			return nil, tss.NewError(errors.New("BigXj is the identity point"),
				TaskName, 4, Pi, Pj)
		}
		newBigXjs[j] = BigXj
	}

	state.temp.newXi = newXi
	state.temp.newKs = newKs
	state.temp.newBigXjs = newBigXjs

	// ACK to both committees.
	r4msg := NewDGRound4Message(params.OldAndNewParties(), Pi)
	state.temp.dgRound4Messages[i] = r4msg

	return &ReshareRoundOutput{Messages: []*tss.Message{r4msg}}, nil
}

// ReshareRound5 finalizes the resharing.  New committee parties save
// their new key material.  Old committee parties zero their old Xi.
func ReshareRound5(
	state *ReshareState,
	r4AckMsgs []*tss.Message,
) (*ReshareRoundOutput, error) {
	if state.params.IsNewCommittee() {
		newPC := len(state.params.NewParties().IDs())
		if len(r4AckMsgs) != newPC {
			return nil, fmt.Errorf("expected %d round 4 ack messages, got %d", newPC, len(r4AckMsgs))
		}
		state.save.BigXj = state.temp.newBigXjs
		state.save.ShareID = state.params.PartyID().KeyInt()
		state.save.Xi = state.temp.newXi
		state.save.Ks = state.temp.newKs
	}
	// Zero old Xi — including dual-committee parties.
	if state.params.IsOldCommittee() {
		state.input.Xi.SetInt64(0)
	}

	return &ReshareRoundOutput{Save: state.save}, nil
}
