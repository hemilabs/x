// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package signing

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/binance-chain/edwards25519/edwards25519"
	"github.com/decred/dcrd/dcrec/edwards/v2"

	"github.com/hemilabs/x/tss-lib/v2/tss"
)

func (round *finalization) Start() *tss.Error {
	if round.started {
		return round.WrapError(errors.New("round already started"))
	}
	round.number = 4
	round.started = true
	round.resetOK()

	// [FORK] Nil guard on si: upstream did not check. Defense-in-depth: unreachable in
	// normal operation because round 3 always sets si before finalization runs, but if
	// round 3 failed to complete (e.g., due to an error in nonce aggregation), si would
	// be nil and the subsequent ScMulAdd would panic. Retained as a safeguard.
	if round.temp.si == nil {
		return round.WrapError(fmt.Errorf("si is nil: round 3 did not complete"))
	}
	sumS := round.temp.si
	// [FORK] Range check on each party's s_j share. Upstream accepts any value from
	// UnmarshalS() without bounds checking. Values outside [0, N) could produce a valid
	// but non-canonical signature or allow a malicious party to bias the aggregate.
	N := round.Params().EC().Params().N
	for j := range round.Parties().IDs() {
		round.ok[j] = true
		if j == round.PartyID().Index {
			continue
		}
		r3msg := round.temp.signRound3Messages[j].Content().(*SignRound3Message)
		sj := r3msg.UnmarshalS()
		// Defense-in-depth: sj.Sign()<0 is unreachable because UnmarshalS() uses SetBytes()
		// which always produces non-negative values. Retained alongside the Cmp(N) check for
		// completeness — the range check [0, N) is the meaningful validation.
		if sj.Sign() < 0 || sj.Cmp(N) >= 0 {
			return round.WrapError(fmt.Errorf("party %d sent s_i outside [0, N)", j),
				round.Parties().IDs()[j])
		}
		sjBytes := bigIntToEncodedBytes(sj)
		var tmpSumS [32]byte
		edwards25519.ScMulAdd(&tmpSumS, sumS, bigIntToEncodedBytes(big.NewInt(1)), sjBytes)
		sumS = &tmpSumS
	}
	s := encodedBytesToBigInt(sumS)

	// [FORK] Zero-S rejection: upstream did not check. A colluding set of malicious parties
	// could craft s_j shares that cancel each other out, producing S=0. A zero S is
	// degenerate and indicates colluding parties have cancelled each other's contributions.
	if s.Sign() == 0 {
		return round.WrapError(fmt.Errorf("accumulated S is zero: malicious share detected"))
	}

	// save the signature for final output
	round.data.Signature = append(bigIntToEncodedBytes(round.temp.r)[:], sumS[:]...)
	round.data.R = round.temp.r.Bytes()
	round.data.S = s.Bytes()
	if round.temp.fullBytesLen == 0 {
		round.data.M = round.temp.m.Bytes()
	} else {
		mBytes := make([]byte, round.temp.fullBytesLen)
		round.temp.m.FillBytes(mBytes)
		round.data.M = mBytes
	}

	pk := edwards.PublicKey{
		Curve: round.Params().EC(),
		X:     round.key.EDDSAPub.X(),
		Y:     round.key.EDDSAPub.Y(),
	}

	ok := edwards.Verify(&pk, round.data.M, round.temp.r, s)
	if !ok {
		return round.WrapError(fmt.Errorf("signature verification failed"))
	}
	round.end <- round.data

	return nil
}

func (round *finalization) CanAccept(msg tss.ParsedMessage) bool {
	// not expecting any incoming messages in this round
	return false
}

func (round *finalization) Update() (bool, *tss.Error) {
	// not expecting any incoming messages in this round
	return false, nil
}

func (round *finalization) NextRound() tss.Round {
	return nil // finished!
}
