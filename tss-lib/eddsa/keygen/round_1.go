// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package keygen

import (
	"errors"
	"math/big"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/crypto"
	cmts "github.com/hemilabs/x/tss-lib/v2/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v2/crypto/vss"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

var zero = big.NewInt(0)

// round 1 represents round 1 of the keygen part of the EDDSA TSS spec
func newRound1(params *tss.Parameters, save *LocalPartySaveData, temp *localTempData, out chan<- tss.Message, end chan<- *LocalPartySaveData) tss.Round {
	return &round1{
		&base{params, save, temp, out, end, make([]bool, len(params.Parties().IDs())), false, 1},
	}
}

func (round *round1) Start() *tss.Error {
	if round.started {
		return round.WrapError(errors.New("round already started"))
	}
	round.number = 1
	round.started = true
	round.resetOK()

	Pi := round.PartyID()
	i := Pi.Index

	// [FORK] Use caller-supplied SSIDNonce instead of upstream's hardcoded 0.
	// This enables distinct session IDs for concurrent ceremonies (SC#662).
	round.temp.ssidNonce = new(big.Int).SetUint64(uint64(round.Params().SSIDNonce()))
	ssid, err := round.getSSID()
	if err != nil {
		return round.WrapError(err)
	}
	round.temp.ssid = ssid

	// 1. calculate "partial" key share ui
	ui := common.GetRandomPositiveInt(round.PartialKeyRand(), round.Params().EC().Params().N)
	round.temp.ui = ui

	// 2. compute the vss shares
	ids := round.Parties().IDs().Keys()
	// [FORK] vss.Create now returns (vs, shares, poly, err). The poly return is used
	// by ECDSA keygen for SNARK witness extraction; unused here but API must match.
	vs, shares, _, err := vss.Create(round.EC(), round.Threshold(), ui, ids, round.Rand())
	if err != nil {
		return round.WrapError(err, Pi)
	}
	round.save.Ks = ids

	// [FORK] Upstream set `ui = zero` here (local variable only — round.temp.ui was unchanged,
	// so the value was still available for round 2's Schnorr proof). We similarly clear the
	// local variable and defer zeroing round.temp.ui to round 2, after the Schnorr proof.
	ui = nil

	// 3. make commitment -> (C, D)
	pGFlat, err := crypto.FlattenECPoints(vs)
	if err != nil {
		return round.WrapError(err, Pi)
	}
	cmt := cmts.NewHashCommitment(round.Rand(), pGFlat...)

	// for this P: SAVE
	// - shareID
	// and keep in temporary storage:
	// - VSS Vs
	// - our set of Shamir shares
	round.save.ShareID = ids[i]
	round.temp.vs = vs
	round.temp.shares = shares

	round.temp.deCommitPolyG = cmt.D

	// BROADCAST commitments
	{
		msg := NewKGRound1Message(round.PartyID(), cmt.C)
		round.temp.kgRound1Messages[i] = msg
		round.out <- msg
	}
	return nil
}

func (round *round1) CanAccept(msg tss.ParsedMessage) bool {
	if _, ok := msg.Content().(*KGRound1Message); ok {
		return msg.IsBroadcast()
	}
	return false
}

func (round *round1) Update() (bool, *tss.Error) {
	ret := true
	for j, msg := range round.temp.kgRound1Messages {
		if round.ok[j] {
			continue
		}
		if msg == nil || !round.CanAccept(msg) {
			ret = false
			continue
		}
		// vss check is in round 2
		round.ok[j] = true
	}
	return ret, nil
}

func (round *round1) NextRound() tss.Round {
	round.started = false
	return &round2{round}
}
