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

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/crypto"
	cmt "github.com/hemilabs/x/tss-lib/v2/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v2/eddsa/keygen"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

// Implements Party
// Implements Stringer
var _ tss.Party = (*LocalParty)(nil)
var _ fmt.Stringer = (*LocalParty)(nil)

type (
	LocalParty struct {
		*tss.BaseParty
		params *tss.Parameters

		keys keygen.LocalPartySaveData
		temp localTempData
		data *common.SignatureData

		// outbound messaging
		out chan<- tss.Message
		end chan<- *common.SignatureData
	}

	localMessageStore struct {
		signRound1Messages,
		signRound2Messages,
		signRound3Messages []tss.ParsedMessage
	}

	localTempData struct {
		localMessageStore

		// temp data (thrown away after sign) / round 1
		wi,
		m,
		ri *big.Int
		fullBytesLen int
		pointRi      *crypto.ECPoint
		deCommit     cmt.HashDeCommitment

		// round 2
		cjs []*big.Int
		si  *[32]byte

		// round 3
		r *big.Int

		ssid      []byte
		ssidNonce *big.Int
	}
)

func NewLocalParty(
	msg *big.Int,
	params *tss.Parameters,
	key keygen.LocalPartySaveData,
	out chan<- tss.Message,
	end chan<- *common.SignatureData,
	fullBytesLen ...int,
) tss.Party {
	// [FORK] Nil guard: upstream silently accepted nil msg, which would panic later in
	// signing rounds when accessing msg.Bytes(). Fail fast at construction time.
	if msg == nil {
		panic("eddsa/signing.NewLocalParty: message must not be nil")
	}
	partyCount := len(params.Parties().IDs())
	p := &LocalParty{
		BaseParty: new(tss.BaseParty),
		params:    params,
		keys:      keygen.BuildLocalSaveDataSubset(key, params.Parties().IDs()),
		temp:      localTempData{},
		data:      &common.SignatureData{},
		out:       out,
		end:       end,
	}
	// msgs init
	p.temp.signRound1Messages = make([]tss.ParsedMessage, partyCount)
	p.temp.signRound2Messages = make([]tss.ParsedMessage, partyCount)
	p.temp.signRound3Messages = make([]tss.ParsedMessage, partyCount)

	// temp data init
	p.temp.m = msg
	if len(fullBytesLen) > 0 {
		p.temp.fullBytesLen = fullBytesLen[0]
	} else {
		p.temp.fullBytesLen = 0
	}
	p.temp.cjs = make([]*big.Int, partyCount)
	return p
}

func (p *LocalParty) FirstRound() tss.Round {
	return newRound1(p.params, &p.keys, p.data, &p.temp, p.out, p.end)
}

func (p *LocalParty) Start() *tss.Error {
	return tss.BaseStart(p, TaskName, func(round tss.Round) *tss.Error {
		round1, ok := round.(*round1)
		if !ok {
			return round.WrapError(errors.New("unable to Start(). party is in an unexpected round"))
		}
		if err := round1.prepare(); err != nil {
			return round.WrapError(err)
		}
		return nil
	})
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
	if msg.GetFrom() == nil || !msg.GetFrom().ValidateBasic() {
		return false, p.WrapError(fmt.Errorf("received msg with an invalid sender: %s", msg))
	}
	// check that the message's "from index" will fit into the array
	if maxFromIdx := len(p.params.Parties().IDs()) - 1; maxFromIdx < msg.GetFrom().Index {
		return false, p.WrapError(fmt.Errorf("received msg with a sender index too great (%d <= %d)",
			maxFromIdx, msg.GetFrom().Index), msg.GetFrom())
	}
	// [FORK] Key-at-Index verification: upstream only checked index bounds. We additionally
	// verify the sender's Key matches the party registered at the claimed Index to prevent
	// a malicious party from impersonating another by sending a valid index with a wrong key.
	knownParty := p.params.Parties().IDs()[msg.GetFrom().Index]
	if knownParty.KeyInt().Cmp(msg.GetFrom().KeyInt()) != 0 {
		return false, p.WrapError(fmt.Errorf("sender Key does not match party at claimed Index %d", msg.GetFrom().Index), msg.GetFrom())
	}
	return p.BaseParty.ValidateMessage(msg)
}

func (p *LocalParty) StoreMessage(msg tss.ParsedMessage) (bool, *tss.Error) {
	// ValidateBasic is cheap; double-check the message here in case the public StoreMessage was called externally
	if ok, err := p.ValidateMessage(msg); !ok || err != nil {
		return ok, err
	}
	fromPIdx := msg.GetFrom().Index

	// switch/case is necessary to store any messages beyond current round
	// [FORK] Defense-in-depth: reject duplicate messages for the same (round, sender) pair.
	// Upstream did not handle replays, leaving it to the caller. We enforce dedup here because
	// overwriting a stored message breaks commit-then-reveal guarantees. We also validate the
	// broadcast/P2P flag at storage time to prevent slot poisoning.
	switch msg.Content().(type) {
	case *SignRound1Message: // broadcast
		if !msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("SignRound1Message expected broadcast but got P2P"), msg.GetFrom())
		}
		if p.temp.signRound1Messages[fromPIdx] != nil {
			common.Logger.Warningf("duplicate SignRound1Message from %d ignored", fromPIdx)
			return true, nil
		}
		p.temp.signRound1Messages[fromPIdx] = msg
	case *SignRound2Message: // broadcast
		if !msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("SignRound2Message expected broadcast but got P2P"), msg.GetFrom())
		}
		if p.temp.signRound2Messages[fromPIdx] != nil {
			common.Logger.Warningf("duplicate SignRound2Message from %d ignored", fromPIdx)
			return true, nil
		}
		p.temp.signRound2Messages[fromPIdx] = msg
	case *SignRound3Message: // broadcast
		if !msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("SignRound3Message expected broadcast but got P2P"), msg.GetFrom())
		}
		if p.temp.signRound3Messages[fromPIdx] != nil {
			common.Logger.Warningf("duplicate SignRound3Message from %d ignored", fromPIdx)
			return true, nil
		}
		p.temp.signRound3Messages[fromPIdx] = msg
	default: // unrecognised message, just ignore!
		common.Logger.Warningf("unrecognised message ignored: %v", msg)
		return false, nil
	}
	return true, nil
}

func (p *LocalParty) PartyID() *tss.PartyID {
	return p.params.PartyID()
}

func (p *LocalParty) String() string {
	return fmt.Sprintf("id: %s, %s", p.PartyID(), p.BaseParty.String())
}
