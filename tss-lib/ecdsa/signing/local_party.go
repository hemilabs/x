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
	"github.com/hemilabs/x/tss-lib/v2/crypto/mta"
	"github.com/hemilabs/x/tss-lib/v2/ecdsa/keygen"
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

		keys keygen.LocalPartySaveData
		temp localTempData
		data *common.SignatureData

		// outbound messaging
		out chan<- tss.Message
		end chan<- *common.SignatureData
	}

	localMessageStore struct {
		signRound1Message1s,
		signRound1Message2s,
		signRound2Messages,
		signRound3Messages,
		signRound4Messages,
		signRound5Messages,
		signRound6Messages,
		signRound7Messages,
		signRound8Messages,
		signRound9Messages []tss.ParsedMessage
	}

	localTempData struct {
		localMessageStore

		// temp data (thrown away after sign) / round 1
		w,
		m,
		k,
		theta,
		thetaInverse,
		sigma,
		keyDerivationDelta,
		gamma *big.Int
		fullBytesLen int
		cis          []*big.Int
		bigWs        []*crypto.ECPoint
		pointGamma   *crypto.ECPoint
		deCommit     cmt.HashDeCommitment

		// round 2
		betas, // return value of Bob_mid
		c1jis,
		c2jis,
		vs []*big.Int // return value of Bob_mid_wc
		pi1jis []*mta.ProofBob
		pi2jis []*mta.ProofBobWC

		// round 5
		li,
		si,
		rx,
		ry,
		roi *big.Int
		bigR,
		bigAi,
		bigVi *crypto.ECPoint
		DPower cmt.HashDeCommitment

		// round 7
		Ui,
		Ti *crypto.ECPoint
		DTelda cmt.HashDeCommitment

		ssidNonce *big.Int
		ssid      []byte
	}
)

func NewLocalParty(
	msg *big.Int,
	params *tss.Parameters,
	key keygen.LocalPartySaveData,
	out chan<- tss.Message,
	end chan<- *common.SignatureData,
	fullBytesLen ...int) tss.Party {
	return NewLocalPartyWithKDD(msg, params, key, nil, out, end, fullBytesLen...)
}

// NewLocalPartyWithKDD returns a party with key derivation delta for HD support
func NewLocalPartyWithKDD(
	msg *big.Int,
	params *tss.Parameters,
	key keygen.LocalPartySaveData,
	keyDerivationDelta *big.Int,
	out chan<- tss.Message,
	end chan<- *common.SignatureData,
	fullBytesLen ...int,
) tss.Party {
	// [FORK] Nil guard: upstream silently accepts nil msg, which would panic later in
	// round 1 when computing the hash. Fail-fast here with a clear error message.
	if msg == nil {
		panic("signing.NewLocalPartyWithKDD: message must not be nil")
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
	p.temp.signRound1Message1s = make([]tss.ParsedMessage, partyCount)
	p.temp.signRound1Message2s = make([]tss.ParsedMessage, partyCount)
	p.temp.signRound2Messages = make([]tss.ParsedMessage, partyCount)
	p.temp.signRound3Messages = make([]tss.ParsedMessage, partyCount)
	p.temp.signRound4Messages = make([]tss.ParsedMessage, partyCount)
	p.temp.signRound5Messages = make([]tss.ParsedMessage, partyCount)
	p.temp.signRound6Messages = make([]tss.ParsedMessage, partyCount)
	p.temp.signRound7Messages = make([]tss.ParsedMessage, partyCount)
	p.temp.signRound8Messages = make([]tss.ParsedMessage, partyCount)
	p.temp.signRound9Messages = make([]tss.ParsedMessage, partyCount)
	// temp data init
	p.temp.keyDerivationDelta = keyDerivationDelta
	p.temp.m = msg
	if len(fullBytesLen) > 0 {
		p.temp.fullBytesLen = fullBytesLen[0]
	} else {
		p.temp.fullBytesLen = 0
	}
	p.temp.cis = make([]*big.Int, partyCount)
	p.temp.bigWs = make([]*crypto.ECPoint, partyCount)
	p.temp.betas = make([]*big.Int, partyCount)
	p.temp.c1jis = make([]*big.Int, partyCount)
	p.temp.c2jis = make([]*big.Int, partyCount)
	p.temp.pi1jis = make([]*mta.ProofBob, partyCount)
	p.temp.pi2jis = make([]*mta.ProofBobWC, partyCount)
	p.temp.vs = make([]*big.Int, partyCount)
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
	if ok, err := p.BaseParty.ValidateMessage(msg); !ok || err != nil {
		return ok, err
	}
	// check that the message's "from index" will fit into the array
	if maxFromIdx := len(p.params.Parties().IDs()) - 1; maxFromIdx < msg.GetFrom().Index {
		return false, p.WrapError(fmt.Errorf("received msg with a sender index too great (%d <= %d)",
			maxFromIdx, msg.GetFrom().Index), msg.GetFrom())
	}
	// [FORK] Key-at-Index verification: upstream only checked index bounds. We additionally
	// verify the sender's Key matches the party registered at the claimed Index, preventing
	// an attacker from spoofing messages with a valid index but a different identity.
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
	// [FORK] Defense-in-depth: reject duplicate messages for the same (round, sender) pair.
	// Upstream overwrites the stored message unconditionally, which breaks commit-then-reveal
	// guarantees (an attacker could send commitment C1, wait for others, then replace with C2).
	// Also validate broadcast/P2P flag at storage time to prevent slot poisoning:
	// a message with the wrong flag would occupy the slot but be rejected by
	// CanAccept(), permanently blocking the round from proceeding.
	switch msg.Content().(type) {
	case *SignRound1Message1: // P2P
		if msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("SignRound1Message1 expected P2P but got broadcast"), msg.GetFrom())
		}
		if p.temp.signRound1Message1s[fromPIdx] != nil {
			common.Logger.Warningf("duplicate SignRound1Message1 from %d ignored", fromPIdx)
			return true, nil
		}
		p.temp.signRound1Message1s[fromPIdx] = msg
	case *SignRound1Message2: // broadcast
		if !msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("SignRound1Message2 expected broadcast but got P2P"), msg.GetFrom())
		}
		if p.temp.signRound1Message2s[fromPIdx] != nil {
			common.Logger.Warningf("duplicate SignRound1Message2 from %d ignored", fromPIdx)
			return true, nil
		}
		p.temp.signRound1Message2s[fromPIdx] = msg
	case *SignRound2Message: // P2P
		if msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("SignRound2Message expected P2P but got broadcast"), msg.GetFrom())
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
	case *SignRound4Message: // broadcast
		if !msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("SignRound4Message expected broadcast but got P2P"), msg.GetFrom())
		}
		if p.temp.signRound4Messages[fromPIdx] != nil {
			common.Logger.Warningf("duplicate SignRound4Message from %d ignored", fromPIdx)
			return true, nil
		}
		p.temp.signRound4Messages[fromPIdx] = msg
	case *SignRound5Message: // broadcast
		if !msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("SignRound5Message expected broadcast but got P2P"), msg.GetFrom())
		}
		if p.temp.signRound5Messages[fromPIdx] != nil {
			common.Logger.Warningf("duplicate SignRound5Message from %d ignored", fromPIdx)
			return true, nil
		}
		p.temp.signRound5Messages[fromPIdx] = msg
	case *SignRound6Message: // broadcast
		if !msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("SignRound6Message expected broadcast but got P2P"), msg.GetFrom())
		}
		if p.temp.signRound6Messages[fromPIdx] != nil {
			common.Logger.Warningf("duplicate SignRound6Message from %d ignored", fromPIdx)
			return true, nil
		}
		p.temp.signRound6Messages[fromPIdx] = msg
	case *SignRound7Message: // broadcast
		if !msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("SignRound7Message expected broadcast but got P2P"), msg.GetFrom())
		}
		if p.temp.signRound7Messages[fromPIdx] != nil {
			common.Logger.Warningf("duplicate SignRound7Message from %d ignored", fromPIdx)
			return true, nil
		}
		p.temp.signRound7Messages[fromPIdx] = msg
	case *SignRound8Message: // broadcast
		if !msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("SignRound8Message expected broadcast but got P2P"), msg.GetFrom())
		}
		if p.temp.signRound8Messages[fromPIdx] != nil {
			common.Logger.Warningf("duplicate SignRound8Message from %d ignored", fromPIdx)
			return true, nil
		}
		p.temp.signRound8Messages[fromPIdx] = msg
	case *SignRound9Message: // broadcast
		if !msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("SignRound9Message expected broadcast but got P2P"), msg.GetFrom())
		}
		if p.temp.signRound9Messages[fromPIdx] != nil {
			common.Logger.Warningf("duplicate SignRound9Message from %d ignored", fromPIdx)
			return true, nil
		}
		p.temp.signRound9Messages[fromPIdx] = msg
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
