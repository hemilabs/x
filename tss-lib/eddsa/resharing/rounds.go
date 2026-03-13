// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package resharing

import (
	"errors"
	"math/big"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/eddsa/keygen"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

const (
	TaskName = "eddsa-resharing"
)

type (
	base struct {
		*tss.ReSharingParameters
		temp        *localTempData
		input, save *keygen.LocalPartySaveData
		out         chan<- tss.Message
		end         chan<- *keygen.LocalPartySaveData
		oldOK,      // old committee "ok" tracker
		newOK []bool // `ok` tracks parties which have been verified by Update(); this one is for the new committee
		started bool
		number  int
	}
	round1 struct {
		*base
	}
	round2 struct {
		*round1
	}
	round3 struct {
		*round2
	}
	round4 struct {
		*round3
	}
	round5 struct {
		*round4
	}
)

var (
	_ tss.Round = (*round1)(nil)
	_ tss.Round = (*round2)(nil)
	_ tss.Round = (*round3)(nil)
	_ tss.Round = (*round4)(nil)
	_ tss.Round = (*round5)(nil)
)

// ----- //

func (round *base) Params() *tss.Parameters {
	return round.ReSharingParameters.Parameters
}

func (round *base) ReSharingParams() *tss.ReSharingParameters {
	return round.ReSharingParameters
}

func (round *base) RoundNumber() int {
	return round.number
}

// CanProceed is inherited by other rounds
func (round *base) CanProceed() bool {
	if !round.started {
		return false
	}
	for _, ok := range append(round.oldOK, round.newOK...) {
		if !ok {
			return false
		}
	}
	return true
}

// WaitingFor is called by a Party for reporting back to the caller
func (round *base) WaitingFor() []*tss.PartyID {
	oldPs := round.OldParties().IDs()
	newPs := round.NewParties().IDs()
	idsMap := make(map[*tss.PartyID]bool)
	ids := make([]*tss.PartyID, 0, len(round.oldOK))
	for j, ok := range round.oldOK {
		if ok {
			continue
		}
		idsMap[oldPs[j]] = true
	}
	for j, ok := range round.newOK {
		if ok {
			continue
		}
		idsMap[newPs[j]] = true
	}
	// consolidate into the list
	for id := range idsMap {
		ids = append(ids, id)
	}
	return ids
}

func (round *base) WrapError(err error, culprits ...*tss.PartyID) *tss.Error {
	return tss.NewError(err, TaskName, round.number, round.PartyID(), culprits...)
}

// ----- //

// `oldOK` tracks parties which have been verified by Update()
func (round *base) resetOK() {
	for j := range round.oldOK {
		round.oldOK[j] = false
	}
	for j := range round.newOK {
		round.newOK[j] = false
	}
}

// sets all pairings in `oldOK` to true
func (round *base) allOldOK() {
	for j := range round.oldOK {
		round.oldOK[j] = true
	}
}

// sets all pairings in `newOK` to true
func (round *base) allNewOK() {
	for j := range round.newOK {
		round.newOK[j] = true
	}
}

// [FORK] getSSID: upstream had no SSID for resharing at all. This is entirely new code.
// Includes: (1) "eddsa-resharing" protocol tag for cross-protocol domain separation,
// (2) full curve parameters including B, (3) both old and new party keys, (4) the EDDSA
// public key being reshared, (5) old/new party counts and thresholds, (6) round number,
// (7) caller-supplied ssidNonce for concurrent sessions. This ensures every resharing
// session has a cryptographically unique context.
func (round *base) getSSID() ([]byte, error) {
	ssidList := []*big.Int{new(big.Int).SetBytes([]byte("eddsa-resharing")), round.EC().Params().P, round.EC().Params().N, round.EC().Params().B, round.EC().Params().Gx, round.EC().Params().Gy} // protocol tag + ec curve
	ssidList = append(ssidList, round.Parties().IDs().Keys()...)    // old parties
	ssidList = append(ssidList, round.NewParties().IDs().Keys()...) // new parties
	if round.input.EDDSAPub == nil {
		return nil, round.WrapError(errors.New("read EDDSAPub failed"), round.PartyID())
	}
	ssidList = append(ssidList, round.input.EDDSAPub.X(), round.input.EDDSAPub.Y()) // public key
	ssidList = append(ssidList, big.NewInt(int64(round.ReSharingParams().PartyCount())))    // old party count
	ssidList = append(ssidList, big.NewInt(int64(round.Threshold())))                       // old threshold
	ssidList = append(ssidList, big.NewInt(int64(round.ReSharingParams().NewPartyCount()))) // new party count
	ssidList = append(ssidList, big.NewInt(int64(round.ReSharingParams().NewThreshold())))  // new threshold
	ssidList = append(ssidList, big.NewInt(int64(round.number)))                            // round number
	ssidList = append(ssidList, round.temp.ssidNonce)
	if cid := round.Params().CeremonyID(); len(cid) > 0 {
		ssidList = append(ssidList, new(big.Int).SetBytes(cid))
	}
	ssid := common.SHA512_256i(ssidList...).Bytes()

	return ssid, nil
}
