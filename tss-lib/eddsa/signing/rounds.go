// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package signing

import (
	"errors"
	"math/big"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/crypto"
	"github.com/hemilabs/x/tss-lib/v2/eddsa/keygen"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

const (
	TaskName = "eddsa-signing"
)

type (
	base struct {
		*tss.Parameters
		key     *keygen.LocalPartySaveData
		data    *common.SignatureData
		temp    *localTempData
		out     chan<- tss.Message
		end     chan<- *common.SignatureData
		ok      []bool // `ok` tracks parties which have been verified by Update()
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
	finalization struct {
		*round3
	}
)

var (
	_ tss.Round = (*round1)(nil)
	_ tss.Round = (*round2)(nil)
	_ tss.Round = (*round3)(nil)
	_ tss.Round = (*finalization)(nil)
)

// ----- //

func (round *base) Params() *tss.Parameters {
	return round.Parameters
}

func (round *base) RoundNumber() int {
	return round.number
}

// CanProceed is inherited by other rounds
func (round *base) CanProceed() bool {
	if !round.started {
		return false
	}
	for _, ok := range round.ok {
		if !ok {
			return false
		}
	}
	return true
}

// WaitingFor is called by a Party for reporting back to the caller
func (round *base) WaitingFor() []*tss.PartyID {
	Ps := round.Parties().IDs()
	ids := make([]*tss.PartyID, 0, len(round.ok))
	for j, ok := range round.ok {
		if ok {
			continue
		}
		ids = append(ids, Ps[j])
	}
	return ids
}

func (round *base) WrapError(err error, culprits ...*tss.PartyID) *tss.Error {
	return tss.NewError(err, TaskName, round.number, round.PartyID(), culprits...)
}

// ----- //

// `ok` tracks parties which have been verified by Update()
func (round *base) resetOK() {
	for j := range round.ok {
		round.ok[j] = false
	}
}

// [FORK] getSSID: upstream SSID included {P, N, Gx, Gy}, party keys, BigXj, round number,
// and ssidNonce (hardcoded to 0). Hardened with: (1) "eddsa-signing" protocol tag to prevent
// cross-protocol SSID collisions, (2) curve parameter B for full curve identification,
// (3) partyCount and threshold binding, (4) parameterized ssidNonce via SSIDNonce() (upstream
// hardcodes to 0), (5) message binding to prevent cross-session Schnorr proof replay when
// two sessions share the same party set.
func (round *base) getSSID() ([]byte, error) {
	ssidList := []*big.Int{new(big.Int).SetBytes([]byte("eddsa-signing")), round.EC().Params().P, round.EC().Params().N, round.EC().Params().B, round.EC().Params().Gx, round.EC().Params().Gy} // protocol tag + ec curve
	ssidList = append(ssidList, round.Parties().IDs().Keys()...)                                                                                                                              // parties
	BigXjList, err := crypto.FlattenECPoints(round.key.BigXj)
	if err != nil {
		return nil, round.WrapError(errors.New("read BigXj failed"), round.PartyID())
	}
	ssidList = append(ssidList, BigXjList...)                          // BigXj
	ssidList = append(ssidList, big.NewInt(int64(round.PartyCount()))) // party count
	ssidList = append(ssidList, big.NewInt(int64(round.Threshold())))  // threshold
	ssidList = append(ssidList, big.NewInt(int64(round.number)))       // round number
	ssidList = append(ssidList, round.temp.ssidNonce)
	// [FORK] Bind the message being signed into the SSID to prevent cross-session
	// proof replay when two signing sessions use the same party set and nonce.
	if round.temp.m != nil {
		ssidList = append(ssidList, round.temp.m)
	}
	ssid := common.SHA512_256i(ssidList...).Bytes()

	return ssid, nil
}
