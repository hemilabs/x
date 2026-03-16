// Copyright © 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.
package resharing

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sync"

	errors2 "github.com/pkg/errors"

	"github.com/hemilabs/x/tss-lib/v3/common"
	"github.com/hemilabs/x/tss-lib/v3/crypto"
	"github.com/hemilabs/x/tss-lib/v3/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v3/crypto/dlnproof"
	"github.com/hemilabs/x/tss-lib/v3/crypto/facproof"
	"github.com/hemilabs/x/tss-lib/v3/crypto/modproof"
	"github.com/hemilabs/x/tss-lib/v3/crypto/vss"
	"github.com/hemilabs/x/tss-lib/v3/ecdsa/keygen"
	"github.com/hemilabs/x/tss-lib/v3/ecdsa/signing"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

var (
	reshareOne        = big.NewInt(1)
	resharePaiBitsLen = 2048
)

func getReshareSSID(params *tss.ReSharingParameters, input *keygen.LocalPartySaveData, temp *localTempData, roundNumber int) ([]byte, error) {
	ssidList := []*big.Int{
		new(big.Int).SetBytes([]byte("ecdsa-resharing")),
		params.EC().Params().P, params.EC().Params().N,
		params.EC().Params().B, params.EC().Params().Gx,
		params.EC().Params().Gy,
	}
	ssidList = append(ssidList, params.OldParties().IDs().Keys()...)
	ssidList = append(ssidList, params.NewParties().IDs().Keys()...)
	BigXjList, err := crypto.FlattenECPoints(input.BigXj)
	if err != nil {
		return nil, fmt.Errorf("flatten ec points: %w", err)
	}
	ssidList = append(ssidList, BigXjList...)
	ssidList = append(ssidList, input.NTildej...)
	ssidList = append(ssidList, input.H1j...)
	ssidList = append(ssidList, input.H2j...)
	ssidList = append(ssidList, big.NewInt(int64(params.PartyCount())))
	ssidList = append(ssidList, big.NewInt(int64(params.Threshold())))
	ssidList = append(ssidList, big.NewInt(int64(params.NewPartyCount())))
	ssidList = append(ssidList, big.NewInt(int64(params.NewThreshold())))
	ssidList = append(ssidList, big.NewInt(int64(roundNumber)))
	ssidList = append(ssidList, temp.ssidNonce)
	if cid := params.CeremonyID(); len(cid) > 0 {
		ssidList = append(ssidList, new(big.Int).SetBytes(cid))
	}
	return common.SHA512_256i(ssidList...).Bytes(), nil
}

// ReshareRound1 creates a new ReshareState and produces the round 1
// broadcast message.  Only old committee members produce output.
//
// key is the existing key share.  preParams is for the new committee
// (may be zero-value if this party is old-only).
func ReshareRound1(
	params *tss.ReSharingParameters,
	key keygen.LocalPartySaveData,
	preParams keygen.LocalPreParams,
) (*ReshareState, *ReshareRoundOutput, error) {
	oldPC := params.OldPartyCount()
	newPC := params.NewPartyCount()
	input := key
	if params.IsOldCommittee() {
		input = keygen.BuildLocalSaveDataSubset(key, params.OldParties().IDs())
	}
	save := keygen.NewLocalPartySaveData(newPC)
	if preParams.ValidateWithProof() {
		save.LocalPreParams = preParams
	}

	temp := &localTempData{
		localMessageStore: localMessageStore{
			dgRound1Messages:  make([]*tss.Message, oldPC),
			dgRound2Message1s: make([]*tss.Message, newPC),
			dgRound2Message2s: make([]*tss.Message, newPC),
			dgRound3Message1s: make([]*tss.Message, oldPC),
			dgRound3Message2s: make([]*tss.Message, oldPC),
			dgRound4Message1s: make([]*tss.Message, newPC),
			dgRound4Message2s: make([]*tss.Message, newPC),
		},
	}

	state := &ReshareState{params: params, input: &input, save: &save, temp: temp}
	out := &ReshareRoundOutput{}

	if !params.IsOldCommittee() {
		return state, out, nil
	}

	temp.ssidNonce = new(big.Int).SetUint64(uint64(params.SSIDNonce()))
	ssid, err := getReshareSSID(params, &input, temp, 1)
	if err != nil {
		return nil, nil, err
	}
	temp.ssid = ssid

	Pi := params.PartyID()
	i := Pi.Index
	xi, ks, bigXj := input.Xi, input.Ks, input.BigXj
	if params.Threshold()+1 > len(ks) {
		return nil, nil, fmt.Errorf("t+1=%d > key count %d", params.Threshold()+1, len(ks))
	}
	newKs := params.NewParties().IDs().Keys()
	wi, _ := signing.PrepareForSigning(params.EC(), i, len(params.OldParties().IDs()), xi, ks, bigXj)

	vi, shares, poly, err := vss.Create(params.EC(), params.NewThreshold(), wi, newKs, params.Rand())
	if err != nil {
		return nil, nil, err
	}

	flatVis, err := crypto.FlattenECPoints(vi)
	if err != nil {
		return nil, nil, err
	}
	vCmt := commitments.NewHashCommitment(params.Rand(), flatVis...)

	temp.VD = vCmt.D
	temp.NewShares = shares
	temp.NewVs = vi
	temp.Poly = poly

	r1msg := NewDGRound1Message(
		params.NewParties().IDs().Exclude(Pi), Pi,
		input.ECDSAPub, vCmt.C, ssid)
	temp.dgRound1Messages[i] = r1msg
	out.Messages = append(out.Messages, r1msg)
	out.Poly = poly
	out.NewVs = vi

	return state, out, nil
}

// ReshareRound2 processes round 1 messages from the old committee and
// produces Pedersen parameters + proofs for the new committee.
//
// r1Msgs are DGRound1Message broadcasts from old committee.
// Only new committee members produce output.
func ReshareRound2(state *ReshareState, r1Msgs []*tss.Message) (*ReshareRoundOutput, error) {
	params, save, temp := state.params, state.save, state.temp
	tss.MergeMsgs(temp.dgRound1Messages, r1Msgs)
	out := &ReshareRoundOutput{}

	if !params.IsNewCommittee() {
		return out, nil
	}

	Pi := params.PartyID()
	i := Pi.Index

	// Validate SSID consistency across old committee
	r1msg0 := r1Msgs[0].Content.(*DGRound1Message)
	SSID := r1msg0.SSID
	for j := range params.OldParties().IDs() {
		if j == 0 {
			continue
		}
		r1msg := r1Msgs[j].Content.(*DGRound1Message)
		SSIDj := r1msg.SSID
		if !bytes.Equal(SSID, SSIDj) {
			return nil, tss.NewError(errors.New("ssid mismatch"), TaskName, 2, Pi, params.OldParties().IDs()[j])
		}
	}
	temp.ssid = SSID

	// Save ECDSAPub from old committee
	for j := range params.OldParties().IDs() {
		r1msg := r1Msgs[j].Content.(*DGRound1Message)
		candidate := r1msg.ECDSAPub
		if candidate == nil {
			return nil, fmt.Errorf("round 2: ecdsa pub nil from party %d", j)
		}
		if save.ECDSAPub != nil && !candidate.Equals(save.ECDSAPub) {
			return nil, errors.New("ecdsa pub key mismatch from old committee")
		}
		save.ECDSAPub = candidate
	}

	// ACK to old committee
	r2msg1 := NewDGRound2Message2(
		params.OldParties().IDs().Exclude(Pi), Pi)
	temp.dgRound2Message2s[i] = r2msg1
	out.Messages = append(out.Messages, r2msg1)

	// Generate pre-params if not provided
	var preParams *keygen.LocalPreParams
	if save.Validate() && !save.ValidateWithProof() {
		return nil, errors.New("preParams failed validation")
	} else if save.ValidateWithProof() {
		preParams = &save.LocalPreParams
	} else {
		var err error
		preParams, err = keygen.GeneratePreParams(params.SafePrimeGenTimeout(), params.Concurrency())
		if err != nil {
			return nil, errors.New("pre-params generation failed")
		}
	}
	save.LocalPreParams = *preParams
	save.NTildej[i] = preParams.NTildei
	save.H1j[i], save.H2j[i] = preParams.H1i, preParams.H2i

	h1i, h2i, alpha, beta := preParams.H1i, preParams.H2i, preParams.Alpha, preParams.Beta
	p, q, NTildei := preParams.P, preParams.Q, preParams.NTildei
	ContextI := common.AppendBigIntToBytesSlice(temp.ssid, big.NewInt(int64(i)))

	var dlnProof1, dlnProof2 *dlnproof.Proof
	if !params.NoProofDLN() {
		dlnProof1 = dlnproof.NewDLNProof(ContextI, h1i, h2i, alpha, p, q, NTildei, params.Rand())
		dlnProof2 = dlnproof.NewDLNProof(ContextI, h2i, h1i, beta, p, q, NTildei, params.Rand())
	}

	var modProofObj *modproof.ProofMod
	if !params.NoProofMod() {
		var err error
		modProofObj, err = modproof.NewProof(ContextI, preParams.PaillierSK.N,
			preParams.PaillierSK.P, preParams.PaillierSK.Q, params.Rand())
		if err != nil {
			return nil, fmt.Errorf("round 2 mod proof: %w", err)
		}
	}

	r2msg2 := NewDGRound2Message1(
		params.NewParties().IDs().Exclude(Pi), Pi,
		&preParams.PaillierSK.PublicKey, modProofObj,
		preParams.NTildei, preParams.H1i, preParams.H2i,
		dlnProof1, dlnProof2)
	temp.dgRound2Message1s[i] = r2msg2
	out.Messages = append(out.Messages, r2msg2)

	save.PaillierSK = preParams.PaillierSK
	save.PaillierPKs[i] = &preParams.PaillierSK.PublicKey

	return out, nil
}

// ReshareRound3 sends VSS shares to new committee members.
// Only old committee members produce output.
//
// r2AckMsgs are DGRound2Message2 broadcasts from new committee.
func ReshareRound3(state *ReshareState, r2AckMsgs []*tss.Message) (*ReshareRoundOutput, error) {
	params, temp := state.params, state.temp
	tss.MergeMsgs(temp.dgRound2Message2s, r2AckMsgs)
	out := &ReshareRoundOutput{}

	if !params.IsOldCommittee() {
		return out, nil
	}

	Pi := params.PartyID()
	i := Pi.Index

	for j, Pj := range params.NewParties().IDs() {
		share := temp.NewShares[j]
		r3msg1 := NewDGRound3Message1(Pj, Pi, share)
		temp.dgRound3Message1s[i] = r3msg1
		out.Messages = append(out.Messages, r3msg1)
	}

	r3msg2 := NewDGRound3Message2(
		params.NewParties().IDs().Exclude(Pi), Pi, temp.VD)
	temp.dgRound3Message2s[i] = r3msg2
	out.Messages = append(out.Messages, r3msg2)

	return out, nil
}

// ReshareRound4 verifies new committee parameters and old committee
// shares, computes the new Xi and BigXj, and produces FacProof +
// ACK messages.  Only new committee members produce output.
//
// r2NewMsgs are DGRound2Message1 broadcasts from new committee.
// r3P2P[j] is old party j's DGRound3Message1 (P2P share).
// r3Bcast[j] is old party j's DGRound3Message2 (decommitment).
func ReshareRound4(
	ctx context.Context,
	state *ReshareState,
	r2NewMsgs []*tss.Message,
	r3P2P, r3Bcast []*tss.Message,
) (*ReshareRoundOutput, error) {
	params, save, temp := state.params, state.save, state.temp
	tss.MergeMsgs(temp.dgRound2Message1s, r2NewMsgs)
	tss.MergeMsgs(temp.dgRound3Message1s, r3P2P)
	tss.MergeMsgs(temp.dgRound3Message2s, r3Bcast)
	out := &ReshareRoundOutput{}

	if !params.IsNewCommittee() {
		return out, nil
	}

	dlnVerifier := keygen.NewDlnProofVerifier(params.Concurrency())
	Pi := params.PartyID()
	i := Pi.Index

	// Parameter validation
	h1H2Map := make(map[string]struct{}, len(r2NewMsgs)*2)
	paillierNMap := make(map[string]struct{}, len(r2NewMsgs))
	nTildeMap := make(map[string]struct{}, len(r2NewMsgs))
	paiProofCulprits := make([]*tss.PartyID, len(r2NewMsgs))
	dlnProof1FailCulprits := make([]*tss.PartyID, len(r2NewMsgs))
	dlnProof2FailCulprits := make([]*tss.PartyID, len(r2NewMsgs))
	wg := new(sync.WaitGroup)
	gctx, gcancel := context.WithCancel(ctx)
	defer gcancel()
	for j, msg := range r2NewMsgs {
		r2msg1 := msg.Content.(*DGRound2Message1)
		paiPK, NTildej, H1j, H2j := r2msg1.PaillierPK,
			r2msg1.NTilde, r2msg1.H1, r2msg1.H2
		if H1j.Cmp(H2j) == 0 {
			return nil, tss.NewError(errors.New("h1j == h2j"), TaskName, 4, Pi, msg.From)
		}
		if H1j.Cmp(reshareOne) == 0 || H2j.Cmp(reshareOne) == 0 {
			return nil, tss.NewError(errors.New("h1j or h2j is 1"), TaskName, 4, Pi, msg.From)
		}
		if paiPK.N.BitLen() < resharePaiBitsLen {
			return nil, tss.NewError(errors.New("paillier N insufficient bits"), TaskName, 4, Pi, msg.From)
		}
		if paiPK.N.Bit(0) == 0 {
			return nil, tss.NewError(errors.New("even paillier N"), TaskName, 4, Pi, msg.From)
		}
		if paiPK.N.ProbablyPrime(20) {
			return nil, tss.NewError(errors.New("prime paillier N"), TaskName, 4, Pi, msg.From)
		}
		sqrtN := new(big.Int).Sqrt(paiPK.N)
		if new(big.Int).Mul(sqrtN, sqrtN).Cmp(paiPK.N) == 0 {
			return nil, tss.NewError(errors.New("perfect-square paillier N"), TaskName, 4, Pi, msg.From)
		}
		if NTildej.BitLen() < resharePaiBitsLen {
			return nil, tss.NewError(errors.New("NTildej insufficient bits"), TaskName, 4, Pi, msg.From)
		}
		if NTildej.Bit(0) == 0 {
			return nil, tss.NewError(errors.New("even NTildej"), TaskName, 4, Pi, msg.From)
		}
		if NTildej.ProbablyPrime(20) {
			return nil, tss.NewError(errors.New("prime NTildej"), TaskName, 4, Pi, msg.From)
		}
		sqrtNT := new(big.Int).Sqrt(NTildej)
		if new(big.Int).Mul(sqrtNT, sqrtNT).Cmp(NTildej) == 0 {
			return nil, tss.NewError(errors.New("perfect-square NTildej"), TaskName, 4, Pi, msg.From)
		}
		if paiPK.N.Cmp(NTildej) == 0 {
			return nil, tss.NewError(errors.New("paillier N == NTilde"), TaskName, 4, Pi, msg.From)
		}
		if new(big.Int).GCD(nil, nil, H1j, NTildej).Cmp(reshareOne) != 0 {
			return nil, tss.NewError(errors.New("h1j not coprime with NTildej"), TaskName, 4, Pi, msg.From)
		}
		if new(big.Int).GCD(nil, nil, H2j, NTildej).Cmp(reshareOne) != 0 {
			return nil, tss.NewError(errors.New("h2j not coprime with NTildej"), TaskName, 4, Pi, msg.From)
		}
		h1Hex, h2Hex := hex.EncodeToString(H1j.Bytes()), hex.EncodeToString(H2j.Bytes())
		if _, found := h1H2Map[h1Hex]; found {
			return nil, tss.NewError(errors.New("duplicate h1j"), TaskName, 4, Pi, msg.From)
		}
		if _, found := h1H2Map[h2Hex]; found {
			return nil, tss.NewError(errors.New("duplicate h2j"), TaskName, 4, Pi, msg.From)
		}
		h1H2Map[h1Hex], h1H2Map[h2Hex] = struct{}{}, struct{}{}
		paiNHex := hex.EncodeToString(paiPK.N.Bytes())
		if _, found := paillierNMap[paiNHex]; found {
			return nil, tss.NewError(errors.New("duplicate paillier N"), TaskName, 4, Pi, msg.From)
		}
		paillierNMap[paiNHex] = struct{}{}
		ntHex := hex.EncodeToString(NTildej.Bytes())
		if _, found := nTildeMap[ntHex]; found {
			return nil, tss.NewError(errors.New("duplicate NTilde"), TaskName, 4, Pi, msg.From)
		}
		nTildeMap[ntHex] = struct{}{}

		nTasks := 1
		if !params.NoProofDLN() {
			nTasks = 3
		}
		wg.Add(nTasks)
		go func(j int, msg *tss.Message, r2msg1 *DGRound2Message1) {
			defer wg.Done()
			if gctx.Err() != nil {
				return
			}
			if params.NoProofMod() {
				return
			}
			modProof := r2msg1.ModProof
			if modProof == nil {
				paiProofCulprits[j] = msg.From
				gcancel()
				return
			}
			ContextJ := common.AppendBigIntToBytesSlice(temp.ssid, big.NewInt(int64(j)))
			if ok := modProof.Verify(ContextJ, paiPK.N); !ok {
				paiProofCulprits[j] = msg.From
				gcancel()
			}
		}(j, msg, r2msg1)
		if !params.NoProofDLN() {
			_j, _msg := j, msg
			ContextJ := common.AppendBigIntToBytesSlice(temp.ssid, big.NewInt(int64(j)))
			dlnVerifier.VerifyDLNProof(r2msg1.DLNProof1, ContextJ, H1j, H2j, NTildej, func(isValid bool) {
				if !isValid {
					dlnProof1FailCulprits[_j] = _msg.From
					gcancel()
				}
				wg.Done()
			})
			dlnVerifier.VerifyDLNProof(r2msg1.DLNProof2, ContextJ, H2j, H1j, NTildej, func(isValid bool) {
				if !isValid {
					dlnProof2FailCulprits[_j] = _msg.From
					gcancel()
				}
				wg.Done()
			})
		}
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, culprits := range [][]*tss.PartyID{paiProofCulprits, dlnProof1FailCulprits, dlnProof2FailCulprits} {
		for _, c := range culprits {
			if c != nil {
				return nil, tss.NewError(errors.New("proof verification failed"), TaskName, 4, Pi, c)
			}
		}
	}

	// Save NTilde, H1, H2 from new committee
	for j, msg := range r2NewMsgs {
		if j == i {
			continue
		}
		r2msg1 := msg.Content.(*DGRound2Message1)
		save.NTildej[j] = r2msg1.NTilde
		save.H1j[j] = r2msg1.H1
		save.H2j[j] = r2msg1.H2
	}

	// Verify old committee shares and commitments
	modQ := common.ModInt(params.EC().Params().N)
	newXi := big.NewInt(0)
	vjc := make([][]*crypto.ECPoint, len(params.OldParties().IDs()))
	for j := 0; j < len(vjc); j++ {
		r1msg := temp.dgRound1Messages[j].Content.(*DGRound1Message)
		r3msg2 := r3Bcast[j].Content.(*DGRound3Message2)
		vCj, vDj := r1msg.VCommitment, r3msg2.VDeCommitment
		cmtDeCmt := commitments.HashCommitDecommit{C: vCj, D: vDj}
		ok, flatVs := cmtDeCmt.DeCommit()
		if !ok || len(flatVs) != (params.NewThreshold()+1)*2 {
			return nil, tss.NewError(errors.New("v decommit failed"), TaskName, 4, Pi, params.OldParties().IDs()[j])
		}
		vj, err := crypto.UnFlattenECPoints(params.EC(), flatVs)
		if err != nil {
			return nil, tss.NewError(err, TaskName, 4, Pi, params.OldParties().IDs()[j])
		}
		vjc[j] = vj

		r3msg1 := r3P2P[j].Content.(*DGRound3Message1)
		myKey := Pi.KeyInt().Bytes()
		if !bytes.Equal(r3msg1.ReceiverID, myKey) {
			return nil, tss.NewError(errors.New("receiverId mismatch"), TaskName, 4, Pi, params.OldParties().IDs()[j])
		}
		sharej := &vss.Share{
			Threshold: params.NewThreshold(),
			ID:        Pi.KeyInt(),
			Share:     r3msg1.Share,
		}
		if ok := sharej.Verify(params.EC(), params.NewThreshold(), vj); !ok {
			return nil, tss.NewError(errors.New("vss share verify failed"), TaskName, 4, Pi, params.OldParties().IDs()[j])
		}
		newXi = new(big.Int).Add(newXi, sharej.Share)
	}
	newXi = new(big.Int).Mod(newXi, params.EC().Params().N)
	if newXi.Sign() == 0 {
		return nil, errors.New("newXi is zero")
	}

	// Compute Vc
	var err error
	Vc := make([]*crypto.ECPoint, params.NewThreshold()+1)
	for c := 0; c <= params.NewThreshold(); c++ {
		Vc[c] = vjc[0][c]
		for j := 1; j < len(vjc); j++ {
			Vc[c], err = Vc[c].Add(vjc[j][c])
			if err != nil {
				return nil, errors2.Wrapf(err, "Vc[%d].Add(vjc[%d][%d])", c, j, c)
			}
		}
	}
	if !Vc[0].Equals(save.ECDSAPub) {
		return nil, errors.New("V_0 != ECDSAPub")
	}

	// Compute newBigXjs
	newKs := make([]*big.Int, 0, params.NewPartyCount())
	newBigXjs := make([]*crypto.ECPoint, params.NewPartyCount())
	culprits := make([]*tss.PartyID, 0)
	for j := 0; j < params.NewPartyCount(); j++ {
		Pj := params.NewParties().IDs()[j]
		kj := Pj.KeyInt()
		newBigXj := Vc[0]
		newKs = append(newKs, kj)
		z := new(big.Int).SetInt64(1)
		for c := 1; c <= params.NewThreshold(); c++ {
			z = modQ.Mul(z, kj)
			newBigXj, err = newBigXj.Add(Vc[c].ScalarMult(z))
			if err != nil {
				culprits = append(culprits, Pj)
				break
			}
		}
		if newBigXj.IsIdentity() {
			culprits = append(culprits, Pj)
		} else {
			newBigXjs[j] = newBigXj
		}
	}
	if len(culprits) > 0 {
		return nil, tss.NewError(errors.New("newBigXj identity or computation error"), TaskName, 4, Pi, culprits...)
	}

	temp.newXi = newXi
	temp.newKs = newKs
	temp.newBigXjs = newBigXjs

	// Send FacProof to new parties
	for j, Pj := range params.NewParties().IDs() {
		if j == i {
			continue
		}
		var fp *facproof.ProofFac
		if !params.NoProofFac() {
			ContextJ := common.AppendBigIntToBytesSlice(temp.ssid, big.NewInt(int64(j)))
			fp, err = facproof.NewProof(ContextJ, params.EC(), save.PaillierSK.N,
				save.NTildej[j], save.H1j[j], save.H2j[j],
				save.PaillierSK.P, save.PaillierSK.Q, params.Rand())
			if err != nil {
				return nil, fmt.Errorf("round 5 fac proof for party %d: %w", j, err)
			}
		}
		r4msg1 := NewDGRound4Message1(Pj, Pi, fp)
		out.Messages = append(out.Messages, r4msg1)
	}

	// ACK to both committees
	r4msg2 := NewDGRound4Message2(params.OldAndNewParties(), Pi)
	temp.dgRound4Message2s[i] = r4msg2
	out.Messages = append(out.Messages, r4msg2)

	return out, nil
}

// ReshareRound5 verifies FacProofs and saves the new key share.
// Only new committee members produce output (the final Save).
//
// r4P2P[j] is new party j's DGRound4Message1 (P2P FacProof).
// r4Bcast[j] is new party j's DGRound4Message2 (ACK broadcast).
func ReshareRound5(
	state *ReshareState,
	r4P2P, r4Bcast []*tss.Message,
) (*ReshareRoundOutput, error) {
	params, save, temp, input := state.params, state.save, state.temp, state.input
	tss.MergeMsgs(temp.dgRound4Message1s, r4P2P)
	tss.MergeMsgs(temp.dgRound4Message2s, r4Bcast)
	out := &ReshareRoundOutput{}

	Pi := params.PartyID()
	i := Pi.Index

	if params.IsNewCommittee() {
		ContextI := common.AppendBigIntToBytesSlice(temp.ssid, big.NewInt(int64(i)))
		save.BigXj = temp.newBigXjs
		save.ShareID = Pi.KeyInt()
		save.Xi = temp.newXi
		save.Ks = temp.newKs

		for j, msg := range temp.dgRound2Message1s {
			if j == i {
				continue
			}
			r2msg1 := msg.Content.(*DGRound2Message1)
			save.PaillierPKs[j] = r2msg1.PaillierPK
		}
		for j, msg := range r4P2P {
			if j == i {
				continue
			}
			r4msg1 := msg.Content.(*DGRound4Message1)
			receiverId := r4msg1.ReceiverID
			if !bytes.Equal(receiverId, Pi.Key) {
				return nil, tss.NewError(errors.New("DGRound4Message1 receiverId mismatch"),
					TaskName, 5, Pi, params.NewParties().IDs()[j])
			}
			if params.NoProofFac() {
				continue
			}
			proof := r4msg1.FacProof
			if proof == nil {
				return nil, tss.NewError(errors.New("facProof missing"), TaskName, 5, Pi, params.NewParties().IDs()[j])
			}
			if ok := proof.Verify(ContextI, params.EC(), save.PaillierPKs[j].N,
				save.NTildei, save.H1i, save.H2i); !ok {
				return nil, tss.NewError(errors.New("facProof verify failed"),
					TaskName, 5, Pi, params.NewParties().IDs()[j])
			}
		}
		out.Save = save
	}

	// Zero old Xi
	if params.IsOldCommittee() {
		input.Xi.SetInt64(0)
	}

	return out, nil
}
