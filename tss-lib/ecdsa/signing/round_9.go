// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package signing

import (
	"errors"

	errors2 "github.com/pkg/errors"

	"github.com/hemilabs/x/tss-lib/v2/crypto"
	"github.com/hemilabs/x/tss-lib/v2/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

func (round *round9) Start() *tss.Error {
	if round.started {
		return round.WrapError(errors.New("round already started"))
	}
	round.number = 9
	round.started = true
	round.resetOK()

	UX, UY := round.temp.Ui.X(), round.temp.Ui.Y()
	TX, TY := round.temp.Ti.X(), round.temp.Ti.Y()
	for j, Pj := range round.Parties().IDs() {
		if j == round.PartyID().Index {
			continue
		}

		r7msg := round.temp.signRound7Messages[j].Content().(*SignRound7Message)
		r8msg := round.temp.signRound8Messages[j].Content().(*SignRound8Message)
		cj, dj := r7msg.UnmarshalCommitment(), r8msg.UnmarshalDeCommitment()
		cmt := commitments.HashCommitDecommit{C: cj, D: dj}
		ok, values := cmt.DeCommit()
		if !ok || len(values) != 4 {
			return round.WrapError(errors.New("de-commitment for bigVj and bigAj failed"), Pj)
		}
		UjX, UjY, TjX, TjY := values[0], values[1], values[2], values[3]
		// [FORK] On-curve and identity-point validation for decommitted Uj, Tj. Upstream
		// uses raw (X,Y) coordinates from the decommitment without constructing ECPoint
		// objects, so off-curve or identity points are not caught. An identity Uj or Tj
		// would make the U==T consistency check trivially pass, hiding a malicious party.
		// Defense-in-depth: on Weierstrass curves, NewECPoint rejects (0,0), making the
		// IsIdentity() checks below unreachable. Essential on Edwards curves where (0,1) passes.
		Uj, err := crypto.NewECPoint(round.Params().EC(), UjX, UjY)
		if err != nil {
			return round.WrapError(errors2.Wrapf(err, "decommitted Uj not on curve"), Pj)
		}
		if Uj.IsIdentity() {
			return round.WrapError(errors.New("decommitted Uj is the identity point"), Pj)
		}
		Tj, err := crypto.NewECPoint(round.Params().EC(), TjX, TjY)
		if err != nil {
			return round.WrapError(errors2.Wrapf(err, "decommitted Tj not on curve"), Pj)
		}
		if Tj.IsIdentity() {
			return round.WrapError(errors.New("decommitted Tj is the identity point"), Pj)
		}
		UX, UY = round.Params().EC().Add(UX, UY, UjX, UjY)
		TX, TY = round.Params().EC().Add(TX, TY, TjX, TjY)
	}
	if UX.Cmp(TX) != 0 || UY.Cmp(TY) != 0 {
		// Don't blame self — the inconsistency is caused by at least one malicious party
		return round.WrapError(errors.New("U doesn't equal T: signature share inconsistency detected"))
	}

	r9msg := NewSignRound9Message(round.PartyID(), round.temp.si)
	round.temp.signRound9Messages[round.PartyID().Index] = r9msg
	round.out <- r9msg
	return nil
}

func (round *round9) Update() (bool, *tss.Error) {
	ret := true
	for j, msg := range round.temp.signRound9Messages {
		if round.ok[j] {
			continue
		}
		if msg == nil || !round.CanAccept(msg) {
			ret = false
			continue
		}
		round.ok[j] = true
	}
	return ret, nil
}

func (round *round9) CanAccept(msg tss.ParsedMessage) bool {
	if _, ok := msg.Content().(*SignRound9Message); ok {
		return msg.IsBroadcast()
	}
	return false
}

func (round *round9) NextRound() tss.Round {
	round.started = false
	return &finalization{round}
}
