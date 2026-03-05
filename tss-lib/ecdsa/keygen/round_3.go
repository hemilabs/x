// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package keygen

import (
	"bytes"
	"errors"
	"math/big"

	"github.com/hashicorp/go-multierror"
	errors2 "github.com/pkg/errors"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/crypto"
	"github.com/hemilabs/x/tss-lib/v2/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v2/crypto/vss"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

func (round *round3) Start() *tss.Error {
	if round.started {
		return round.WrapError(errors.New("round already started"))
	}
	round.number = 3
	round.started = true
	round.resetOK()

	Ps := round.Parties().IDs()
	PIdx := round.PartyID().Index

	// 1,9. calculate xi
	xi := new(big.Int).Set(round.temp.shares[PIdx].Share)
	for j := range Ps {
		if j == PIdx {
			continue
		}
		r2msg1 := round.temp.kgRound2Message1s[j].Content().(*KGRound2Message1)
		share := r2msg1.UnmarshalShare()
		xi = new(big.Int).Add(xi, share)
	}
	round.save.Xi = new(big.Int).Mod(xi, round.Params().EC().Params().N)
	// [FORK] Xi=0 check: upstream did not validate. A zero private key share means the party
	// contributes nothing to the aggregate secret, and its public key share BigXj would be
	// the identity point.
	if round.save.Xi.Sign() == 0 {
		return round.WrapError(errors.New("Xi is zero"))
	}

	// 2-3.
	Vc := make(vss.Vs, round.Threshold()+1)
	for c := range Vc {
		Vc[c] = round.temp.vs[c] // ours
	}

	// 4-11.
	type vssOut struct {
		unWrappedErr error
		pjVs         vss.Vs
	}
	chs := make([]chan vssOut, len(Ps))
	for i := range chs {
		if i == PIdx {
			continue
		}
		chs[i] = make(chan vssOut)
	}
	for j := range Ps {
		if j == PIdx {
			continue
		}
		ContextJ := common.AppendBigIntToBytesSlice(round.temp.ssid, big.NewInt(int64(j)))
		// 6-8.
		go func(j int, ch chan<- vssOut) {
			// 4-9.
			KGCj := round.temp.KGCs[j]
			r2msg2 := round.temp.kgRound2Message2s[j].Content().(*KGRound2Message2)
			KGDj := r2msg2.UnmarshalDeCommitment()
			cmtDeCmt := commitments.HashCommitDecommit{C: KGCj, D: KGDj}
			ok, flatPolyGs := cmtDeCmt.DeCommit()
			if !ok || flatPolyGs == nil {
				ch <- vssOut{errors.New("de-commitment verify failed"), nil}
				return
			}
			PjVs, err := crypto.UnFlattenECPoints(round.Params().EC(), flatPolyGs)
			if err != nil {
				ch <- vssOut{err, nil}
				return
			}
			// [FORK] ModProof gating: upstream also gates by NoProofMod(), but uses it for
			// backward-compatible error tolerance (attempts unmarshal, then logs a warning
			// and continues if it fails). We skip unmarshal entirely in SNARK mode.
			if !round.Params().NoProofMod() {
				modProof, err := r2msg2.UnmarshalModProof()
				if err != nil {
					ch <- vssOut{errors.New("modProof verify failed"), nil}
					return
				}
				if ok = modProof.Verify(ContextJ, round.save.PaillierPKs[j].N); !ok {
					ch <- vssOut{errors.New("modProof verify failed"), nil}
					return
				}
			}
			r2msg1 := round.temp.kgRound2Message1s[j].Content().(*KGRound2Message1)
			// [FORK] ReceiverID binding check: upstream did not include or verify a receiver
			// identifier in P2P messages. We verify the ReceiverId field matches our Key to
			// prevent share misdirection attacks where a compromised transport layer routes
			// party A's share to party B.
			myKey := round.PartyID().KeyInt().Bytes()
			if !bytes.Equal(r2msg1.GetReceiverId(), myKey) {
				ch <- vssOut{errors.New("receiverId mismatch: message not intended for this party"), nil}
				return
			}
			PjShare := vss.Share{
				Threshold: round.Threshold(),
				ID:        round.PartyID().KeyInt(),
				Share:     r2msg1.UnmarshalShare(),
			}
			if ok = PjShare.Verify(round.Params().EC(), round.Threshold(), PjVs); !ok {
				ch <- vssOut{errors.New("vss verify failed"), nil}
				return
			}
			// [FORK] FacProof gating: upstream also gates by NoProofFac(), but uses it for
			// backward-compatible error tolerance (attempts unmarshal, then logs a warning
			// and continues if it fails). We skip unmarshal entirely in SNARK mode.
			if !round.Params().NoProofFac() {
				facProof, err := r2msg1.UnmarshalFacProof()
				if err != nil {
					ch <- vssOut{errors.New("facProof verify failed"), nil}
					return
				}
				if ok = facProof.Verify(ContextJ, round.EC(), round.save.PaillierPKs[j].N, round.save.NTildei,
					round.save.H1i, round.save.H2i); !ok {
					ch <- vssOut{errors.New("facProof verify failed"), nil}
					return
				}
			}

			// (9) handled above
			ch <- vssOut{nil, PjVs}
		}(j, chs[j])
	}

	// consume unbuffered channels (end the goroutines)
	vssResults := make([]vssOut, len(Ps))
	{
		culprits := make([]*tss.PartyID, 0, len(Ps)) // who caused the error(s)
		for j, Pj := range Ps {
			if j == PIdx {
				continue
			}
			vssResults[j] = <-chs[j]
			// collect culprits to error out with
			if err := vssResults[j].unWrappedErr; err != nil {
				culprits = append(culprits, Pj)
			}
		}
		var multiErr error
		if len(culprits) > 0 {
			for _, vssResult := range vssResults {
				if vssResult.unWrappedErr != nil {
					multiErr = multierror.Append(multiErr, vssResult.unWrappedErr)
				}
			}
			return round.WrapError(multiErr, culprits...)
		}
	}
	{
		var err error
		culprits := make([]*tss.PartyID, 0, len(Ps)) // who caused the error(s)
		for j, Pj := range Ps {
			if j == PIdx {
				continue
			}
			// 10-11.
			PjVs := vssResults[j].pjVs
			for c := 0; c <= round.Threshold(); c++ {
				Vc[c], err = Vc[c].Add(PjVs[c])
				if err != nil {
					culprits = append(culprits, Pj)
				}
			}
		}
		if len(culprits) > 0 {
			return round.WrapError(errors.New("adding PjVs[c] to Vc[c] resulted in a point not on the curve"), culprits...)
		}
	}

	// 12-16. compute Xj for each Pj
	{
		var err error
		modQ := common.ModInt(round.Params().EC().Params().N)
		culprits := make([]*tss.PartyID, 0, len(Ps)) // who caused the error(s)
		bigXj := round.save.BigXj
		for j := 0; j < round.PartyCount(); j++ {
			Pj := round.Parties().IDs()[j]
			kj := Pj.KeyInt()
			BigXj := Vc[0]
			z := new(big.Int).SetInt64(int64(1))
			for c := 1; c <= round.Threshold(); c++ {
				z = modQ.Mul(z, kj)
				BigXj, err = BigXj.Add(Vc[c].ScalarMult(z))
				if err != nil {
					culprits = append(culprits, Pj)
				}
			}
			// [FORK] BigXj identity-point check: upstream did not validate. A public key share
			// at the identity point (point at infinity) breaks threshold ECDSA verification.
			// Defense-in-depth: on Weierstrass curves, Add() calls NewECPoint which rejects (0,0),
			// so this is unreachable. Essential on Edwards curves where identity (0,1) passes.
			if BigXj.IsIdentity() {
				culprits = append(culprits, Pj)
			} else {
				bigXj[j] = BigXj
			}
		}
		if len(culprits) > 0 {
			return round.WrapError(errors.New("BigXj is the identity point or could not be computed"), culprits...)
		}
		round.save.BigXj = bigXj
	}

	// 17. compute and SAVE the ECDSA public key `y`
	ecdsaPubKey, err := crypto.NewECPoint(round.Params().EC(), Vc[0].X(), Vc[0].Y())
	if err != nil {
		return round.WrapError(errors2.Wrapf(err, "public key is not on the curve"))
	}
	// [FORK] ECDSAPub identity-point check: upstream did not validate. An identity-point
	// public key means the aggregate secret is zero, which is catastrophic for ECDSA.
	// Defense-in-depth: on Weierstrass curves, NewECPoint above rejects (0,0), making
	// this unreachable. Essential on Edwards curves where (0,1) passes IsOnCurve.
	if ecdsaPubKey.IsIdentity() {
		return round.WrapError(errors.New("public key is the identity point"))
	}
	round.save.ECDSAPub = ecdsaPubKey

	// PRINT public key & private share
	common.Logger.Debugf("%s public key: %x", round.PartyID(), ecdsaPubKey)

	// BROADCAST paillier proof for Pi
	ki := round.PartyID().KeyInt()
	proof := round.save.PaillierSK.Proof(ki, ecdsaPubKey)
	r3msg := NewKGRound3Message(round.PartyID(), proof)
	round.temp.kgRound3Messages[PIdx] = r3msg
	round.out <- r3msg
	return nil
}

func (round *round3) CanAccept(msg tss.ParsedMessage) bool {
	if _, ok := msg.Content().(*KGRound3Message); ok {
		return msg.IsBroadcast()
	}
	return false
}

func (round *round3) Update() (bool, *tss.Error) {
	ret := true
	for j, msg := range round.temp.kgRound3Messages {
		if round.ok[j] {
			continue
		}
		if msg == nil || !round.CanAccept(msg) {
			ret = false
			continue
		}
		// proof check is in round 4
		round.ok[j] = true
	}
	return ret, nil
}

func (round *round3) NextRound() tss.Round {
	round.started = false
	return &round4{round}
}
