// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package signing

import (
	"crypto/sha512"
	"errors"
	"fmt"
	"math/big"

	decredEdwards "github.com/decred/dcrd/dcrec/edwards/v2"

	"github.com/hemilabs/x/tss/v3/common"
	"github.com/hemilabs/x/tss/v3/crypto"
	"github.com/hemilabs/x/tss/v3/crypto/commitments"
	"github.com/hemilabs/x/tss/v3/crypto/schnorr"
	"github.com/hemilabs/x/tss/v3/eddsa/keygen"
	"github.com/hemilabs/x/tss/v3/tss"
)

// getSigningSSID computes the session ID for domain separation.
func getSigningSSID(params *tss.Parameters, key *keygen.LocalPartySaveData, temp *localTempData, roundNumber int) ([]byte, error) {
	ssidList := []*big.Int{
		new(big.Int).SetBytes([]byte("eddsa-signing")),
		params.EC().Params().P, params.EC().Params().N,
		params.EC().Params().B,
		params.EC().Params().Gx, params.EC().Params().Gy,
	}
	ssidList = append(ssidList, params.Parties().IDs().Keys()...)
	ssidList = append(ssidList, big.NewInt(int64(params.PartyCount())))
	ssidList = append(ssidList, big.NewInt(int64(params.Threshold())))
	ssidList = append(ssidList, big.NewInt(int64(roundNumber)))
	ssidList = append(ssidList, temp.ssidNonce)
	if cid := params.CeremonyID(); len(cid) > 0 {
		ssidList = append(ssidList, new(big.Int).SetBytes(cid))
	}
	return common.SHA512_256i(ssidList...).Bytes(), nil
}

// SignRound1 initializes signing state and broadcasts a commitment
// to the signing nonce Ri.
func SignRound1(params *tss.Parameters, key keygen.LocalPartySaveData, msg *big.Int, fullBytesLen int) (*SigningState, *SignRoundOutput, error) {
	if key.Xi == nil || key.Xi.Sign() == 0 {
		return nil, nil, errors.New("invalid key data: Xi is nil or zero")
	}
	if key.EDDSAPub == nil || !key.EDDSAPub.ValidateBasic() {
		return nil, nil, errors.New("invalid key data: EDDSAPub is nil or not on curve")
	}

	n := params.PartyCount()
	i := params.PartyID().Index

	temp := &localTempData{
		localMessageStore: localMessageStore{
			signRound1Messages: make([]*tss.Message, n),
			signRound2Messages: make([]*tss.Message, n),
			signRound3Messages: make([]*tss.Message, n),
		},
		cjs:          make([]*big.Int, n),
		m:            msg,
		fullBytesLen: fullBytesLen,
	}

	temp.ssidNonce = new(big.Int).SetUint64(uint64(params.SSIDNonce()))
	ssid, err := getSigningSSID(params, &key, temp, 1)
	if err != nil {
		return nil, nil, fmt.Errorf("round 1 SSID: %w", err)
	}
	temp.ssid = ssid

	// Compute Lagrange-interpolated secret share wi.
	if len(key.Ks) != n {
		return nil, nil, fmt.Errorf("key count %d does not match party count %d", len(key.Ks), n)
	}
	wi := PrepareForSigning(params.EC(), i, n, key.Xi, key.Ks)
	temp.wi = wi

	// Select signing nonce ri.
	ri := common.GetRandomPositiveInt(params.Rand(), params.EC().Params().N)
	temp.ri = ri

	// Compute Ri = ri*G.
	pointRi := crypto.ScalarBaseMult(params.EC(), ri)
	temp.pointRi = pointRi

	// Commitment.
	cmt := commitments.NewHashCommitment(params.Rand(), pointRi.X(), pointRi.Y())
	temp.deCommit = cmt.D

	r1msg := NewSignRound1Message(params.PartyID(), cmt.C)
	temp.signRound1Messages[i] = r1msg

	state := &SigningState{
		params: params,
		key:    &key,
		data:   &SignatureData{},
		temp:   *temp,
	}
	return state, &SignRoundOutput{Messages: []*tss.Message{r1msg}}, nil
}

// SignRound2 stores round 1 commitments and broadcasts the
// decommitment + Schnorr proof.
func SignRound2(state *SigningState, r1Msgs []*tss.Message) (*SignRoundOutput, error) {
	params := state.params
	temp := &state.temp
	i := params.PartyID().Index

	// Store commitments.
	for j, msg := range r1Msgs {
		r1msg := msg.Content.(*SignRound1Message)
		if !r1msg.ValidateBasic() {
			return nil, tss.NewError(errors.New("invalid round 1 message"), TaskName, 2, params.PartyID(), msg.From)
		}
		temp.cjs[j] = r1msg.Commitment
	}

	// Schnorr proof for ri.
	ContextI := common.AppendBigIntToBytesSlice(temp.ssid, new(big.Int).SetInt64(int64(i)))
	pointRi := temp.pointRi.(*crypto.ECPoint)
	pir, err := schnorr.NewZKProof(ContextI, temp.ri, pointRi, params.Rand())
	if err != nil {
		return nil, fmt.Errorf("round 2 schnorr proof: %w", err)
	}

	r2msg := NewSignRound2Message(params.PartyID(), temp.deCommit, pir)
	temp.signRound2Messages[i] = r2msg

	return &SignRoundOutput{Messages: []*tss.Message{r2msg}}, nil
}

// SignRound3 verifies all decommitments and Schnorr proofs,
// computes the aggregate nonce R, and produces the partial
// signature si.
func SignRound3(state *SigningState, r2Msgs []*tss.Message) (*SignRoundOutput, error) {
	params := state.params
	temp := &state.temp
	i := params.PartyID().Index
	ec := params.EC()
	N := ec.Params().N

	// Init R with own Ri = ri·G.
	Rx, Ry := ec.ScalarBaseMult(temp.ri.Bytes())

	// Verify each party's decommitment + proof, accumulate R.
	for j, Pj := range params.Parties().IDs() {
		if j == i {
			continue
		}
		ContextJ := common.AppendBigIntToBytesSlice(temp.ssid, big.NewInt(int64(j)))
		r2msg := r2Msgs[j].Content.(*SignRound2Message)

		cmtDeCmt := commitments.HashCommitDecommit{C: temp.cjs[j], D: r2msg.DeCommitment}
		ok, coordinates := cmtDeCmt.DeCommit()
		if !ok || len(coordinates) != 2 {
			return nil, tss.NewError(errors.New("de-commitment verify failed"), TaskName, 3, params.PartyID(), Pj)
		}

		Rj, err := crypto.NewECPoint(ec, coordinates[0], coordinates[1])
		if err != nil {
			return nil, tss.NewError(fmt.Errorf("NewECPoint(Rj): %w", err), TaskName, 3, params.PartyID(), Pj)
		}
		Rj = Rj.EightInvEight()

		if r2msg.ZKProof == nil {
			return nil, tss.NewError(errors.New("missing schnorr proof"), TaskName, 3, params.PartyID(), Pj)
		}
		if !r2msg.ZKProof.Verify(ContextJ, Rj) {
			return nil, tss.NewError(errors.New("schnorr proof verify failed"), TaskName, 3, params.PartyID(), Pj)
		}

		Rx, Ry = ec.Add(Rx, Ry, Rj.X(), Rj.Y())
	}

	// Encode R in Ed25519 compressed form.
	encodedR := ecPointToEncodedBytes(Rx, Ry)

	// R identity check: the identity encodes as (0, 1) → LE bytes
	// [0x01, 0x00, ...].
	Rpoint, err := crypto.NewECPoint(ec, Rx, Ry)
	if err != nil || Rpoint.IsIdentity() {
		return nil, tss.NewError(errors.New("r is the identity point"), TaskName, 3, params.PartyID())
	}

	encodedPubKey := ecPointToEncodedBytes(state.key.EDDSAPub.X(), state.key.EDDSAPub.Y())

	h := sha512.New()
	h.Write(encodedR[:])
	h.Write(encodedPubKey[:])
	if temp.fullBytesLen == 0 {
		h.Write(temp.m.Bytes())
	} else {
		mBytes := make([]byte, temp.fullBytesLen)
		temp.m.FillBytes(mBytes)
		h.Write(mBytes)
	}

	var lambda [64]byte
	h.Sum(lambda[:0])
	lambdaReduced := scReduce(&lambda, N)

	// Compute si = lambda*wi + ri mod N.
	localS := scMulAdd(lambdaReduced, temp.wi, temp.ri, N)

	// Clear signing nonces.
	temp.ri = new(big.Int)
	temp.wi = new(big.Int)

	temp.si = localS
	temp.r = encodedBytesToBigInt(encodedR)

	r3msg := NewSignRound3Message(params.PartyID(), localS)
	temp.signRound3Messages[i] = r3msg

	return &SignRoundOutput{Messages: []*tss.Message{r3msg}}, nil
}

// SignFinalize sums partial signatures and verifies the EdDSA signature.
func SignFinalize(state *SigningState, r3Msgs []*tss.Message) (*SignRoundOutput, error) {
	params := state.params
	temp := &state.temp
	i := params.PartyID().Index

	if temp.si == nil {
		return nil, fmt.Errorf("si is nil: round 3 did not complete")
	}
	sumS := temp.si
	N := params.EC().Params().N

	for j := range params.Parties().IDs() {
		if j == i {
			continue
		}
		r3msg := r3Msgs[j].Content.(*SignRound3Message)
		sj := r3msg.S
		if sj.Sign() < 0 || sj.Cmp(N) >= 0 {
			return nil, tss.NewError(
				fmt.Errorf("party %d sent s_i outside [0, N)", j),
				TaskName, 4, params.PartyID(), params.Parties().IDs()[j])
		}
		sumS = new(big.Int).Mod(new(big.Int).Add(sumS, sj), N)
	}

	if sumS.Sign() == 0 {
		return nil, fmt.Errorf("accumulated S is zero: malicious share detected")
	}

	// Build signature data.
	data := state.data
	encodedSumS := bigIntToEncodedBytes(sumS)
	data.Signature = append(bigIntToEncodedBytes(temp.r)[:], encodedSumS[:]...)
	data.R = temp.r.Bytes()
	data.S = sumS.Bytes()
	if temp.fullBytesLen == 0 {
		data.M = temp.m.Bytes()
	} else {
		mBytes := make([]byte, temp.fullBytesLen)
		temp.m.FillBytes(mBytes)
		data.M = mBytes
	}

	// Verify.
	pk := decredEdwards.PublicKey{
		Curve: params.EC(),
		X:     state.key.EDDSAPub.X(),
		Y:     state.key.EDDSAPub.Y(),
	}
	if !decredEdwards.Verify(&pk, data.M, temp.r, sumS) {
		return nil, fmt.Errorf("signature verification failed")
	}

	return &SignRoundOutput{Signature: data}, nil
}
