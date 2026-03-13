// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

// Channel-free signing round functions.  Each round takes explicit
// state + inbound messages and returns outbound messages.
//
// The crypto is identical to the channel-based Start() methods in
// round_1.go through finalize.go.  This file exists so callers can
// drive the signing protocol without channels, goroutines, or the
// recursive BaseUpdate state machine.

package signing

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"sync"

	errorspkg "github.com/pkg/errors"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/crypto"
	"github.com/hemilabs/x/tss-lib/v2/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v2/crypto/mta"
	"github.com/hemilabs/x/tss-lib/v2/crypto/schnorr"
	"github.com/hemilabs/x/tss-lib/v2/ecdsa/keygen"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

// getSigningSSID computes the SSID for signing rounds.
func getSigningSSID(params *tss.Parameters, key *keygen.LocalPartySaveData, temp *localTempData, roundNumber int) ([]byte, error) {
	ssidList := []*big.Int{
		new(big.Int).SetBytes([]byte("ecdsa-signing")),
		params.EC().Params().P, params.EC().Params().N,
		params.EC().Params().B, params.EC().Params().Gx,
		params.EC().Params().Gy,
	}
	ssidList = append(ssidList, params.Parties().IDs().Keys()...)
	BigXjList, err := crypto.FlattenECPoints(key.BigXj)
	if err != nil {
		return nil, fmt.Errorf("flatten ec points: %w", err)
	}
	ssidList = append(ssidList, BigXjList...)
	ssidList = append(ssidList, key.NTildej...)
	ssidList = append(ssidList, key.H1j...)
	ssidList = append(ssidList, key.H2j...)
	ssidList = append(ssidList, big.NewInt(int64(params.PartyCount())))
	ssidList = append(ssidList, big.NewInt(int64(params.Threshold())))
	ssidList = append(ssidList, big.NewInt(int64(roundNumber)))
	ssidList = append(ssidList, temp.ssidNonce)
	if cid := params.CeremonyID(); len(cid) > 0 {
		ssidList = append(ssidList, new(big.Int).SetBytes(cid))
	}
	if temp.m != nil {
		ssidList = append(ssidList, temp.m)
	}
	return common.SHA512_256i(ssidList...).Bytes(), nil
}

// SignRound1 initializes the signing state and produces MtA messages.
//
// msg is the hash to sign (must be in [0, N)).
// key is the party's saved keygen data.
// keyDerivationDelta is optional HD derivation delta (nil for no derivation).
// fullBytesLen is the byte length for message padding (0 for default).
func SignRound1(
	params *tss.Parameters,
	key keygen.LocalPartySaveData,
	msg *big.Int,
	keyDerivationDelta *big.Int,
	fullBytesLen int,
) (*SigningState, *SignRoundOutput, error) {
	n := params.PartyCount()
	temp := &localTempData{
		localMessageStore: localMessageStore{
			signRound1Message1s: make([]tss.ParsedMessage, n),
			signRound1Message2s: make([]tss.ParsedMessage, n),
			signRound2Messages:  make([]tss.ParsedMessage, n),
			signRound3Messages:  make([]tss.ParsedMessage, n),
			signRound4Messages:  make([]tss.ParsedMessage, n),
			signRound5Messages:  make([]tss.ParsedMessage, n),
			signRound6Messages:  make([]tss.ParsedMessage, n),
			signRound7Messages:  make([]tss.ParsedMessage, n),
			signRound8Messages:  make([]tss.ParsedMessage, n),
			signRound9Messages:  make([]tss.ParsedMessage, n),
		},
		cis:          make([]*big.Int, n),
		bigWs:        make([]*crypto.ECPoint, n),
		m:            msg,
		fullBytesLen: fullBytesLen,
		keyDerivationDelta: keyDerivationDelta,
		betas:  make([]*big.Int, n),
		c1jis:  make([]*big.Int, n),
		c2jis:  make([]*big.Int, n),
		vs:     make([]*big.Int, n),
		pi1jis: make([]*mta.ProofBob, n),
		pi2jis: make([]*mta.ProofBobWC, n),
	}
	data := &common.SignatureData{}

	// Validate message
	if msg.Sign() < 0 || msg.Cmp(params.EC().Params().N) >= 0 {
		return nil, nil, errors.New("hashed message is not valid")
	}

	// Auto-subset: if the key has more parties than the signing
	// committee (threshold signing with a subset of keygen parties),
	// trim the key data to only the signing parties.  This avoids
	// requiring callers to call BuildLocalSaveDataSubset manually.
	if len(key.Ks) > params.PartyCount() {
		key = keygen.BuildLocalSaveDataSubset(key, params.Parties().IDs())
	}
	if len(key.Ks) != params.PartyCount() {
		return nil, nil, fmt.Errorf("key count %d != party count %d after subset", len(key.Ks), params.PartyCount())
	}
	if params.Threshold()+1 > len(key.Ks) {
		return nil, nil, fmt.Errorf("t+1=%d > key count %d", params.Threshold()+1, len(key.Ks))
	}

	// Prepare (Lagrange coefficients)
	i := params.PartyID().Index
	xi := key.Xi
	if keyDerivationDelta != nil {
		mod := common.ModInt(params.EC().Params().N)
		xi = mod.Add(keyDerivationDelta, xi)
	}
	wi, bigWs := PrepareForSigning(params.EC(), i, len(key.Ks), xi, key.Ks, key.BigXj)
	temp.w = wi
	temp.bigWs = bigWs

	// SSID
	temp.ssidNonce = new(big.Int).SetUint64(uint64(params.SSIDNonce()))
	ssid, err := getSigningSSID(params, &key, temp, 1)
	if err != nil {
		return nil, nil, err
	}
	temp.ssid = ssid

	// Generate k, gamma
	k := common.GetRandomPositiveInt(params.Rand(), params.EC().Params().N)
	gamma := common.GetRandomPositiveInt(params.Rand(), params.EC().Params().N)
	pointGamma := crypto.ScalarBaseMult(params.EC(), gamma)
	cmt := commitments.NewHashCommitment(params.Rand(), pointGamma.X(), pointGamma.Y())
	temp.k = k
	temp.gamma = gamma
	temp.pointGamma = pointGamma
	temp.deCommit = cmt.D

	// MtA init for each peer
	out := &SignRoundOutput{}
	ContextI := common.AppendBigIntToBytesSlice(ssid, new(big.Int).SetUint64(uint64(i)))
	for j, Pj := range params.Parties().IDs() {
		if j == i {
			continue
		}
		cA, pi, err := mta.AliceInit(ContextI, params.EC(), key.PaillierPKs[i], k,
			key.NTildej[j], key.H1j[j], key.H2j[j], params.Rand())
		if err != nil {
			return nil, nil, fmt.Errorf("mta AliceInit: %v", err)
		}
		r1msg1 := NewSignRound1Message1(Pj, params.PartyID(), cA, pi)
		temp.cis[j] = cA
		out.Messages = append(out.Messages, r1msg1)
	}

	r1msg2 := NewSignRound1Message2(params.PartyID(), cmt.C)
	temp.signRound1Message2s[i] = r1msg2
	out.Messages = append(out.Messages, r1msg2)

	state := &SigningState{params: params, key: &key, data: data, temp: temp}
	return state, out, nil
}

// SignRound2 processes round 1 messages and runs MtA Bob.
//
// r1p2p[j] is party j's SignRound1Message1 (P2P).
// r1bcast[j] is party j's SignRound1Message2 (broadcast).
func SignRound2(ctx context.Context, state *SigningState, r1p2p, r1bcast []tss.ParsedMessage) (*SignRoundOutput, error) {
	params, key, temp := state.params, state.key, state.temp
	tss.MergeMsgs(temp.signRound1Message1s, r1p2p)
	tss.MergeMsgs(temp.signRound1Message2s, r1bcast)

	i := params.PartyID().Index

	// ReceiverID check
	myKey := params.PartyID().KeyInt().Bytes()
	for j, Pj := range params.Parties().IDs() {
		if j == i {
			continue
		}
		r1msg := r1p2p[j].Content().(*SignRound1Message1)
		if !bytes.Equal(r1msg.GetReceiverId(), myKey) {
			return nil, tss.NewError(errors.New("receiverId mismatch"), TaskName, 2, params.PartyID(), Pj)
		}
	}

	errs := make([]*tss.Error, len(params.Parties().IDs()))
	gctx, gcancel := context.WithCancel(ctx)
	defer gcancel()
	wg := sync.WaitGroup{}
	wg.Add((len(params.Parties().IDs()) - 1) * 2)
	ContextI := common.AppendBigIntToBytesSlice(temp.ssid, new(big.Int).SetUint64(uint64(i)))
	for j, Pj := range params.Parties().IDs() {
		if j == i {
			continue
		}
		// Bob_mid
		go func(j int, Pj *tss.PartyID) {
			defer wg.Done()
			if gctx.Err() != nil {
				return
			}
			r1msg := r1p2p[j].Content().(*SignRound1Message1)
			rangeProofAliceJ, err := r1msg.UnmarshalRangeProofAlice()
			if err != nil {
				errs[j] = tss.NewError(errorspkg.Wrapf(err, "UnmarshalRangeProofAlice"), TaskName, 2, params.PartyID(), Pj)
				gcancel()
				return
			}
			if gctx.Err() != nil {
				return
			}
			AliceContextJ := common.AppendBigIntToBytesSlice(temp.ssid, new(big.Int).SetUint64(uint64(j)))
			beta, c1ji, _, pi1ji, err := mta.BobMid(
				AliceContextJ, ContextI, params.EC(), key.PaillierPKs[j],
				rangeProofAliceJ, temp.gamma, r1msg.UnmarshalC(),
				key.NTildej[j], key.H1j[j], key.H2j[j],
				key.NTildej[i], key.H1j[i], key.H2j[i], params.Rand())
			if err != nil {
				errs[j] = tss.NewError(err, TaskName, 2, params.PartyID(), Pj)
				gcancel()
				return
			}
			temp.betas[j] = beta
			temp.c1jis[j] = c1ji
			temp.pi1jis[j] = pi1ji
		}(j, Pj)
		// Bob_mid_wc
		go func(j int, Pj *tss.PartyID) {
			defer wg.Done()
			if gctx.Err() != nil {
				return
			}
			r1msg := r1p2p[j].Content().(*SignRound1Message1)
			rangeProofAliceJ, err := r1msg.UnmarshalRangeProofAlice()
			if err != nil {
				errs[j] = tss.NewError(errorspkg.Wrapf(err, "UnmarshalRangeProofAlice"), TaskName, 2, params.PartyID(), Pj)
				gcancel()
				return
			}
			if gctx.Err() != nil {
				return
			}
			AliceContextJ := common.AppendBigIntToBytesSlice(temp.ssid, new(big.Int).SetUint64(uint64(j)))
			v, c2ji, _, pi2ji, err := mta.BobMidWC(
				AliceContextJ, ContextI, params.EC(), key.PaillierPKs[j],
				rangeProofAliceJ, temp.w, r1msg.UnmarshalC(),
				key.NTildej[j], key.H1j[j], key.H2j[j],
				key.NTildej[i], key.H1j[i], key.H2j[i],
				temp.bigWs[i], params.Rand())
			if err != nil {
				errs[j] = tss.NewError(err, TaskName, 2, params.PartyID(), Pj)
				gcancel()
				return
			}
			temp.vs[j] = v
			temp.c2jis[j] = c2ji
			temp.pi2jis[j] = pi2ji
		}(j, Pj)
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	out := &SignRoundOutput{}
	for j, Pj := range params.Parties().IDs() {
		if j == i {
			continue
		}
		r2msg := NewSignRound2Message(Pj, params.PartyID(),
			temp.c1jis[j], temp.pi1jis[j], temp.c2jis[j], temp.pi2jis[j])
		out.Messages = append(out.Messages, r2msg)
	}
	return out, nil
}

// SignRound3 processes round 2 MtA responses and computes theta/sigma.
func SignRound3(ctx context.Context, state *SigningState, r2p2p []tss.ParsedMessage) (*SignRoundOutput, error) {
	params, key, temp := state.params, state.key, state.temp
	tss.MergeMsgs(temp.signRound2Messages, r2p2p)

	n := len(params.Parties().IDs())
	alphas := make([]*big.Int, n)
	us := make([]*big.Int, n)
	i := params.PartyID().Index

	// ReceiverID check
	myKey := params.PartyID().KeyInt().Bytes()
	for j, Pj := range params.Parties().IDs() {
		if j == i {
			continue
		}
		r2msg := r2p2p[j].Content().(*SignRound2Message)
		if !bytes.Equal(r2msg.GetReceiverId(), myKey) {
			return nil, tss.NewError(errors.New("receiverId mismatch"), TaskName, 3, params.PartyID(), Pj)
		}
	}

	errs := make([]*tss.Error, n)
	gctx, gcancel := context.WithCancel(ctx)
	defer gcancel()
	wg := sync.WaitGroup{}
	wg.Add((n - 1) * 2)
	for j, Pj := range params.Parties().IDs() {
		if j == i {
			continue
		}
		ContextJ := common.AppendBigIntToBytesSlice(temp.ssid, new(big.Int).SetUint64(uint64(j)))
		go func(j int, Pj *tss.PartyID) {
			defer wg.Done()
			if gctx.Err() != nil {
				return
			}
			r2msg := r2p2p[j].Content().(*SignRound2Message)
			proofBob, err := r2msg.UnmarshalProofBob()
			if err != nil {
				errs[j] = tss.NewError(errorspkg.Wrapf(err, "UnmarshalProofBob"), TaskName, 3, params.PartyID(), Pj)
				gcancel()
				return
			}
			if gctx.Err() != nil {
				return
			}
			alphaIj, err := mta.AliceEnd(ContextJ, params.EC(), key.PaillierPKs[i],
				proofBob, key.H1j[i], key.H2j[i], temp.cis[j],
				new(big.Int).SetBytes(r2msg.GetC1()), key.NTildej[i], key.PaillierSK)
			if err != nil {
				errs[j] = tss.NewError(err, TaskName, 3, params.PartyID(), Pj)
				gcancel()
				return
			}
			alphas[j] = alphaIj
		}(j, Pj)
		go func(j int, Pj *tss.PartyID) {
			defer wg.Done()
			if gctx.Err() != nil {
				return
			}
			r2msg := r2p2p[j].Content().(*SignRound2Message)
			proofBobWC, err := r2msg.UnmarshalProofBobWC(params.EC())
			if err != nil {
				errs[j] = tss.NewError(errorspkg.Wrapf(err, "UnmarshalProofBobWC"), TaskName, 3, params.PartyID(), Pj)
				gcancel()
				return
			}
			if gctx.Err() != nil {
				return
			}
			uIj, err := mta.AliceEndWC(ContextJ, params.EC(), key.PaillierPKs[i],
				proofBobWC, temp.bigWs[j], temp.cis[j],
				new(big.Int).SetBytes(r2msg.GetC2()), key.NTildej[i],
				key.H1j[i], key.H2j[i], key.PaillierSK)
			if err != nil {
				errs[j] = tss.NewError(err, TaskName, 3, params.PartyID(), Pj)
				gcancel()
				return
			}
			us[j] = uIj
		}(j, Pj)
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	modN := common.ModInt(params.EC().Params().N)
	theta := modN.Mul(temp.k, temp.gamma)
	sigma := modN.Mul(temp.k, temp.w)
	for j := range params.Parties().IDs() {
		if j == i {
			continue
		}
		theta = modN.Add(theta, new(big.Int).Add(alphas[j], temp.betas[j]))
		sigma = modN.Add(sigma, new(big.Int).Add(us[j], temp.vs[j]))
	}
	temp.theta = theta
	temp.sigma = sigma

	r3msg := NewSignRound3Message(params.PartyID(), theta)
	temp.signRound3Messages[i] = r3msg
	return &SignRoundOutput{Messages: []tss.Message{r3msg}}, nil
}

// SignRound4 computes thetaInverse and Schnorr proof for gamma.
func SignRound4(state *SigningState, r3bcast []tss.ParsedMessage) (*SignRoundOutput, error) {
	params, temp := state.params, state.temp
	tss.MergeMsgs(temp.signRound3Messages, r3bcast)

	theta := new(big.Int).Set(temp.theta)
	modN := common.ModInt(params.EC().Params().N)
	i := params.PartyID().Index

	for j := range params.Parties().IDs() {
		if j == i {
			continue
		}
		r3msg := r3bcast[j].Content().(*SignRound3Message)
		thetaJ := r3msg.GetTheta()
		theta = modN.Add(theta, new(big.Int).SetBytes(thetaJ))
	}
	thetaInverse := modN.ModInverse(theta)
	if thetaInverse == nil {
		return nil, errors.New("theta is zero")
	}

	ContextI := common.AppendBigIntToBytesSlice(temp.ssid, new(big.Int).SetUint64(uint64(i)))
	piGamma, err := schnorr.NewZKProof(ContextI, temp.gamma, temp.pointGamma, params.Rand())
	if err != nil {
		return nil, errorspkg.Wrapf(err, "NewZKProof(gamma, bigGamma)")
	}
	temp.thetaInverse = thetaInverse

	r4msg := NewSignRound4Message(params.PartyID(), temp.deCommit, piGamma)
	temp.signRound4Messages[i] = r4msg
	return &SignRoundOutput{Messages: []tss.Message{r4msg}}, nil
}

// SignRound5 verifies commitments, computes R, and produces blinding.
func SignRound5(state *SigningState, r4bcast []tss.ParsedMessage) (*SignRoundOutput, error) {
	params, temp := state.params, state.temp
	tss.MergeMsgs(temp.signRound4Messages, r4bcast)

	i := params.PartyID().Index
	R := temp.pointGamma
	for j, Pj := range params.Parties().IDs() {
		if j == i {
			continue
		}
		ContextJ := common.AppendBigIntToBytesSlice(temp.ssid, big.NewInt(int64(j)))
		r1msg2 := temp.signRound1Message2s[j].Content().(*SignRound1Message2)
		r4msg := r4bcast[j].Content().(*SignRound4Message)
		SCj, SDj := r1msg2.UnmarshalCommitment(), r4msg.UnmarshalDeCommitment()
		cmtDeCmt := commitments.HashCommitDecommit{C: SCj, D: SDj}
		ok, bigGammaJ := cmtDeCmt.DeCommit()
		if !ok || len(bigGammaJ) != 2 {
			return nil, tss.NewError(errors.New("commitment verify failed"), TaskName, 5, params.PartyID(), Pj)
		}
		bigGammaJPoint, err := crypto.NewECPoint(params.EC(), bigGammaJ[0], bigGammaJ[1])
		if err != nil {
			return nil, tss.NewError(err, TaskName, 5, params.PartyID(), Pj)
		}
		proof, err := r4msg.UnmarshalZKProof(params.EC())
		if err != nil {
			return nil, tss.NewError(errors.New("unmarshal bigGamma proof failed"), TaskName, 5, params.PartyID(), Pj)
		}
		if ok = proof.Verify(ContextJ, bigGammaJPoint); !ok {
			return nil, tss.NewError(errors.New("bigGamma proof verify failed"), TaskName, 5, params.PartyID(), Pj)
		}
		var err2 error
		R, err2 = R.Add(bigGammaJPoint)
		if err2 != nil {
			return nil, tss.NewError(err2, TaskName, 5, params.PartyID(), Pj)
		}
	}

	if R.IsIdentity() {
		return nil, errors.New("sum of gamma points is identity")
	}
	R = R.ScalarMult(temp.thetaInverse)
	if R.IsIdentity() {
		return nil, errors.New("R is identity after theta-inverse")
	}

	N := params.EC().Params().N
	modN := common.ModInt(N)
	rx, ry := R.X(), R.Y()
	if new(big.Int).Mod(rx, N).Sign() == 0 {
		return nil, errors.New("r component is zero")
	}

	si := modN.Add(modN.Mul(temp.m, temp.k), modN.Mul(rx, temp.sigma))
	if si.Sign() == 0 {
		return nil, errors.New("si is zero")
	}

	// Clear secrets
	temp.w = new(big.Int)
	temp.k = new(big.Int)
	temp.gamma = new(big.Int)
	temp.sigma = new(big.Int)

	li := common.GetRandomPositiveInt(params.Rand(), N)
	roI := common.GetRandomPositiveInt(params.Rand(), N)
	rToSi := R.ScalarMult(si)
	liPoint := crypto.ScalarBaseMult(params.EC(), li)
	bigAi := crypto.ScalarBaseMult(params.EC(), roI)
	bigVi, err := rToSi.Add(liPoint)
	if err != nil {
		return nil, fmt.Errorf("round 5 compute bigVi: %w", err)
	}

	cmt := commitments.NewHashCommitment(params.Rand(), bigVi.X(), bigVi.Y(), bigAi.X(), bigAi.Y())
	r5msg := NewSignRound5Message(params.PartyID(), cmt.C)
	temp.signRound5Messages[i] = r5msg
	temp.li = li
	temp.bigAi = bigAi
	temp.bigVi = bigVi
	temp.roi = roI
	temp.DPower = cmt.D
	temp.si = si
	temp.rx = rx
	temp.ry = ry
	temp.bigR = R

	return &SignRoundOutput{Messages: []tss.Message{r5msg}}, nil
}

// SignRound6 produces Schnorr proofs for the blinding values.
func SignRound6(state *SigningState) (*SignRoundOutput, error) {
	params, temp := state.params, state.temp
	i := params.PartyID().Index
	ContextI := common.AppendBigIntToBytesSlice(temp.ssid, new(big.Int).SetUint64(uint64(i)))

	piAi, err := schnorr.NewZKProof(ContextI, temp.roi, temp.bigAi, params.Rand())
	if err != nil {
		return nil, errorspkg.Wrapf(err, "NewZKProof(roi, bigAi)")
	}
	piV, err := schnorr.NewZKVProof(ContextI, temp.bigVi, temp.bigR, temp.si, temp.li, params.Rand())
	if err != nil {
		return nil, errorspkg.Wrapf(err, "NewZKVProof")
	}

	r6msg := NewSignRound6Message(params.PartyID(), temp.DPower, piAi, piV)
	temp.signRound6Messages[i] = r6msg
	return &SignRoundOutput{Messages: []tss.Message{r6msg}}, nil
}

// SignRound7 verifies blinding proofs, computes Ui/Ti, and commits.
func SignRound7(state *SigningState, r5bcast, r6bcast []tss.ParsedMessage) (*SignRoundOutput, error) {
	params, key, temp := state.params, state.key, state.temp
	tss.MergeMsgs(temp.signRound5Messages, r5bcast)
	tss.MergeMsgs(temp.signRound6Messages, r6bcast)

	i := params.PartyID().Index
	bigVjs := make([]*crypto.ECPoint, len(params.Parties().IDs()))
	bigAjs := make([]*crypto.ECPoint, len(params.Parties().IDs()))
	for j, Pj := range params.Parties().IDs() {
		if j == i {
			continue
		}
		ContextJ := common.AppendBigIntToBytesSlice(temp.ssid, big.NewInt(int64(j)))
		r5msg := r5bcast[j].Content().(*SignRound5Message)
		r6msg := r6bcast[j].Content().(*SignRound6Message)
		cj, dj := r5msg.UnmarshalCommitment(), r6msg.UnmarshalDeCommitment()
		cmtDeCmt := commitments.HashCommitDecommit{C: cj, D: dj}
		ok, values := cmtDeCmt.DeCommit()
		if !ok || len(values) != 4 {
			return nil, tss.NewError(errors.New("de-commitment failed"), TaskName, 7, params.PartyID(), Pj)
		}
		bigVj, err := crypto.NewECPoint(params.EC(), values[0], values[1])
		if err != nil {
			return nil, tss.NewError(err, TaskName, 7, params.PartyID(), Pj)
		}
		if bigVj.IsIdentity() {
			return nil, tss.NewError(errors.New("bigVj is identity"), TaskName, 7, params.PartyID(), Pj)
		}
		bigVjs[j] = bigVj
		bigAj, err := crypto.NewECPoint(params.EC(), values[2], values[3])
		if err != nil {
			return nil, tss.NewError(err, TaskName, 7, params.PartyID(), Pj)
		}
		if bigAj.IsIdentity() {
			return nil, tss.NewError(errors.New("bigAj is identity"), TaskName, 7, params.PartyID(), Pj)
		}
		bigAjs[j] = bigAj
		pijA, err := r6msg.UnmarshalZKProof(params.EC())
		if err != nil || !pijA.Verify(ContextJ, bigAj) {
			return nil, tss.NewError(errors.New("schnorr Aj verify failed"), TaskName, 7, params.PartyID(), Pj)
		}
		pijV, err := r6msg.UnmarshalZKVProof(params.EC())
		if err != nil || !pijV.Verify(ContextJ, bigVj, temp.bigR) {
			return nil, tss.NewError(errors.New("vverify Vj failed"), TaskName, 7, params.PartyID(), Pj)
		}
	}

	modN := common.ModInt(params.EC().Params().N)
	AX, AY := temp.bigAi.X(), temp.bigAi.Y()
	minusM := modN.Sub(big.NewInt(0), temp.m)
	gToMInvX, gToMInvY := params.EC().ScalarBaseMult(minusM.Bytes())
	minusR := modN.Sub(big.NewInt(0), temp.rx)
	yToRInvX, yToRInvY := params.EC().ScalarMult(key.ECDSAPub.X(), key.ECDSAPub.Y(), minusR.Bytes())
	VX, VY := params.EC().Add(gToMInvX, gToMInvY, yToRInvX, yToRInvY)
	VX, VY = params.EC().Add(VX, VY, temp.bigVi.X(), temp.bigVi.Y())
	for j := range params.Parties().IDs() {
		if j == i {
			continue
		}
		VX, VY = params.EC().Add(VX, VY, bigVjs[j].X(), bigVjs[j].Y())
		AX, AY = params.EC().Add(AX, AY, bigAjs[j].X(), bigAjs[j].Y())
	}

	UiX, UiY := params.EC().ScalarMult(VX, VY, temp.roi.Bytes())
	TiX, TiY := params.EC().ScalarMult(AX, AY, temp.li.Bytes())
	Ui, err := crypto.NewECPoint(params.EC(), UiX, UiY)
	if err != nil {
		return nil, fmt.Errorf("round 7 compute Ui: %w", err)
	}
	Ti, err := crypto.NewECPoint(params.EC(), TiX, TiY)
	if err != nil {
		return nil, fmt.Errorf("round 7 compute Ti: %w", err)
	}
	temp.Ui = Ui
	temp.Ti = Ti
	cmt := commitments.NewHashCommitment(params.Rand(), UiX, UiY, TiX, TiY)
	r7msg := NewSignRound7Message(params.PartyID(), cmt.C)
	temp.signRound7Messages[i] = r7msg
	temp.DTelda = cmt.D
	return &SignRoundOutput{Messages: []tss.Message{r7msg}}, nil
}

// SignRound8 decommits Ui/Ti.
func SignRound8(state *SigningState) (*SignRoundOutput, error) {
	params, temp := state.params, state.temp
	i := params.PartyID().Index
	r8msg := NewSignRound8Message(params.PartyID(), temp.DTelda)
	temp.signRound8Messages[i] = r8msg
	return &SignRoundOutput{Messages: []tss.Message{r8msg}}, nil
}

// SignRound9 verifies Ui==Ti consistency and reveals si.
func SignRound9(state *SigningState, r7bcast, r8bcast []tss.ParsedMessage) (*SignRoundOutput, error) {
	params, temp := state.params, state.temp
	tss.MergeMsgs(temp.signRound7Messages, r7bcast)
	tss.MergeMsgs(temp.signRound8Messages, r8bcast)

	i := params.PartyID().Index
	UX, UY := temp.Ui.X(), temp.Ui.Y()
	TX, TY := temp.Ti.X(), temp.Ti.Y()
	for j, Pj := range params.Parties().IDs() {
		if j == i {
			continue
		}
		r7msg := r7bcast[j].Content().(*SignRound7Message)
		r8msg := r8bcast[j].Content().(*SignRound8Message)
		cj, dj := r7msg.UnmarshalCommitment(), r8msg.UnmarshalDeCommitment()
		cmt := commitments.HashCommitDecommit{C: cj, D: dj}
		ok, values := cmt.DeCommit()
		if !ok || len(values) != 4 {
			return nil, tss.NewError(errors.New("Uj/Tj decommit failed"), TaskName, 9, params.PartyID(), Pj)
		}
		Uj, err := crypto.NewECPoint(params.EC(), values[0], values[1])
		if err != nil {
			return nil, tss.NewError(err, TaskName, 9, params.PartyID(), Pj)
		}
		if Uj.IsIdentity() {
			return nil, tss.NewError(errors.New("Uj is identity"), TaskName, 9, params.PartyID(), Pj)
		}
		Tj, err := crypto.NewECPoint(params.EC(), values[2], values[3])
		if err != nil {
			return nil, tss.NewError(err, TaskName, 9, params.PartyID(), Pj)
		}
		if Tj.IsIdentity() {
			return nil, tss.NewError(errors.New("Tj is identity"), TaskName, 9, params.PartyID(), Pj)
		}
		UX, UY = params.EC().Add(UX, UY, values[0], values[1])
		TX, TY = params.EC().Add(TX, TY, values[2], values[3])
	}
	if UX.Cmp(TX) != 0 || UY.Cmp(TY) != 0 {
		return nil, errors.New("U != T: signature share inconsistency")
	}

	r9msg := NewSignRound9Message(params.PartyID(), temp.si)
	temp.signRound9Messages[i] = r9msg
	return &SignRoundOutput{Messages: []tss.Message{r9msg}}, nil
}

// SignFinalize collects partial signatures, aggregates S, normalizes,
// and verifies the final ECDSA signature.
func SignFinalize(state *SigningState, r9bcast []tss.ParsedMessage) (*SignRoundOutput, error) {
	params, key, temp, data := state.params, state.key, state.temp, state.data
	tss.MergeMsgs(temp.signRound9Messages, r9bcast)

	sumS := new(big.Int).Set(temp.si)
	modN := common.ModInt(params.EC().Params().N)
	N := params.EC().Params().N
	i := params.PartyID().Index

	for j := range params.Parties().IDs() {
		if j == i {
			continue
		}
		r9msg := r9bcast[j].Content().(*SignRound9Message)
		sj := r9msg.UnmarshalS()
		if sj.Sign() < 0 || sj.Cmp(N) >= 0 {
			return nil, fmt.Errorf("party %d sent s_i outside [0, N)", j)
		}
		sumS = modN.Add(sumS, sj)
	}
	if sumS.Sign() == 0 {
		return nil, errors.New("accumulated S is zero")
	}

	recid := 0
	if temp.rx.Cmp(N) > 0 {
		recid = 2
	}
	if temp.ry.Bit(0) != 0 {
		recid |= 1
	}

	// Low-S normalization
	secp256k1halfN := new(big.Int).Rsh(N, 1)
	if sumS.Cmp(secp256k1halfN) > 0 {
		sumS.Sub(N, sumS)
		recid ^= 1
	}

	bitSizeInBytes := (params.EC().Params().BitSize + 7) / 8
	data.R = padToLengthBytesInPlace(temp.rx.Bytes(), bitSizeInBytes)
	data.S = padToLengthBytesInPlace(sumS.Bytes(), bitSizeInBytes)
	data.Signature = append(data.R, data.S...)
	data.SignatureRecovery = []byte{byte(recid)}
	if temp.fullBytesLen == 0 {
		data.M = temp.m.Bytes()
	} else {
		mBytes := make([]byte, temp.fullBytesLen)
		temp.m.FillBytes(mBytes)
		data.M = mBytes
	}

	pk := ecdsa.PublicKey{
		Curve: params.EC(),
		X:     key.ECDSAPub.X(),
		Y:     key.ECDSAPub.Y(),
	}
	if ok := ecdsa.Verify(&pk, data.M, temp.rx, sumS); !ok {
		return nil, errors.New("signature verification failed")
	}

	return &SignRoundOutput{Signature: data}, nil
}
