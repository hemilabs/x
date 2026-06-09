// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/hashicorp/go-multierror"

	"github.com/hemilabs/x/tss/v3/common"
	"github.com/hemilabs/x/tss/v3/crypto"
	cmts "github.com/hemilabs/x/tss/v3/crypto/commitments"
	"github.com/hemilabs/x/tss/v3/crypto/dlnproof"
	"github.com/hemilabs/x/tss/v3/crypto/facproof"
	"github.com/hemilabs/x/tss/v3/crypto/modproof"
	"github.com/hemilabs/x/tss/v3/crypto/paillier"
	"github.com/hemilabs/x/tss/v3/crypto/vss"
	"github.com/hemilabs/x/tss/v3/tss"
)

// getSSID computes the SSID for a given round number using the state's
// params and temp data.  Same hash as base.getSSID but without the
// round-method receiver.
func getSSID(params *tss.Parameters, temp *localTempData, roundNumber int) ([]byte, error) {
	ssidList := []*big.Int{
		new(big.Int).SetBytes([]byte("ecdsa-keygen")),
		params.EC().Params().P, params.EC().Params().N,
		params.EC().Params().B, params.EC().Params().Gx,
		params.EC().Params().Gy,
	}
	ssidList = append(ssidList, params.Parties().IDs().Keys()...)
	ssidList = append(ssidList, big.NewInt(int64(params.PartyCount())))
	ssidList = append(ssidList, big.NewInt(int64(params.Threshold())))
	ssidList = append(ssidList, big.NewInt(int64(roundNumber)))
	ssidList = append(ssidList, temp.ssidNonce)
	if cid := params.CeremonyID(); len(cid) > 0 {
		ssidList = append(ssidList, new(big.Int).SetBytes(cid))
	}
	ssid := common.SHA512_256i(ssidList...).Bytes()
	return ssid, nil
}

// Round1 initializes keygen state, generates the VSS shares and
// commitment, and produces the round 1 broadcast message.
//
// ctx is used when preParams are absent and safe primes must be
// generated (slow — several seconds).  When valid preParams are
// provided, ctx is unused.
//
// preParams must be pre-generated and validated via
// GeneratePreParams / ValidateWithProof.  If preParams is zero-value
// (Validate() returns false), Round1 generates fresh safe primes.
func Round1(ctx context.Context, params *tss.Parameters, preParams LocalPreParams) (*KeygenState, *RoundOutput, error) {
	partyCount := params.PartyCount()
	save := NewLocalPartySaveData(partyCount)
	temp := &localTempData{
		localMessageStore: localMessageStore{
			kgRound1Messages:  make([]*tss.Message, partyCount),
			kgRound2Message1s: make([]*tss.Message, partyCount),
			kgRound2Message2s: make([]*tss.Message, partyCount),
			kgRound3Messages:  make([]*tss.Message, partyCount),
		},
		KGCs: make([]cmts.HashCommitment, partyCount),
	}

	Pi := params.PartyID()
	i := Pi.Index

	// 1. calculate "partial" key share ui
	ui := common.GetRandomPositiveInt(params.PartialKeyRand(), params.EC().Params().N)
	temp.ui = ui

	// 2. compute the vss shares
	ids := params.Parties().IDs().Keys()
	vs, shares, poly, err := vss.Create(params.EC(), params.Threshold(), ui, ids, params.Rand())
	if err != nil {
		return nil, nil, err
	}
	temp.Poly = poly
	save.Ks = ids

	// Clear ui after last use.
	temp.ui = new(big.Int)

	// make commitment -> (C, D)
	pGFlat, err := crypto.FlattenECPoints(vs)
	if err != nil {
		return nil, nil, err
	}
	cmt := cmts.NewHashCommitment(params.Rand(), pGFlat...)

	// Paillier key and safe primes
	var pp *LocalPreParams
	if preParams.Validate() && !preParams.ValidateWithProof() {
		return nil, nil, errors.New("preParams failed validation (may be from older tss-lib version)")
	} else if preParams.ValidateWithProof() {
		pp = &preParams
	} else {
		ctx, cancel := context.WithTimeout(ctx, params.SafePrimeGenTimeout())
		defer cancel()
		pp, err = GeneratePreParamsWithContextAndRandom(ctx, params.Rand(), params.Concurrency())
		if err != nil {
			return nil, nil, errors.New("pre-params generation failed")
		}
	}
	save.LocalPreParams = *pp
	save.NTildej[i] = pp.NTildei
	save.H1j[i], save.H2j[i] = pp.H1i, pp.H2i

	temp.ssidNonce = new(big.Int).SetUint64(uint64(params.SSIDNonce()))
	save.ShareID = ids[i]
	temp.vs = vs
	ssid, err := getSSID(params, temp, 1)
	if err != nil {
		return nil, nil, errors.New("failed to generate ssid")
	}
	temp.ssid = ssid
	temp.shares = shares

	// DLN proofs (gated by NoProofDLN for SNARK mode)
	var dlnProof1, dlnProof2 *dlnproof.Proof
	if !params.NoProofDLN() {
		h1i, h2i, alpha, beta := pp.H1i, pp.H2i, pp.Alpha, pp.Beta
		p, q, NTildei := pp.P, pp.Q, pp.NTildei
		ContextI := common.AppendBigIntToBytesSlice(temp.ssid, big.NewInt(int64(i)))
		dlnProof1 = dlnproof.NewDLNProof(ContextI, h1i, h2i, alpha, p, q, NTildei, params.Rand())
		dlnProof2 = dlnproof.NewDLNProof(ContextI, h2i, h1i, beta, p, q, NTildei, params.Rand())
	}

	save.PaillierSK = pp.PaillierSK
	save.PaillierPKs[i] = &pp.PaillierSK.PublicKey
	temp.deCommitPolyG = cmt.D

	msg := NewKGRound1Message(
		Pi, cmt.C, &pp.PaillierSK.PublicKey,
		pp.NTildei, pp.H1i, pp.H2i,
		dlnProof1, dlnProof2)
	// Store own round 1 message so round 2 can validate uniformly.
	temp.kgRound1Messages[i] = msg

	state := &KeygenState{params: params, save: &save, temp: temp}
	out := &RoundOutput{
		Messages: []*tss.Message{msg},
		Poly:     poly,
	}
	return state, out, nil
}

// Round2 validates round 1 messages (DLN proofs, parameter checks),
// generates VSS shares for each party, and produces P2P + broadcast
// messages.
//
// r1Msgs must be indexed by party: r1Msgs[j] is party j's
// KGRound1Message broadcast.  r1Msgs[i] (own message from Round1)
// must be present.
func Round2(ctx context.Context, state *KeygenState, r1Msgs []*tss.Message) (*RoundOutput, error) {
	params := state.params
	save := state.save
	temp := state.temp

	// Populate temp message store so validation code reads from it.
	tss.MergeMsgs(temp.kgRound1Messages, r1Msgs)

	i := params.PartyID().Index

	dlnVerifier := NewDlnProofVerifier(params.Concurrency())

	// Comprehensive parameter validation battery.
	h1H2Map := make(map[string]struct{}, len(r1Msgs)*2)
	// Single modulus map for both PaillierN and NTilde: catches cross-party
	// collisions (e.g., Party A's PaillierN == Party B's NTilde) that separate
	// maps would miss. Such a collision lets A forge range proofs against B.
	modulusMap := make(map[string]struct{}, len(r1Msgs)*2)
	dlnProof1FailCulprits := make([]*tss.PartyID, len(r1Msgs))
	dlnProof2FailCulprits := make([]*tss.PartyID, len(r1Msgs))
	wg := new(sync.WaitGroup)
	for j, msg := range r1Msgs {
		r1msg := msg.Content.(*KGRound1Message)
		H1j, H2j, NTildej, paillierPKj := r1msg.H1,
			r1msg.H2,
			r1msg.NTilde,
			r1msg.PaillierPK
		if paillierPKj.N.BitLen() != paillierBitsLen {
			return nil, tss.NewError(errors.New("paillier modulus insufficient bits"), TaskName, 2, params.PartyID(), msg.From)
		}
		if paillierPKj.N.Bit(0) == 0 {
			return nil, tss.NewError(errors.New("even paillier modulus"), TaskName, 2, params.PartyID(), msg.From)
		}
		if paillierPKj.N.ProbablyPrime(20) {
			return nil, tss.NewError(errors.New("prime paillier modulus"), TaskName, 2, params.PartyID(), msg.From)
		}
		sqrtN := new(big.Int).Sqrt(paillierPKj.N)
		if new(big.Int).Mul(sqrtN, sqrtN).Cmp(paillierPKj.N) == 0 {
			return nil, tss.NewError(errors.New("perfect-square paillier modulus"), TaskName, 2, params.PartyID(), msg.From)
		}
		if H1j.Cmp(H2j) == 0 {
			return nil, tss.NewError(errors.New("h1j == h2j"), TaskName, 2, params.PartyID(), msg.From)
		}
		if H1j.Cmp(big.NewInt(1)) == 0 || H2j.Cmp(big.NewInt(1)) == 0 {
			return nil, tss.NewError(errors.New("h1j or h2j is 1"), TaskName, 2, params.PartyID(), msg.From)
		}
		if NTildej.BitLen() != paillierBitsLen {
			return nil, tss.NewError(errors.New("NTildej insufficient bits"), TaskName, 2, params.PartyID(), msg.From)
		}
		if NTildej.Bit(0) == 0 {
			return nil, tss.NewError(errors.New("even NTildej"), TaskName, 2, params.PartyID(), msg.From)
		}
		if NTildej.ProbablyPrime(20) {
			return nil, tss.NewError(errors.New("prime NTildej"), TaskName, 2, params.PartyID(), msg.From)
		}
		sqrtNT := new(big.Int).Sqrt(NTildej)
		if new(big.Int).Mul(sqrtNT, sqrtNT).Cmp(NTildej) == 0 {
			return nil, tss.NewError(errors.New("perfect-square NTildej"), TaskName, 2, params.PartyID(), msg.From)
		}
		if paillierPKj.N.Cmp(NTildej) == 0 {
			return nil, tss.NewError(errors.New("paillier N == NTilde"), TaskName, 2, params.PartyID(), msg.From)
		}
		if new(big.Int).GCD(nil, nil, H1j, NTildej).Cmp(big.NewInt(1)) != 0 {
			return nil, tss.NewError(errors.New("h1j not coprime with NTildej"), TaskName, 2, params.PartyID(), msg.From)
		}
		if new(big.Int).GCD(nil, nil, H2j, NTildej).Cmp(big.NewInt(1)) != 0 {
			return nil, tss.NewError(errors.New("h2j not coprime with NTildej"), TaskName, 2, params.PartyID(), msg.From)
		}
		h1JHex, h2JHex := hex.EncodeToString(H1j.Bytes()), hex.EncodeToString(H2j.Bytes())
		if _, found := h1H2Map[h1JHex]; found {
			return nil, tss.NewError(errors.New("duplicate h1j"), TaskName, 2, params.PartyID(), msg.From)
		}
		if _, found := h1H2Map[h2JHex]; found {
			return nil, tss.NewError(errors.New("duplicate h2j"), TaskName, 2, params.PartyID(), msg.From)
		}
		h1H2Map[h1JHex], h1H2Map[h2JHex] = struct{}{}, struct{}{}
		paillierNHex := hex.EncodeToString(paillierPKj.N.Bytes())
		if _, found := modulusMap[paillierNHex]; found {
			return nil, tss.NewError(errors.New("duplicate modulus (Paillier N)"), TaskName, 2, params.PartyID(), msg.From)
		}
		modulusMap[paillierNHex] = struct{}{}
		nTildeHex := hex.EncodeToString(NTildej.Bytes())
		if _, found := modulusMap[nTildeHex]; found {
			return nil, tss.NewError(errors.New("duplicate modulus (NTilde)"), TaskName, 2, params.PartyID(), msg.From)
		}
		modulusMap[nTildeHex] = struct{}{}

		if !params.NoProofDLN() {
			wg.Add(2)
			_j := j
			_msg := msg
			ContextJ := common.AppendBigIntToBytesSlice(temp.ssid, big.NewInt(int64(j)))
			dlnVerifier.VerifyDLNProof(r1msg.DLNProof1, ContextJ, H1j, H2j, NTildej, func(isValid bool) {
				if !isValid {
					dlnProof1FailCulprits[_j] = _msg.From
				}
				wg.Done()
			})
			dlnVerifier.VerifyDLNProof(r1msg.DLNProof2, ContextJ, H2j, H1j, NTildej, func(isValid bool) {
				if !isValid {
					dlnProof2FailCulprits[_j] = _msg.From
				}
				wg.Done()
			})
		}
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, culprits := range [][]*tss.PartyID{dlnProof1FailCulprits, dlnProof2FailCulprits} {
		for _, culprit := range culprits {
			if culprit != nil {
				return nil, tss.NewError(errors.New("dln proof verification failed"), TaskName, 2, params.PartyID(), culprit)
			}
		}
	}

	// Save NTilde_j, h1_j, h2_j, PaillierPKs, KGCs
	for j, msg := range r1Msgs {
		if j == i {
			continue
		}
		r1msg := msg.Content.(*KGRound1Message)
		save.PaillierPKs[j] = r1msg.PaillierPK
		save.NTildej[j] = r1msg.NTilde
		save.H1j[j] = r1msg.H1
		save.H2j[j] = r1msg.H2
		temp.KGCs[j] = r1msg.Commitment
	}

	// P2P send share ij to Pj
	out := &RoundOutput{}
	ContextI := common.AppendBigIntToBytesSlice(temp.ssid, big.NewInt(int64(i)))
	for j, Pj := range params.Parties().IDs() {
		var facProofObj *facproof.ProofFac
		if !params.NoProofFac() {
			var err error
			facProofObj, err = facproof.NewProof(ContextI, params.EC(), save.PaillierSK.N,
				save.NTildej[j], save.H1j[j], save.H2j[j],
				save.PaillierSK.P, save.PaillierSK.Q, params.Rand())
			if err != nil {
				return nil, fmt.Errorf("round 2 fac proof for party %d: %w", j, err)
			}
		}
		r2msg1 := NewKGRound2Message1(Pj, params.PartyID(), temp.shares[j], facProofObj)
		if j == i {
			temp.kgRound2Message1s[j] = r2msg1
			continue
		}
		out.Messages = append(out.Messages, r2msg1)
	}

	// BROADCAST de-commitments
	var modProofObj *modproof.ProofMod
	if !params.NoProofMod() {
		var err error
		modProofObj, err = modproof.NewProof(ContextI, save.PaillierSK.N,
			save.PaillierSK.P, save.PaillierSK.Q, params.Rand())
		if err != nil {
			return nil, fmt.Errorf("round 2 mod proof: %w", err)
		}
	}
	r2msg2 := NewKGRound2Message2(params.PartyID(), temp.deCommitPolyG, modProofObj)
	temp.kgRound2Message2s[i] = r2msg2
	out.Messages = append(out.Messages, r2msg2)

	return out, nil
}

// Round3 validates round 2 messages (de-commitments, VSS shares,
// FacProof, ModProof, ReceiverID), computes the aggregate public
// key and per-party public key shares, and produces the round 3
// Paillier proof broadcast.
//
// r2p2p[j] is party j's KGRound2Message1 (P2P, contains VSS share).
// r2bcast[j] is party j's KGRound2Message2 (broadcast, contains
// de-commitment and ModProof).
func Round3(ctx context.Context, state *KeygenState, r2p2p, r2bcast []*tss.Message) (*RoundOutput, error) {
	params := state.params
	save := state.save
	temp := state.temp
	Ps := params.Parties().IDs()
	PIdx := params.PartyID().Index

	// Store own messages
	tss.MergeMsgs(temp.kgRound2Message1s, r2p2p)
	tss.MergeMsgs(temp.kgRound2Message2s, r2bcast)

	// 1,9. calculate xi
	xi := new(big.Int).Set(temp.shares[PIdx].Share)
	for j := range Ps {
		if j == PIdx {
			continue
		}
		r2msg1 := r2p2p[j].Content.(*KGRound2Message1)
		share := r2msg1.Share
		xi = new(big.Int).Add(xi, share)
	}
	save.Xi = new(big.Int).Mod(xi, params.EC().Params().N)
	if save.Xi.Sign() == 0 {
		return nil, errors.New("xi is zero")
	}

	// 2-3.
	Vc := make(vss.Vs, params.Threshold()+1)
	for c := range Vc {
		Vc[c] = temp.vs[c]
	}

	// 4-11. Concurrent VSS/proof verification
	type vssOut struct {
		unWrappedErr error
		pjVs         vss.Vs
	}
	vssResults := make([]vssOut, len(Ps))
	gctx, gcancel := context.WithCancel(ctx)
	defer gcancel()
	wg := sync.WaitGroup{}
	for j := range Ps {
		if j == PIdx {
			continue
		}
		wg.Add(1)
		ContextJ := common.AppendBigIntToBytesSlice(temp.ssid, big.NewInt(int64(j)))
		go func(j int) {
			defer wg.Done()
			if gctx.Err() != nil {
				return
			}
			KGCj := temp.KGCs[j]
			r2msg2 := r2bcast[j].Content.(*KGRound2Message2)
			KGDj := r2msg2.DeCommitment
			cmtDeCmt := cmts.HashCommitDecommit{C: KGCj, D: KGDj}
			ok, flatPolyGs := cmtDeCmt.DeCommit()
			if !ok || flatPolyGs == nil {
				vssResults[j] = vssOut{errors.New("de-commitment verify failed"), nil}
				gcancel()
				return
			}
			PjVs, err := crypto.UnFlattenECPoints(params.EC(), flatPolyGs)
			if err != nil {
				vssResults[j] = vssOut{err, nil}
				gcancel()
				return
			}
			if gctx.Err() != nil {
				return
			}
			if !params.NoProofMod() {
				if r2msg2.ModProof == nil {
					vssResults[j] = vssOut{errors.New("modProof missing"), nil}
					gcancel()
					return
				}
				if ok = r2msg2.ModProof.Verify(ContextJ, save.PaillierPKs[j].N); !ok {
					vssResults[j] = vssOut{errors.New("modProof verify failed"), nil}
					gcancel()
					return
				}
			}
			if gctx.Err() != nil {
				return
			}
			r2msg1 := r2p2p[j].Content.(*KGRound2Message1)
			myKey := params.PartyID().KeyInt().Bytes()
			if !bytes.Equal(r2msg1.ReceiverID, myKey) {
				vssResults[j] = vssOut{errors.New("receiverId mismatch"), nil}
				gcancel()
				return
			}
			PjShare := vss.Share{
				Threshold: params.Threshold(),
				ID:        params.PartyID().KeyInt(),
				Share:     r2msg1.Share,
			}
			if ok = PjShare.Verify(params.EC(), params.Threshold(), PjVs); !ok {
				vssResults[j] = vssOut{errors.New("vss verify failed"), nil}
				gcancel()
				return
			}
			if gctx.Err() != nil {
				return
			}
			if !params.NoProofFac() {
				if r2msg1.FacProof == nil {
					vssResults[j] = vssOut{errors.New("facProof missing"), nil}
					gcancel()
					return
				}
				if ok = r2msg1.FacProof.Verify(ContextJ, params.EC(), save.PaillierPKs[j].N,
					save.NTildei, save.H1i, save.H2i); !ok {
					vssResults[j] = vssOut{errors.New("facProof verify failed"), nil}
					gcancel()
					return
				}
			}
			vssResults[j] = vssOut{nil, PjVs}
		}(j)
	}
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Collect results
	{
		culprits := make([]*tss.PartyID, 0, len(Ps))
		for j, Pj := range Ps {
			if j == PIdx {
				continue
			}
			if err := vssResults[j].unWrappedErr; err != nil {
				culprits = append(culprits, Pj)
			}
		}
		if len(culprits) > 0 {
			var multiErr error
			for _, vssResult := range vssResults {
				if vssResult.unWrappedErr != nil {
					multiErr = multierror.Append(multiErr, vssResult.unWrappedErr)
				}
			}
			return nil, tss.NewError(multiErr, TaskName, 3, params.PartyID(), culprits...)
		}
	}
	{
		var err error
		culprits := make([]*tss.PartyID, 0, len(Ps))
		for j, Pj := range Ps {
			if j == PIdx {
				continue
			}
			PjVs := vssResults[j].pjVs
			for c := 0; c <= params.Threshold(); c++ {
				Vc[c], err = Vc[c].Add(PjVs[c])
				if err != nil {
					culprits = append(culprits, Pj)
				}
			}
		}
		if len(culprits) > 0 {
			return nil, tss.NewError(errors.New("vc point addition failed"), TaskName, 3, params.PartyID(), culprits...)
		}
	}

	// 12-16. compute Xj for each Pj
	{
		var err error
		modQ := common.ModInt(params.EC().Params().N)
		culprits := make([]*tss.PartyID, 0, len(Ps))
		bigXj := save.BigXj
		for j := 0; j < params.PartyCount(); j++ {
			Pj := Ps[j]
			kj := Pj.KeyInt()
			BigXj := Vc[0]
			z := new(big.Int).SetInt64(1)
			for c := 1; c <= params.Threshold(); c++ {
				z = modQ.Mul(z, kj)
				BigXj, err = BigXj.Add(Vc[c].ScalarMult(z))
				if err != nil {
					culprits = append(culprits, Pj)
				}
			}
			if BigXj.IsIdentity() {
				culprits = append(culprits, Pj)
			} else {
				bigXj[j] = BigXj
			}
		}
		if len(culprits) > 0 {
			return nil, tss.NewError(errors.New("BigXj identity or computation error"), TaskName, 3, params.PartyID(), culprits...)
		}
		save.BigXj = bigXj
	}

	// 17. compute and SAVE the ECDSA public key
	ecdsaPubKey, err := crypto.NewECPoint(params.EC(), Vc[0].X(), Vc[0].Y())
	if err != nil {
		return nil, fmt.Errorf("round 4 ecdsa pubkey: %w", err)
	}
	if ecdsaPubKey.IsIdentity() {
		return nil, errors.New("public key is the identity point")
	}
	save.ECDSAPub = ecdsaPubKey

	// BROADCAST paillier proof
	ki := params.PartyID().KeyInt()
	proof := save.PaillierSK.Proof(ki, ecdsaPubKey)
	r3msg := NewKGRound3Message(params.PartyID(), proof)

	return &RoundOutput{Messages: []*tss.Message{r3msg}}, nil
}

// Round4 verifies round 3 Paillier proofs and returns the final
// key share data.
//
// r3Msgs[j] is party j's KGRound3Message broadcast containing the
// Paillier proof.
func Round4(ctx context.Context, state *KeygenState, r3Msgs []*tss.Message) (*RoundOutput, error) {
	params := state.params
	save := state.save

	i := params.PartyID().Index
	Ps := params.Parties().IDs()
	PIDs := Ps.Keys()
	ecdsaPub := save.ECDSAPub

	// Concurrent Paillier proof verification
	ok := make([]bool, len(Ps))
	ok[i] = true // self
	gctx, gcancel := context.WithCancel(ctx)
	defer gcancel()
	wg := sync.WaitGroup{}
	for j, msg := range r3Msgs {
		if j == i {
			continue
		}
		wg.Add(1)
		r3msg := msg.Content.(*KGRound3Message)
		go func(prf paillier.Proof, j int) {
			defer wg.Done()
			if gctx.Err() != nil {
				return
			}
			ppk := save.PaillierPKs[j]
			verified, err := prf.Verify(ppk.N, PIDs[j], ecdsaPub)
			if err != nil {
				common.Logger.Error(err)
				gcancel()
				return
			}
			ok[j] = verified
			if !verified {
				gcancel()
			}
		}(r3msg.PaillierProof, j)
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	culprits := make([]*tss.PartyID, 0, len(Ps))
	for j, v := range ok {
		if !v {
			culprits = append(culprits, Ps[j])
		}
	}
	if len(culprits) > 0 {
		return nil, tss.NewError(errors.New("paillier verify failed"), TaskName, 4, params.PartyID(), culprits...)
	}

	return &RoundOutput{Save: save}, nil
}
