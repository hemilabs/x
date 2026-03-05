// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package resharing

import (
	"bytes"
	"errors"
	"math/big"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

func (round *round5) Start() *tss.Error {
	if round.started {
		return round.WrapError(errors.New("round already started"))
	}
	round.number = 5
	round.started = true

	round.allOldOK()
	round.allNewOK()

	Pi := round.PartyID()
	i := Pi.Index

	if round.IsNewCommittee() {
		// 21.
		// for this P: SAVE data
		ContextI := common.AppendBigIntToBytesSlice(round.temp.ssid, big.NewInt(int64(i)))
		round.save.BigXj = round.temp.newBigXjs
		round.save.ShareID = round.PartyID().KeyInt()
		round.save.Xi = round.temp.newXi
		round.save.Ks = round.temp.newKs

		// misc: build list of paillier public keys to save
		for j, msg := range round.temp.dgRound2Message1s {
			if j == i {
				continue
			}
			r2msg1 := msg.Content().(*DGRound2Message1)
			round.save.PaillierPKs[j] = r2msg1.UnmarshalPaillierPK()
		}
		for j, msg := range round.temp.dgRound4Message1s {
			if j == i {
				continue
			}
			r4msg1 := msg.Content().(*DGRound4Message1)
			// [FORK] ReceiverID binding check on DGRound4Message1: upstream did not include
			// or verify a receiver identifier. We verify the ReceiverId matches our Key to
			// prevent fac proof redirection attacks (same pattern as share misdirection in round 4).
			receiverId := r4msg1.UnmarshalReceiverId()
			if !bytes.Equal(receiverId, round.PartyID().GetKey()) {
				return round.WrapError(errors.New("DGRound4Message1 receiverId does not match our key"), round.NewParties().IDs()[j])
			}
			// [FORK] FacProof verification gated by NoProofFac(). In SNARK mode, classical
			// fac proofs are replaced by per-participant SNARKs.
			if round.Parameters.NoProofFac() {
				continue
			}
			proof, err := r4msg1.UnmarshalFacProof()
			if err != nil {
				common.Logger.Warningf("facProof unmarshal failed for party %s: %v", msg.GetFrom(), err)
				return round.WrapError(err, round.NewParties().IDs()[j])
			}
			if ok := proof.Verify(ContextI, round.EC(), round.save.PaillierPKs[j].N, round.save.NTildei,
				round.save.H1i, round.save.H2i); !ok {
				common.Logger.Warningf("facProof verify failed for party %s", msg.GetFrom())
				return round.WrapError(errors.New("facProof verify failed"), round.NewParties().IDs()[j])
			}
		}
	}
	// [FORK] Unconditionally zero old Xi for any party in the old committee.
	// Upstream used an `else if` branch that missed dual-committee parties (members in both
	// old and new committees), leaving their old Xi in memory after resharing completed.
	// This correctness fix ensures the old secret share is wiped regardless of committee
	// membership configuration.
	if round.IsOldCommittee() {
		round.input.Xi.SetInt64(0)
	}

	round.end <- round.save
	return nil
}

func (round *round5) CanAccept(msg tss.ParsedMessage) bool {
	return false
}

func (round *round5) Update() (bool, *tss.Error) {
	return false, nil
}

func (round *round5) NextRound() tss.Round {
	return nil // both committees are finished!
}
