// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package signing

import (
	"errors"
	"math/big"

	errors2 "github.com/pkg/errors"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/crypto"
	"github.com/hemilabs/x/tss-lib/v2/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

func (round *round5) Start() *tss.Error {
	if round.started {
		return round.WrapError(errors.New("round already started"))
	}
	round.number = 5
	round.started = true
	round.resetOK()

	R := round.temp.pointGamma
	for j, Pj := range round.Parties().IDs() {
		if j == round.PartyID().Index {
			continue
		}
		ContextJ := common.AppendBigIntToBytesSlice(round.temp.ssid, big.NewInt(int64(j)))
		r1msg2 := round.temp.signRound1Message2s[j].Content().(*SignRound1Message2)
		r4msg := round.temp.signRound4Messages[j].Content().(*SignRound4Message)
		SCj, SDj := r1msg2.UnmarshalCommitment(), r4msg.UnmarshalDeCommitment()
		cmtDeCmt := commitments.HashCommitDecommit{C: SCj, D: SDj}
		ok, bigGammaJ := cmtDeCmt.DeCommit()
		if !ok || len(bigGammaJ) != 2 {
			return round.WrapError(errors.New("commitment verify failed"), Pj)
		}
		bigGammaJPoint, err := crypto.NewECPoint(round.Params().EC(), bigGammaJ[0], bigGammaJ[1])
		if err != nil {
			return round.WrapError(errors2.Wrapf(err, "NewECPoint(bigGammaJ)"), Pj)
		}
		proof, err := r4msg.UnmarshalZKProof(round.Params().EC())
		if err != nil {
			return round.WrapError(errors.New("failed to unmarshal bigGamma proof"), Pj)
		}
		ok = proof.Verify(ContextJ, bigGammaJPoint)
		if !ok {
			return round.WrapError(errors.New("failed to prove bigGamma"), Pj)
		}
		R, err = R.Add(bigGammaJPoint)
		if err != nil {
			return round.WrapError(errors2.Wrapf(err, "R.Add(bigGammaJ)"), Pj)
		}
	}

	// [FORK] Identity point checks: upstream does not verify the accumulated gamma point
	// or the resulting R. In upstream, the point at infinity would cause ScalarMult to
	// panic via NewECPoint. The fork's ScalarMult handles identity without panic, but
	// the resulting R still produces an invalid ECDSA signature (r=0).
	// Defense-in-depth: On Weierstrass curves (secp256k1, P-256), NewECPoint inside Add()
	// rejects (0,0), so an identity result would surface as an Add error above. On Edwards
	// curves the identity (0,1) passes NewECPoint, making this check essential. Retained for
	// curve-agnostic safety.
	if R.IsIdentity() {
		return round.WrapError(errors.New("sum of gamma points is the identity: degenerate nonce combination"))
	}
	R = R.ScalarMult(round.temp.thetaInverse)
	// Defense-in-depth: mathematically unreachable on prime-order groups — a non-zero scalar
	// times a non-identity point cannot produce the identity. Retained as a safeguard.
	if R.IsIdentity() {
		return round.WrapError(errors.New("R is the point at infinity after theta-inverse scaling"))
	}
	N := round.Params().EC().Params().N
	modN := common.ModInt(N)
	rx := R.X()
	ry := R.Y()

	// [FORK] Zero-r check: upstream does not validate r. ECDSA requires r = R.x mod N != 0;
	// a zero r produces an invalid signature. Early detection avoids wasting 4 more rounds.
	if new(big.Int).Mod(rx, N).Sign() == 0 {
		return round.WrapError(errors.New("r component of signature is zero: invalid nonce combination"))
	}

	si := modN.Add(modN.Mul(round.temp.m, round.temp.k), modN.Mul(rx, round.temp.sigma))

	// [FORK] Guard si=0: R.ScalarMult(si) panics on zero scalar (identity point).
	// si=0 means a degenerate key/nonce combination that cannot produce a valid signature.
	if si.Sign() == 0 {
		return round.WrapError(errors.New("partial signature si is zero: degenerate key/nonce combination"))
	}

	// [FORK] Clear secret nonces from memory. Upstream sets these to the package-level
	// `zero` variable (e.g. `round.temp.w = zero`), which aliases a shared mutable
	// pointer — a latent corruption vector if any future code mutates these fields.
	// We use fresh allocations (new(big.Int)) to avoid that aliasing bug.
	// Additionally, upstream does not clear gamma or sigma at all; we zero them here
	// to minimize the window during which secret material remains in memory.
	round.temp.w = new(big.Int)
	round.temp.k = new(big.Int)
	round.temp.gamma = new(big.Int)
	round.temp.sigma = new(big.Int)

	li := common.GetRandomPositiveInt(round.Rand(), N)  // li
	roI := common.GetRandomPositiveInt(round.Rand(), N) // pi
	rToSi := R.ScalarMult(si)
	liPoint := crypto.ScalarBaseMult(round.Params().EC(), li)
	bigAi := crypto.ScalarBaseMult(round.Params().EC(), roI)
	bigVi, err := rToSi.Add(liPoint)
	if err != nil {
		return round.WrapError(errors2.Wrapf(err, "rToSi.Add(li)"))
	}

	cmt := commitments.NewHashCommitment(round.Rand(), bigVi.X(), bigVi.Y(), bigAi.X(), bigAi.Y())
	r5msg := NewSignRound5Message(round.PartyID(), cmt.C)
	round.temp.signRound5Messages[round.PartyID().Index] = r5msg
	round.out <- r5msg

	round.temp.li = li
	round.temp.bigAi = bigAi
	round.temp.bigVi = bigVi
	round.temp.roi = roI
	round.temp.DPower = cmt.D
	round.temp.si = si
	round.temp.rx = rx
	round.temp.ry = ry
	round.temp.bigR = R

	return nil
}

func (round *round5) Update() (bool, *tss.Error) {
	ret := true
	for j, msg := range round.temp.signRound5Messages {
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

func (round *round5) CanAccept(msg tss.ParsedMessage) bool {
	if _, ok := msg.Content().(*SignRound5Message); ok {
		return msg.IsBroadcast()
	}
	return false
}

func (round *round5) NextRound() tss.Round {
	round.started = false
	return &round6{round}
}
