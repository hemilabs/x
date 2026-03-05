// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package signing

import (
	"bytes"
	"errors"
	"math/big"
	"sync"

	errorspkg "github.com/pkg/errors"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/crypto/mta"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

func (round *round2) Start() *tss.Error {
	if round.started {
		return round.WrapError(errors.New("round already started"))
	}
	round.number = 2
	round.started = true
	round.resetOK()

	i := round.PartyID().Index
	round.ok[i] = true

	// [FORK] ReceiverID verification: upstream does not include or check a receiver field
	// in P2P messages. Without this, a relay/reflection attack can deliver a P2P message
	// intended for party A to party B, causing B to use A's MtA ciphertext.
	myKey := round.PartyID().KeyInt().Bytes()
	for j, Pj := range round.Parties().IDs() {
		if j == i {
			continue
		}
		r1msg := round.temp.signRound1Message1s[j].Content().(*SignRound1Message1)
		if !bytes.Equal(r1msg.GetReceiverId(), myKey) {
			return round.WrapError(errors.New("receiverId mismatch: message not intended for this party"), Pj)
		}
	}

	errChs := make(chan *tss.Error, (len(round.Parties().IDs())-1)*2)
	wg := sync.WaitGroup{}
	wg.Add((len(round.Parties().IDs()) - 1) * 2)
	// [FORK] Session-tagged MtA context: upstream passes a single ContextI = SSID || i
	// (raw byte concatenation) to BobMid/BobMidWC. Our fork: (1) uses length-prefixed
	// encoding for ContextI, and (2) passes a separate AliceContextJ = SSID || j so
	// Alice's and Bob's proofs are bound to distinct per-party contexts.
	ContextI := common.AppendBigIntToBytesSlice(round.temp.ssid, new(big.Int).SetUint64(uint64(i)))
	for j, Pj := range round.Parties().IDs() {
		if j == i {
			continue
		}
		// Bob_mid
		go func(j int, Pj *tss.PartyID) {
			defer wg.Done()
			r1msg := round.temp.signRound1Message1s[j].Content().(*SignRound1Message1)
			rangeProofAliceJ, err := r1msg.UnmarshalRangeProofAlice()
			if err != nil {
				errChs <- round.WrapError(errorspkg.Wrapf(err, "UnmarshalRangeProofAlice failed"), Pj)
				return
			}
			// Alice's range proof was created with Alice's context (SSID || j),
			// Bob's own proof is created with Bob's context (SSID || i).
			AliceContextJ := common.AppendBigIntToBytesSlice(round.temp.ssid, new(big.Int).SetUint64(uint64(j)))
			beta, c1ji, _, pi1ji, err := mta.BobMid(
				AliceContextJ,
				ContextI,
				round.Parameters.EC(),
				round.key.PaillierPKs[j],
				rangeProofAliceJ,
				round.temp.gamma,
				r1msg.UnmarshalC(),
				round.key.NTildej[j],
				round.key.H1j[j],
				round.key.H2j[j],
				round.key.NTildej[i],
				round.key.H1j[i],
				round.key.H2j[i],
				round.Rand(),
			)
			if err != nil {
				errChs <- round.WrapError(err, Pj)
				return
			}
			// thread safe as these are pre-allocated
			round.temp.betas[j] = beta
			round.temp.c1jis[j] = c1ji
			round.temp.pi1jis[j] = pi1ji
		}(j, Pj)
		// Bob_mid_wc
		go func(j int, Pj *tss.PartyID) {
			defer wg.Done()
			r1msg := round.temp.signRound1Message1s[j].Content().(*SignRound1Message1)
			rangeProofAliceJ, err := r1msg.UnmarshalRangeProofAlice()
			if err != nil {
				errChs <- round.WrapError(errorspkg.Wrapf(err, "UnmarshalRangeProofAlice failed"), Pj)
				return
			}
			AliceContextJ := common.AppendBigIntToBytesSlice(round.temp.ssid, new(big.Int).SetUint64(uint64(j)))
			v, c2ji, _, pi2ji, err := mta.BobMidWC(
				AliceContextJ,
				ContextI,
				round.Parameters.EC(),
				round.key.PaillierPKs[j],
				rangeProofAliceJ,
				round.temp.w,
				r1msg.UnmarshalC(),
				round.key.NTildej[j],
				round.key.H1j[j],
				round.key.H2j[j],
				round.key.NTildej[i],
				round.key.H1j[i],
				round.key.H2j[i],
				round.temp.bigWs[i],
				round.Rand(),
			)
			if err != nil {
				errChs <- round.WrapError(err, Pj)
				return
			}
			round.temp.vs[j] = v
			round.temp.c2jis[j] = c2ji
			round.temp.pi2jis[j] = pi2ji
		}(j, Pj)
	}
	// consume error channels; wait for goroutines
	wg.Wait()
	close(errChs)
	culprits := make([]*tss.PartyID, 0, len(round.Parties().IDs()))
	for err := range errChs {
		culprits = append(culprits, err.Culprits()...)
	}
	if len(culprits) > 0 {
		return round.WrapError(errors.New("failed to calculate Bob_mid or Bob_mid_wc"), culprits...)
	}
	// create and send messages
	for j, Pj := range round.Parties().IDs() {
		if j == i {
			continue
		}
		r2msg := NewSignRound2Message(
			Pj, round.PartyID(), round.temp.c1jis[j], round.temp.pi1jis[j], round.temp.c2jis[j], round.temp.pi2jis[j])
		round.out <- r2msg
	}
	return nil
}

func (round *round2) Update() (bool, *tss.Error) {
	ret := true
	for j, msg := range round.temp.signRound2Messages {
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

func (round *round2) CanAccept(msg tss.ParsedMessage) bool {
	if _, ok := msg.Content().(*SignRound2Message); ok {
		return !msg.IsBroadcast()
	}
	return false
}

func (round *round2) NextRound() tss.Round {
	round.started = false
	return &round3{round}
}
