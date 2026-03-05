// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package resharing

import (
	"bytes"
	"encoding/hex"
	"errors"
	"math/big"
	"sync"

	"github.com/hemilabs/x/tss-lib/v2/crypto/facproof"

	errors2 "github.com/pkg/errors"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/crypto"
	"github.com/hemilabs/x/tss-lib/v2/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v2/crypto/vss"
	"github.com/hemilabs/x/tss-lib/v2/ecdsa/keygen"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

var (
	one             = big.NewInt(1)
	paillierBitsLen = 2048
)

func (round *round4) Start() *tss.Error {
	if round.started {
		return round.WrapError(errors.New("round already started"))
	}
	round.number = 4
	round.started = true
	round.resetOK() // resets both round.oldOK and round.newOK

	round.allOldOK()

	if !round.ReSharingParams().IsNewCommittee() {
		// both committees proceed to round 5 after receiving "ACK" messages from the new committee
		return nil
	}

	common.Logger.Debugf(
		"%s Setting up DLN verification with concurrency level of %d",
		round.PartyID(),
		round.Concurrency(),
	)
	dlnVerifier := keygen.NewDlnProofVerifier(round.Concurrency())

	Pi := round.PartyID()
	i := Pi.Index
	round.newOK[i] = true

	// [FORK] Comprehensive parameter validation battery (resharing equivalent of keygen round 2).
	// Upstream verified DLN proofs, ModProof, H1==H2, and h1/h2 cross-party uniqueness.
	// We add structural checks on Paillier N and NTilde (oddness, non-prime,
	// non-perfect-square), Pedersen parameter sanity (H1/H2 not 1, coprime with NTilde),
	// N != NTilde, and cross-party uniqueness for Paillier N and NTilde.
	// These checks prevent a malicious new committee member from using degenerate parameters
	// that would break the security of ZK proofs used in future signing ceremonies.
	h1H2Map := make(map[string]struct{}, len(round.temp.dgRound2Message1s)*2)
	paillierNMap := make(map[string]struct{}, len(round.temp.dgRound2Message1s))
	nTildeMap := make(map[string]struct{}, len(round.temp.dgRound2Message1s))
	paiProofCulprits := make([]*tss.PartyID, len(round.temp.dgRound2Message1s)) // who caused the error(s)
	dlnProof1FailCulprits := make([]*tss.PartyID, len(round.temp.dgRound2Message1s))
	dlnProof2FailCulprits := make([]*tss.PartyID, len(round.temp.dgRound2Message1s))
	wg := new(sync.WaitGroup)
	for j, msg := range round.temp.dgRound2Message1s {
		r2msg1 := msg.Content().(*DGRound2Message1)
		paiPK, NTildej, H1j, H2j := r2msg1.UnmarshalPaillierPK(),
			r2msg1.UnmarshalNTilde(),
			r2msg1.UnmarshalH1(),
			r2msg1.UnmarshalH2()
		if H1j.Cmp(H2j) == 0 {
			return round.WrapError(errors.New("h1j and h2j were equal for this party"), msg.GetFrom())
		}
		if H1j.Cmp(one) == 0 || H2j.Cmp(one) == 0 {
			return round.WrapError(errors.New("h1j or h2j was 1 (degenerate Pedersen parameter)"), msg.GetFrom())
		}
		// NOTE: resharing uses `<` (minimum threshold) while keygen uses `!=` (exact match)
		// because resharing may accept pre-existing parameters from parties with >= 2048-bit keys.
		if paiPK.N.BitLen() < paillierBitsLen {
			return round.WrapError(errors.New("got paillier modulus with insufficient bits for this party"), msg.GetFrom())
		}
		if paiPK.N.Bit(0) == 0 {
			return round.WrapError(errors.New("got even paillier modulus (trivially factorable)"), msg.GetFrom())
		}
		if paiPK.N.ProbablyPrime(20) {
			return round.WrapError(errors.New("got prime paillier modulus (degenerate Paillier)"), msg.GetFrom())
		}
		sqrtN := new(big.Int).Sqrt(paiPK.N)
		if new(big.Int).Mul(sqrtN, sqrtN).Cmp(paiPK.N) == 0 {
			return round.WrapError(errors.New("got perfect-square paillier modulus (trivially factorable)"), msg.GetFrom())
		}
		if NTildej.BitLen() < paillierBitsLen {
			return round.WrapError(errors.New("got NTildej with insufficient bits for this party"), msg.GetFrom())
		}
		if NTildej.Bit(0) == 0 {
			return round.WrapError(errors.New("got even NTildej (trivially factorable)"), msg.GetFrom())
		}
		if NTildej.ProbablyPrime(20) {
			return round.WrapError(errors.New("got prime NTildej (degenerate Pedersen parameters)"), msg.GetFrom())
		}
		sqrtNT := new(big.Int).Sqrt(NTildej)
		if new(big.Int).Mul(sqrtNT, sqrtNT).Cmp(NTildej) == 0 {
			return round.WrapError(errors.New("got perfect-square NTildej (trivially factorable)"), msg.GetFrom())
		}
		if paiPK.N.Cmp(NTildej) == 0 {
			return round.WrapError(errors.New("Paillier N must differ from NTilde"), msg.GetFrom())
		}
		// Pedersen parameters must be coprime with NTilde
		if new(big.Int).GCD(nil, nil, H1j, NTildej).Cmp(one) != 0 {
			return round.WrapError(errors.New("h1j is not coprime with NTildej"), msg.GetFrom())
		}
		if new(big.Int).GCD(nil, nil, H2j, NTildej).Cmp(one) != 0 {
			return round.WrapError(errors.New("h2j is not coprime with NTildej"), msg.GetFrom())
		}
		h1JHex, h2JHex := hex.EncodeToString(H1j.Bytes()), hex.EncodeToString(H2j.Bytes())
		if _, found := h1H2Map[h1JHex]; found {
			return round.WrapError(errors.New("this h1j was already used by another party"), msg.GetFrom())
		}
		if _, found := h1H2Map[h2JHex]; found {
			return round.WrapError(errors.New("this h2j was already used by another party"), msg.GetFrom())
		}
		h1H2Map[h1JHex], h1H2Map[h2JHex] = struct{}{}, struct{}{}
		// Reject duplicate Paillier moduli across parties
		paillierNHex := hex.EncodeToString(paiPK.N.Bytes())
		if _, found := paillierNMap[paillierNHex]; found {
			return round.WrapError(errors.New("this Paillier N was already used by another party"), msg.GetFrom())
		}
		paillierNMap[paillierNHex] = struct{}{}
		// Reject duplicate NTilde across parties
		nTildeHex := hex.EncodeToString(NTildej.Bytes())
		if _, found := nTildeMap[nTildeHex]; found {
			return round.WrapError(errors.New("this NTilde was already used by another party"), msg.GetFrom())
		}
		nTildeMap[nTildeHex] = struct{}{}
		// [FORK] Proof verification gated by NoProofMod() and NoProofDLN(). In SNARK mode,
		// classical ModProof and DLN proofs are replaced by per-participant SNARKs.
		// ContextJ provides SSID domain separation to prevent cross-ceremony proof replay.
		nTasks := 1 // modProof goroutine
		if !round.Parameters.NoProofDLN() {
			nTasks = 3 // + 2 DLN proof verifications
		}
		wg.Add(nTasks)
		go func(j int, msg tss.ParsedMessage, r2msg1 *DGRound2Message1) {
			defer wg.Done()
			if round.Parameters.NoProofMod() {
				return
			}
			modProof, err := r2msg1.UnmarshalModProof()
			if err != nil {
				paiProofCulprits[j] = msg.GetFrom()
				common.Logger.Warningf("modProof unmarshal failed for party %s: %v", msg.GetFrom(), err)
				return
			}
			ContextJ := common.AppendBigIntToBytesSlice(round.temp.ssid, big.NewInt(int64(j)))
			if ok := modProof.Verify(ContextJ, paiPK.N); !ok {
				paiProofCulprits[j] = msg.GetFrom()
				common.Logger.Warningf("modProof verify failed for party %s", msg.GetFrom())
			}
		}(j, msg, r2msg1)
		if !round.Parameters.NoProofDLN() {
			_j := j
			_msg := msg
			ContextJ := common.AppendBigIntToBytesSlice(round.temp.ssid, big.NewInt(int64(j)))
			dlnVerifier.VerifyDLNProof1(r2msg1, ContextJ, H1j, H2j, NTildej, func(isValid bool) {
				if !isValid {
					dlnProof1FailCulprits[_j] = _msg.GetFrom()
					common.Logger.Warningf("dln proof 1 verify failed for party %s", _msg.GetFrom())
				}
				wg.Done()
			})
			dlnVerifier.VerifyDLNProof2(r2msg1, ContextJ, H2j, H1j, NTildej, func(isValid bool) {
				if !isValid {
					dlnProof2FailCulprits[_j] = _msg.GetFrom()
					common.Logger.Warningf("dln proof 2 verify failed for party %s", _msg.GetFrom())
				}
				wg.Done()
			})
		}
	}
	wg.Wait()
	for _, culprit := range append(append(paiProofCulprits, dlnProof1FailCulprits...), dlnProof2FailCulprits...) {
		if culprit != nil {
			return round.WrapError(errors.New("dln proof verification failed"), culprit)
		}
	}
	// save NTilde_j, h1_j, h2_j received in NewCommitteeStep1 here
	for j, msg := range round.temp.dgRound2Message1s {
		if j == i {
			continue
		}
		r2msg1 := msg.Content().(*DGRound2Message1)
		round.save.NTildej[j] = new(big.Int).SetBytes(r2msg1.NTilde)
		round.save.H1j[j] = new(big.Int).SetBytes(r2msg1.H1)
		round.save.H2j[j] = new(big.Int).SetBytes(r2msg1.H2)
	}

	// 4.
	newXi := big.NewInt(0)

	// 5-9.
	modQ := common.ModInt(round.Params().EC().Params().N)
	vjc := make([][]*crypto.ECPoint, len(round.OldParties().IDs()))
	for j := 0; j <= len(vjc)-1; j++ { // P1..P_t+1. Ps are indexed from 0 here
		// 6-7.
		r1msg := round.temp.dgRound1Messages[j].Content().(*DGRound1Message)
		r3msg2 := round.temp.dgRound3Message2s[j].Content().(*DGRound3Message2)

		vCj, vDj := r1msg.UnmarshalVCommitment(), r3msg2.UnmarshalVDeCommitment()

		// 6. unpack flat "v" commitment content
		vCmtDeCmt := commitments.HashCommitDecommit{C: vCj, D: vDj}
		ok, flatVs := vCmtDeCmt.DeCommit()
		if !ok || len(flatVs) != (round.NewThreshold()+1)*2 { // they're points so * 2
			// TODO collect culprits and return a list of them as per convention
			return round.WrapError(errors.New("de-commitment of v_j0..v_jt failed"), round.Parties().IDs()[j])
		}
		vj, err := crypto.UnFlattenECPoints(round.Params().EC(), flatVs)
		if err != nil {
			return round.WrapError(err, round.Parties().IDs()[j])
		}
		vjc[j] = vj

		// [FORK] ReceiverID binding check: upstream did not include or verify a receiver
		// identifier in P2P resharing messages. We verify the ReceiverId field matches our
		// Key to prevent share misdirection attacks where a compromised transport layer
		// routes party A's resharing share to party B.
		r3msg1 := round.temp.dgRound3Message1s[j].Content().(*DGRound3Message1)
		myKey := round.PartyID().KeyInt().Bytes()
		if !bytes.Equal(r3msg1.GetReceiverId(), myKey) {
			return round.WrapError(errors.New("receiverId mismatch: resharing share not intended for this party"), round.Parties().IDs()[j])
		}
		sharej := &vss.Share{
			Threshold: round.NewThreshold(),
			ID:        round.PartyID().KeyInt(),
			Share:     new(big.Int).SetBytes(r3msg1.Share),
		}
		if ok := sharej.Verify(round.Params().EC(), round.NewThreshold(), vj); !ok {
			// TODO collect culprits and return a list of them as per convention
			return round.WrapError(errors.New("share from old committee did not pass Verify()"), round.Parties().IDs()[j])
		}

		// 9.
		newXi = new(big.Int).Add(newXi, sharej.Share)
	}
	// [FORK] Mod reduction + zero check: upstream did not reduce newXi mod q and did not check
	// for zero. Without mod reduction, the value could exceed the curve order (correctness issue).
	// A zero private key share is degenerate and would break threshold ECDSA signing.
	newXi = new(big.Int).Mod(newXi, round.Params().EC().Params().N)
	if newXi.Sign() == 0 {
		return round.WrapError(errors.New("newXi is zero"))
	}

	// 10-13.
	var err error
	Vc := make([]*crypto.ECPoint, round.NewThreshold()+1)
	for c := 0; c <= round.NewThreshold(); c++ {
		Vc[c] = vjc[0][c]
		for j := 1; j <= len(vjc)-1; j++ {
			Vc[c], err = Vc[c].Add(vjc[j][c])
			if err != nil {
				return round.WrapError(errors2.Wrapf(err, "Vc[c].Add(vjc[j][c])"))
			}
		}
	}

	// 14.
	if !Vc[0].Equals(round.save.ECDSAPub) {
		return round.WrapError(errors.New("assertion failed: V_0 != y"), round.PartyID())
	}

	// 15-19.
	newKs := make([]*big.Int, 0, round.NewPartyCount())
	newBigXjs := make([]*crypto.ECPoint, round.NewPartyCount())
	paiProofCulprits = make([]*tss.PartyID, 0, round.NewPartyCount()) // who caused the error(s)
	for j := 0; j < round.NewPartyCount(); j++ {
		Pj := round.NewParties().IDs()[j]
		kj := Pj.KeyInt()
		newBigXj := Vc[0]
		newKs = append(newKs, kj)
		z := new(big.Int).SetInt64(int64(1))
		for c := 1; c <= round.NewThreshold(); c++ {
			z = modQ.Mul(z, kj)
			newBigXj, err = newBigXj.Add(Vc[c].ScalarMult(z))
			// [FORK] Break on Add error: upstream continued the inner polynomial evaluation
			// loop after an Add error (recording the culprit but potentially corrupting
			// subsequent point additions on the already-corrupted accumulator). We break
			// immediately on the first error.
			if err != nil {
				paiProofCulprits = append(paiProofCulprits, Pj)
				break
			}
		}
		// [FORK] newBigXj identity-point check: upstream did not validate. A public key share
		// at the identity point breaks threshold ECDSA verification in future signing ceremonies.
		// Defense-in-depth: on Weierstrass curves, Add() calls NewECPoint which rejects (0,0),
		// so this is unreachable. Essential on Edwards curves where identity (0,1) passes.
		if newBigXj.IsIdentity() {
			paiProofCulprits = append(paiProofCulprits, Pj)
		} else {
			newBigXjs[j] = newBigXj
		}
	}
	if len(paiProofCulprits) > 0 {
		return round.WrapError(errors.New("newBigXj is the identity point or could not be computed"), paiProofCulprits...)
	}

	round.temp.newXi = newXi
	round.temp.newKs = newKs
	round.temp.newBigXjs = newBigXjs

	// Send facProof to new parties
	for j, Pj := range round.NewParties().IDs() {
		if j == i {
			continue
		}
		// [FORK] FacProof generation gated by NoProofFac(). In SNARK mode, classical fac
		// proofs are replaced by per-participant SNARKs. ContextJ provides SSID domain separation.
		var facProof *facproof.ProofFac
		if !round.Parameters.NoProofFac() {
			ContextJ := common.AppendBigIntToBytesSlice(round.temp.ssid, big.NewInt(int64(j)))
			facProof, err = facproof.NewProof(ContextJ, round.EC(), round.save.PaillierSK.N, round.save.NTildej[j],
				round.save.H1j[j], round.save.H2j[j], round.save.PaillierSK.P, round.save.PaillierSK.Q, round.Rand())
			if err != nil {
				return round.WrapError(err, Pi)
			}
		}
		r4msg1 := NewDGRound4Message1(Pj, Pi, facProof)
		round.out <- r4msg1
	}

	// Send an "ACK" message to both committees to signal that we're ready to save our data
	r4msg2 := NewDGRound4Message2(round.OldAndNewParties(), Pi)
	round.temp.dgRound4Message2s[i] = r4msg2
	round.out <- r4msg2

	return nil
}

func (round *round4) CanAccept(msg tss.ParsedMessage) bool {
	if _, ok := msg.Content().(*DGRound4Message1); ok {
		return !msg.IsBroadcast()
	}
	if _, ok := msg.Content().(*DGRound4Message2); ok {
		return msg.IsBroadcast()
	}
	return false
}

func (round *round4) Update() (bool, *tss.Error) {
	// accept messages from new -> old&new committees
	for j, msg2 := range round.temp.dgRound4Message2s {
		if round.newOK[j] {
			continue
		}
		if msg2 == nil || !round.CanAccept(msg2) {
			return false, nil
		}
		if round.ReSharingParams().IsNewCommittee() {
			msg1 := round.temp.dgRound4Message1s[j]
			if msg1 == nil || !round.CanAccept(msg1) {
				return false, nil
			}
		}
		round.newOK[j] = true
	}
	return true, nil
}

func (round *round4) NextRound() tss.Round {
	round.started = false
	return &round5{round}
}
