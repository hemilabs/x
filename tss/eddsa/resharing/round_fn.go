// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package resharing

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"

	"github.com/hemilabs/x/tss/v3/common"
	"github.com/hemilabs/x/tss/v3/crypto"
	"github.com/hemilabs/x/tss/v3/crypto/commitments"
	"github.com/hemilabs/x/tss/v3/crypto/vss"
	"github.com/hemilabs/x/tss/v3/eddsa/keygen"
	"github.com/hemilabs/x/tss/v3/eddsa/signing"
	"github.com/hemilabs/x/tss/v3/tss"
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
func ReshareRound1(params *tss.ReSharingParameters, input *keygen.LocalPartySaveData) (*ReshareState, *ReshareRoundOutput, error) {
	oldPC := params.OldPartyCount()
	newPC := params.NewPartyCount()

	temp := &localTempData{
		localMessageStore: localMessageStore{
			dgRound1Messages:  make([]*tss.Message, oldPC),
			dgRound2Messages:  make([]*tss.Message, newPC),
			dgRound3Message1s: make([]*tss.Message, oldPC),
			dgRound3Message2s: make([]*tss.Message, oldPC),
			dgRound4Messages:  make([]*tss.Message, newPC),
			dgRound5Messages:  make([]*tss.Message, newPC),
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

	state.temp.VC = vCmt.C
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

	// Validate all old parties agree on the same EdDSA pub key.
	for j, msg := range r1Msgs {
		r1msg := msg.Content.(*DGRound1Message)
		if !r1msg.ValidateBasic() {
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

	ssid, err := getReshareSSID(params, &state.temp)
	if err != nil {
		return nil, fmt.Errorf("round 2 SSID: %w", err)
	}
	state.temp.ssid = ssid

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
func ReshareRound4(state *ReshareState, r1Msgs []*tss.Message, r3p2p []*tss.Message, r3bcast []*tss.Message) (*ReshareRoundOutput, error) {
	params := state.params

	if !params.IsNewCommittee() {
		return &ReshareRoundOutput{}, nil
	}

	Pi := params.PartyID()
	i := newIndex(params)
	oldPC := params.OldPartyCount()
	modQ := common.ModInt(params.EC().Params().N)

	// Verify decommitments, shares, accumulate newXi.
	newXi := big.NewInt(0)
	vjc := make([][]*crypto.ECPoint, oldPC)

	for j := 0; j < oldPC; j++ {
		r1msg := r1Msgs[j].Content.(*DGRound1Message)
		r3msg2 := r3bcast[j].Content.(*DGRound3Message2)

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
		r3msg1 := r3p2p[j].Content.(*DGRound3Message1)
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
// their new key material and send a DGRound5Message that confirms the
// new sharing to the old committee.
//
// ReshareRound5 does not touch the old share.  Old parties must keep
// it until ReshareConfirm returns true.  New parties must store the
// new share durably before they send the confirmation.
func ReshareRound5(state *ReshareState, r4AckMsgs []*tss.Message) (*ReshareRoundOutput, error) {
	out := &ReshareRoundOutput{Save: state.save}
	if !state.params.IsNewCommittee() {
		return out, nil
	}
	params := state.params
	state.save.BigXj = state.temp.newBigXjs
	state.save.ShareID = params.PartyID().KeyInt()
	state.save.Xi = state.temp.newXi
	state.save.Ks = state.temp.newKs

	vcts := make([]*big.Int, len(state.temp.dgRound1Messages))
	for j, msg := range state.temp.dgRound1Messages {
		vcts[j] = msg.Content.(*DGRound1Message).VCommitment
	}
	digest, err := confirmDigest(state.temp.ssid, state.save.EDDSAPub, vcts, state.save.BigXj)
	if err != nil {
		return nil, err
	}
	Pi := params.PartyID()
	r5msg := NewDGRound5Message(params.OldParties().IDs().Exclude(Pi), Pi,
		state.save.EDDSAPub, vcts, digest)
	state.temp.dgRound5Messages[newIndex(params)] = r5msg
	out.Messages = []*tss.Message{r5msg}

	return out, nil
}

// confirmQuorum returns the number of matching confirmations that
// old parties need before they retire their shares.  The digest is
// public, so up to t corrupt new parties can copy it.  Out of 2t+1
// confirmers at least t+1 are honest and hold working shares, and
// they can sign without the corrupt parties.  A committee with fewer
// than 2t+1 parties must confirm in full.
func confirmQuorum(t, n int) int {
	return min(2*t+1, n)
}

// confirmDigest binds a confirmation to the session, the group key,
// the fresh round 1 commitments and the new public shares.
func confirmDigest(ssid []byte, pub *crypto.ECPoint, vcts []*big.Int, bigXj []*crypto.ECPoint) ([]byte, error) {
	flat, err := crypto.FlattenECPoints(append([]*crypto.ECPoint{pub}, bigXj...))
	if err != nil {
		return nil, fmt.Errorf("confirm digest: %w", err)
	}
	list := make([]*big.Int, 0, 2+len(vcts)+len(flat))
	list = append(list,
		new(big.Int).SetBytes([]byte("eddsa-resharing-confirm")),
		new(big.Int).SetBytes(ssid))
	list = append(list, vcts...)
	list = append(list, flat...)
	return common.SHA512_256i(list...).Bytes(), nil
}

// ReshareConfirm reports whether an old committee party may retire
// its old share.  It returns true when confirmQuorum new parties
// confirm the same sharing of the same key for this ceremony.
//
// r5Msgs[j] is new party j's DGRound5Message.  Missing, foreign or
// stale confirmations are not counted.  ReshareConfirm does not
// modify the old share; the caller deletes it when this returns true.
func ReshareConfirm(state *ReshareState, r5Msgs []*tss.Message) (bool, error) {
	params := state.params
	if !params.IsOldCommittee() {
		return false, errors.New("reshare confirm: not in old committee")
	}
	newIDs := params.NewParties().IDs()
	if len(r5Msgs) != len(newIDs) {
		return false, fmt.Errorf("reshare confirm: got %d messages, want %d",
			len(r5Msgs), len(newIDs))
	}
	tss.MergeMsgs(state.temp.dgRound5Messages, r5Msgs)

	i := oldIndex(params)
	counts := make(map[string]int, len(newIDs))
	for j, msg := range state.temp.dgRound5Messages {
		if msg == nil || msg.From == nil ||
			msg.From.KeyInt().Cmp(newIDs[j].KeyInt()) != 0 {
			continue
		}
		c, ok := msg.Content.(*DGRound5Message)
		if !ok || !c.ValidateBasic() || !c.EDDSAPub.Equals(state.input.EDDSAPub) {
			continue
		}
		if len(c.VCommitments) != params.OldPartyCount() ||
			c.VCommitments[i].Cmp(state.temp.VC) != 0 {
			continue
		}
		counts[string(c.Digest)]++
	}
	quorum := confirmQuorum(params.NewThreshold(), params.NewPartyCount())
	for _, n := range counts {
		if n >= quorum {
			return true, nil
		}
	}
	return false, nil
}
