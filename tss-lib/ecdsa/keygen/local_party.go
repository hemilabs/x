// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package keygen

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/hemilabs/x/tss-lib/v2/common"
	cmt "github.com/hemilabs/x/tss-lib/v2/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v2/crypto/vss"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

// Implements Party
// Implements Stringer
var (
	_ tss.Party    = (*LocalParty)(nil)
	_ fmt.Stringer = (*LocalParty)(nil)
)

type (
	LocalParty struct {
		*tss.BaseParty
		params *tss.Parameters

		temp localTempData
		data LocalPartySaveData

		// outbound messaging
		out chan<- tss.Message
		end chan<- *LocalPartySaveData
	}

	localMessageStore struct {
		kgRound1Messages,
		kgRound2Message1s,
		kgRound2Message2s,
		kgRound3Messages []tss.ParsedMessage
	}

	localTempData struct {
		localMessageStore

		// temp data (thrown away after keygen)
		ui            *big.Int // used for tests
		KGCs          []cmt.HashCommitment
		vs            vss.Vs
		ssid          []byte
		ssidNonce     *big.Int
		shares        vss.Shares
		deCommitPolyG cmt.HashDeCommitment
		// [FORK] Store VSS polynomial coefficients for SNARK witness extraction.
		// Upstream does not expose the polynomial; we need it so the SP1 per-participant
		// prover can reconstruct the party's secret share commitment.
		Poly []*big.Int
	}
)

// Exported, used in `tss` client
func NewLocalParty(
	params *tss.Parameters,
	out chan<- tss.Message,
	end chan<- *LocalPartySaveData,
	optionalPreParams ...LocalPreParams,
) tss.Party {
	partyCount := params.PartyCount()
	data := NewLocalPartySaveData(partyCount)
	// when `optionalPreParams` is provided we'll use the pre-computed primes instead of generating them from scratch
	if 0 < len(optionalPreParams) {
		if 1 < len(optionalPreParams) {
			panic(errors.New("keygen.NewLocalParty expected 0 or 1 item in `optionalPreParams`"))
		}
		if !optionalPreParams[0].ValidateWithProof() {
			panic(errors.New("`optionalPreParams` failed to validate; it might have been generated with an older version of tss-lib"))
		}
		data.LocalPreParams = optionalPreParams[0]
	}
	p := &LocalParty{
		BaseParty: new(tss.BaseParty),
		params:    params,
		temp:      localTempData{},
		data:      data,
		out:       out,
		end:       end,
	}
	// msgs init
	p.temp.kgRound1Messages = make([]tss.ParsedMessage, partyCount)
	p.temp.kgRound2Message1s = make([]tss.ParsedMessage, partyCount)
	p.temp.kgRound2Message2s = make([]tss.ParsedMessage, partyCount)
	p.temp.kgRound3Messages = make([]tss.ParsedMessage, partyCount)
	// temp data init
	p.temp.KGCs = make([]cmt.HashCommitment, partyCount)
	return p
}

func (p *LocalParty) FirstRound() tss.Round {
	return newRound1(p.params, &p.data, &p.temp, p.out, p.end)
}

func (p *LocalParty) Start() *tss.Error {
	return tss.BaseStart(p, TaskName)
}

func (p *LocalParty) Update(msg tss.ParsedMessage) (ok bool, err *tss.Error) {
	return tss.BaseUpdate(p, msg, TaskName)
}

func (p *LocalParty) UpdateFromBytes(wireBytes []byte, from *tss.PartyID, isBroadcast bool) (bool, *tss.Error) {
	msg, err := tss.ParseWireMessage(wireBytes, from, isBroadcast)
	if err != nil {
		return false, p.WrapError(err)
	}
	return p.Update(msg)
}

func (p *LocalParty) ValidateMessage(msg tss.ParsedMessage) (bool, *tss.Error) {
	if ok, err := p.BaseParty.ValidateMessage(msg); !ok || err != nil {
		return ok, err
	}
	// check that the message's "from index" will fit into the array
	if maxFromIdx := p.params.PartyCount() - 1; maxFromIdx < msg.GetFrom().Index {
		return false, p.WrapError(fmt.Errorf("received msg with a sender index too great (%d <= %d)",
			p.params.PartyCount(), msg.GetFrom().Index), msg.GetFrom())
	}
	// [FORK] Key-at-Index verification: upstream only checked index bounds. We additionally
	// verify that the sender's Key matches the party registered at the claimed Index. Without
	// this, an attacker could impersonate another party by sending a valid index with a
	// different Key, causing messages to be stored under the wrong party's slot.
	knownParty := p.params.Parties().IDs()[msg.GetFrom().Index]
	if knownParty.KeyInt().Cmp(msg.GetFrom().KeyInt()) != 0 {
		return false, p.WrapError(fmt.Errorf("sender Key does not match party at claimed Index %d", msg.GetFrom().Index), msg.GetFrom())
	}
	return true, nil
}

func (p *LocalParty) StoreMessage(msg tss.ParsedMessage) (bool, *tss.Error) {
	// ValidateBasic is cheap; double-check the message here in case the public StoreMessage was called externally
	if ok, err := p.ValidateMessage(msg); !ok || err != nil {
		return ok, err
	}
	fromPIdx := msg.GetFrom().Index

	// switch/case is necessary to store any messages beyond current round
	// [FORK] Reject duplicate messages for the same (round, sender) pair. Upstream would
	// silently overwrite stored messages, which breaks commit-then-reveal guarantees (an
	// attacker could replace a commitment after seeing the decommitment). We also validate
	// the broadcast/P2P flag at storage time to prevent slot poisoning (a P2P message
	// stored in a broadcast slot or vice versa).
	switch msg.Content().(type) {
	case *KGRound1Message: // broadcast
		if !msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("KGRound1Message expected broadcast but got P2P"), msg.GetFrom())
		}
		if p.temp.kgRound1Messages[fromPIdx] != nil {
			common.Logger.Warningf("duplicate KGRound1Message from %d ignored", fromPIdx)
			return true, nil
		}
		p.temp.kgRound1Messages[fromPIdx] = msg
	case *KGRound2Message1: // P2P
		if msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("KGRound2Message1 expected P2P but got broadcast"), msg.GetFrom())
		}
		if p.temp.kgRound2Message1s[fromPIdx] != nil {
			common.Logger.Warningf("duplicate KGRound2Message1 from %d ignored", fromPIdx)
			return true, nil
		}
		p.temp.kgRound2Message1s[fromPIdx] = msg
	case *KGRound2Message2: // broadcast
		if !msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("KGRound2Message2 expected broadcast but got P2P"), msg.GetFrom())
		}
		if p.temp.kgRound2Message2s[fromPIdx] != nil {
			common.Logger.Warningf("duplicate KGRound2Message2 from %d ignored", fromPIdx)
			return true, nil
		}
		p.temp.kgRound2Message2s[fromPIdx] = msg
	case *KGRound3Message: // broadcast
		if !msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("KGRound3Message expected broadcast but got P2P"), msg.GetFrom())
		}
		if p.temp.kgRound3Messages[fromPIdx] != nil {
			common.Logger.Warningf("duplicate KGRound3Message from %d ignored", fromPIdx)
			return true, nil
		}
		p.temp.kgRound3Messages[fromPIdx] = msg
	default: // unrecognised message, just ignore!
		common.Logger.Warningf("unrecognised message ignored: %v", msg)
		return false, nil
	}
	return true, nil
}

// recovers a party's original index in the set of parties during keygen
func (save LocalPartySaveData) OriginalIndex() (int, error) {
	index := -1
	ki := save.ShareID
	for j, kj := range save.Ks {
		if kj.Cmp(ki) != 0 {
			continue
		}
		index = j
		break
	}
	if index < 0 {
		return -1, errors.New("a party index could not be recovered from Ks")
	}
	return index, nil
}

// [FORK] GetPoly returns the VSS polynomial coefficients stored during Round 1.
// Returns nil if Round 1 has not completed yet. This method does not exist in
// upstream; it is used by the SP1 per-participant prover for witness extraction.
func (p *LocalParty) GetPoly() []*big.Int {
	return p.temp.Poly
}

func (p *LocalParty) PartyID() *tss.PartyID {
	return p.params.PartyID()
}

func (p *LocalParty) String() string {
	return fmt.Sprintf("id: %s, %s", p.PartyID(), p.BaseParty.String())
}
