// Copyright © 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"

	"github.com/hemilabs/x/tss-lib/v3/common"
	"github.com/hemilabs/x/tss-lib/v3/crypto"
	cmts "github.com/hemilabs/x/tss-lib/v3/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v3/crypto/schnorr"
	"github.com/hemilabs/x/tss-lib/v3/crypto/vss"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// getSSID computes the session ID for domain separation.
func getSSID(params *tss.Parameters, temp *localTempData, roundNumber int) ([]byte, error) {
	ssidList := []*big.Int{
		temp.ssidNonce,
		big.NewInt(int64(roundNumber)),
		big.NewInt(int64(params.PartyCount())),
		big.NewInt(int64(params.Threshold())),
	}
	for _, id := range params.Parties().IDs() {
		ssidList = append(ssidList, id.KeyInt())
	}
	if id := params.CeremonyID(); len(id) > 0 {
		ssidList = append(ssidList, new(big.Int).SetBytes(id))
	}
	ssid := common.SHA512_256i(ssidList...)
	return ssid.Bytes(), nil
}

// Round1 generates the VSS polynomial and broadcasts a commitment.
// Returns the keygen state to pass to subsequent rounds.
func Round1(params *tss.Parameters, preParams ...interface{}) (*KeygenState, *RoundOutput, error) {
	n := params.PartyCount()
	Pi := params.PartyID()
	i := Pi.Index

	temp := &localTempData{
		localMessageStore: localMessageStore{
			kgRound1Messages:  make([]*tss.Message, n),
			kgRound2Message1s: make([]*tss.Message, n),
			kgRound2Message2s: make([]*tss.Message, n),
		},
	}
	save := NewLocalPartySaveData(n)

	temp.ssidNonce = new(big.Int).SetUint64(uint64(params.SSIDNonce()))
	ssid, err := getSSID(params, temp, 1)
	if err != nil {
		return nil, nil, fmt.Errorf("round 1 SSID: %w", err)
	}
	temp.ssid = ssid

	// Generate partial key share ui.
	ui := common.GetRandomPositiveInt(params.PartialKeyRand(), params.EC().Params().N)
	temp.ui = ui

	// VSS create.
	ids := params.Parties().IDs().Keys()
	vs, shares, _, err := vss.Create(params.EC(), params.Threshold(), ui, ids, params.Rand())
	if err != nil {
		return nil, nil, fmt.Errorf("round 1 vss create: %w", err)
	}
	save.Ks = ids
	save.ShareID = ids[i]

	temp.vs = vs
	temp.shares = shares

	// Commitment.
	pGFlat, err := crypto.FlattenECPoints(vs)
	if err != nil {
		return nil, nil, fmt.Errorf("round 1 flatten: %w", err)
	}
	cmt := cmts.NewHashCommitment(params.Rand(), pGFlat...)
	temp.deCommitPolyG = cmt.D

	r1msg := NewKGRound1Message(Pi, cmt.C)
	temp.kgRound1Messages[i] = r1msg

	state := &KeygenState{params: params, save: save, temp: *temp}
	return state, &RoundOutput{
		Messages: []*tss.Message{r1msg},
		Poly:     vs,
	}, nil
}

// Round2 sends P2P shares and broadcasts decommitment + Schnorr proof.
func Round2(state *KeygenState, r1Msgs []*tss.Message) (*RoundOutput, error) {
	params := state.params
	temp := &state.temp
	n := params.PartyCount()
	i := params.PartyID().Index

	// Store r1 commitments.
	for j := 0; j < n; j++ {
		r1msg := r1Msgs[j].Content.(*KGRound1Message)
		if !r1msg.ValidateBasic() {
			return nil, tss.NewError(errors.New("invalid round 1 message"), TaskName, 2, params.PartyID(),
				r1Msgs[j].From)
		}
		temp.kgRound1Messages[j] = r1Msgs[j]
	}

	// P2P share messages.
	msgs := make([]*tss.Message, 0, n)
	for j, Pj := range params.Parties().IDs() {
		r2msg1 := NewKGRound2Message1(Pj, params.PartyID(), temp.shares[j])
		if j == i {
			temp.kgRound2Message1s[j] = r2msg1
			continue
		}
		temp.kgRound2Message1s[j] = nil // will come from network
		msgs = append(msgs, r2msg1)
	}

	// Schnorr proof.
	ContextI := common.AppendBigIntToBytesSlice(temp.ssid, new(big.Int).SetUint64(uint64(i)))
	pii, err := schnorr.NewZKProof(ContextI, temp.ui, temp.vs[0], params.Rand())
	if err != nil {
		return nil, fmt.Errorf("round 2 schnorr proof: %w", err)
	}
	// Clear ui from memory.
	temp.ui = new(big.Int)

	// Broadcast decommitment + proof.
	r2msg2 := NewKGRound2Message2(params.PartyID(), temp.deCommitPolyG, pii)
	temp.kgRound2Message2s[i] = r2msg2
	msgs = append(msgs, r2msg2)

	return &RoundOutput{Messages: msgs}, nil
}

// Round3 verifies all decommitments, shares, and Schnorr proofs,
// then computes the distributed EdDSA public key and saves the result.
func Round3(state *KeygenState, r2p2p, r2bcast []*tss.Message) (*RoundOutput, error) {
	params := state.params
	temp := &state.temp
	save := &state.save
	n := params.PartyCount()
	PIdx := params.PartyID().Index

	// Compute own Xi from shares.
	xi := new(big.Int).Set(temp.shares[PIdx].Share)
	for j := 0; j < n; j++ {
		if j == PIdx {
			continue
		}
		r2msg1 := r2p2p[j].Content.(*KGRound2Message1)
		xi = new(big.Int).Add(xi, r2msg1.Share)
	}
	save.Xi = new(big.Int).Mod(xi, params.EC().Params().N)
	if save.Xi.Sign() == 0 {
		return nil, tss.NewError(errors.New("xi is zero"), TaskName, 3, params.PartyID())
	}

	// Verify each party's decommitment, Schnorr proof, and VSS share.
	Vc := make(vss.Vs, params.Threshold()+1)
	for c := range Vc {
		Vc[c] = temp.vs[c]
	}

	for j := 0; j < n; j++ {
		if j == PIdx {
			continue
		}
		Pj := params.Parties().IDs()[j]
		ContextJ := common.AppendBigIntToBytesSlice(temp.ssid, big.NewInt(int64(j)))

		// Verify commitment.
		r1msg := r1MsgContent(state, j)
		r2msg2 := r2bcast[j].Content.(*KGRound2Message2)
		cmtDeCmt := cmts.HashCommitDecommit{C: r1msg.Commitment, D: r2msg2.DeCommitment}
		ok, flatPolyGs := cmtDeCmt.DeCommit()
		if !ok || flatPolyGs == nil {
			return nil, tss.NewError(errors.New("de-commitment verify failed"), TaskName, 3, params.PartyID(), Pj)
		}
		PjVs, err := crypto.UnFlattenECPoints(params.EC(), flatPolyGs)
		if err != nil {
			return nil, tss.NewError(fmt.Errorf("unflatten: %w", err), TaskName, 3, params.PartyID(), Pj)
		}
		// Cofactor clearing for Edwards curve — rejects torsion points.
		for k := range PjVs {
			PjVs[k] = PjVs[k].EightInvEight()
		}

		// Schnorr proof verify.
		if r2msg2.ZKProof == nil {
			return nil, tss.NewError(errors.New("missing schnorr proof"), TaskName, 3, params.PartyID(), Pj)
		}
		if !r2msg2.ZKProof.Verify(ContextJ, PjVs[0]) {
			return nil, tss.NewError(errors.New("schnorr proof verify failed"), TaskName, 3, params.PartyID(), Pj)
		}

		// Receiver binding check.
		r2msg1 := r2p2p[j].Content.(*KGRound2Message1)
		if !bytes.Equal(r2msg1.ReceiverID, params.PartyID().Key) {
			return nil, tss.NewError(errors.New("receiverId mismatch"), TaskName, 3, params.PartyID(), Pj)
		}

		// VSS share verify.
		PjShare := vss.Share{
			Threshold: params.Threshold(),
			ID:        params.PartyID().KeyInt(),
			Share:     r2msg1.Share,
		}
		if !PjShare.Verify(params.EC(), params.Threshold(), PjVs) {
			return nil, tss.NewError(errors.New("vss share verify failed"), TaskName, 3, params.PartyID(), Pj)
		}

		// Accumulate Vc.
		for c := 0; c <= params.Threshold(); c++ {
			var err error
			Vc[c], err = Vc[c].Add(PjVs[c])
			if err != nil {
				return nil, tss.NewError(fmt.Errorf("vc point addition failed"), TaskName, 3, params.PartyID(), Pj)
			}
		}
	}

	// Compute BigXj for each party.
	modQ := common.ModInt(params.EC().Params().N)
	for j := 0; j < n; j++ {
		Pj := params.Parties().IDs()[j]
		kj := Pj.KeyInt()
		BigXj := Vc[0]
		z := new(big.Int).SetInt64(1)
		for c := 1; c <= params.Threshold(); c++ {
			z = modQ.Mul(z, kj)
			var err error
			BigXj, err = BigXj.Add(Vc[c].ScalarMult(z))
			if err != nil {
				return nil, tss.NewError(errors.New("BigXj computation failed"), TaskName, 3, params.PartyID(), Pj)
			}
		}
		if BigXj.IsIdentity() {
			return nil, tss.NewError(errors.New("BigXj is the identity point"), TaskName, 3, params.PartyID(), Pj)
		}
		save.BigXj[j] = BigXj
	}

	// Compute EdDSA public key.
	eddsaPubKey, err := crypto.NewECPoint(params.EC(), Vc[0].X(), Vc[0].Y())
	if err != nil {
		return nil, fmt.Errorf("public key not on curve: %w", err)
	}
	if eddsaPubKey.IsIdentity() {
		return nil, tss.NewError(errors.New("public key is the identity point"), TaskName, 3, params.PartyID())
	}
	save.EDDSAPub = eddsaPubKey

	return &RoundOutput{Save: save}, nil
}

// r1MsgContent extracts the KGRound1Message from the keygen state's
// stored round 1 messages. For the party's own message, it reads from
// the state; for others, from the passed-in r1Msgs via Round1.
// This helper keeps Round3 clean.
func r1MsgContent(state *KeygenState, j int) *KGRound1Message {
	return state.temp.kgRound1Messages[j].Content.(*KGRound1Message)
}
