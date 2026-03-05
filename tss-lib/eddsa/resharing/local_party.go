// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package resharing

import (
	"fmt"
	"math/big"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/crypto"
	cmt "github.com/hemilabs/x/tss-lib/v2/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v2/crypto/vss"
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
		params *tss.ReSharingParameters

		temp        localTempData
		input, save keygen.LocalPartySaveData

		// outbound messaging
		out chan<- tss.Message
		end chan<- *keygen.LocalPartySaveData
	}

	localMessageStore struct {
		dgRound1Messages,
		dgRound2Messages,
		dgRound3Message1s,
		dgRound3Message2s,
		dgRound4Messages []tss.ParsedMessage
	}

	localTempData struct {
		localMessageStore

		// [FORK] Session identifier: upstream had no SSID in resharing. Added for ZK proof
		// domain separation and to bind resharing sessions to their exact configuration.
		ssid      []byte
		ssidNonce *big.Int

		// temp data (thrown away after rounds)
		NewVs     vss.Vs
		NewShares vss.Shares
		VD        cmt.HashDeCommitment

		// temporary storage of data that is persisted by the new party in round 5 if all "ACK" messages are received
		newXi     *big.Int
		newKs     []*big.Int
		newBigXjs []*crypto.ECPoint // Xj to save in round 5
	}
)

// Exported, used in `tss` client
// The `key` is read from and/or written to depending on whether this party is part of the old or the new committee.
// You may optionally generate and set the LocalPreParams if you would like to use pre-generated safe primes and Paillier secret.
// (This is similar to providing the `optionalPreParams` to `keygen.LocalParty`).
func NewLocalParty(
	params *tss.ReSharingParameters,
	key keygen.LocalPartySaveData,
	out chan<- tss.Message,
	end chan<- *keygen.LocalPartySaveData,
) tss.Party {
	oldPartyCount := len(params.OldParties().IDs())
	subset := key
	if params.IsOldCommittee() {
		subset = keygen.BuildLocalSaveDataSubset(key, params.OldParties().IDs())
	}
	p := &LocalParty{
		BaseParty: new(tss.BaseParty),
		params:    params,
		temp:      localTempData{},
		input:     subset,
		save:      keygen.NewLocalPartySaveData(params.NewPartyCount()),
		out:       out,
		end:       end,
	}
	// msgs init
	p.temp.dgRound1Messages = make([]tss.ParsedMessage, oldPartyCount)          // from t+1 of Old Committee
	p.temp.dgRound2Messages = make([]tss.ParsedMessage, params.NewPartyCount()) // from n of New Committee
	p.temp.dgRound3Message1s = make([]tss.ParsedMessage, oldPartyCount)         // from t+1 of Old Committee
	p.temp.dgRound3Message2s = make([]tss.ParsedMessage, oldPartyCount)         // "
	p.temp.dgRound4Messages = make([]tss.ParsedMessage, params.NewPartyCount()) // from n of New Committee

	return p
}

func (p *LocalParty) FirstRound() tss.Round {
	return newRound1(p.params, &p.input, &p.save, &p.temp, p.out, p.end)
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
	var partyIDs tss.SortedPartyIDs
	switch msg.Content().(type) {
	case *DGRound2Message, *DGRound4Message:
		partyIDs = p.params.NewParties().IDs()
	default:
		partyIDs = p.params.OldParties().IDs()
	}
	maxFromIdx := len(partyIDs) - 1
	if maxFromIdx < msg.GetFrom().Index {
		return false, p.WrapError(fmt.Errorf("received msg with a sender index too great (%d <= %d)",
			maxFromIdx, msg.GetFrom().Index), msg.GetFrom())
	}
	// [FORK] Key-at-Index verification: upstream only checked index bounds. We additionally
	// verify the sender's Key matches the party registered at the claimed Index to prevent
	// impersonation. In resharing, we look up from the correct committee (old vs new) based
	// on message type.
	knownParty := partyIDs[msg.GetFrom().Index]
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
	// Upstream did not handle replays, leaving it to the caller. We enforce dedup here because
	// overwriting a stored message breaks commit-then-reveal guarantees. We also validate the
	// broadcast/P2P flag at storage time to prevent slot poisoning.
	switch msg.Content().(type) {
	case *DGRound1Message: // broadcast
		if !msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("DGRound1Message expected broadcast but got P2P"), msg.GetFrom())
		}
		if p.temp.dgRound1Messages[fromPIdx] != nil {
			common.Logger.Warningf("duplicate DGRound1Message from %d ignored", fromPIdx)
			return true, nil
		}
		p.temp.dgRound1Messages[fromPIdx] = msg
	case *DGRound2Message: // broadcast
		if !msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("DGRound2Message expected broadcast but got P2P"), msg.GetFrom())
		}
		if p.temp.dgRound2Messages[fromPIdx] != nil {
			common.Logger.Warningf("duplicate DGRound2Message from %d ignored", fromPIdx)
			return true, nil
		}
		p.temp.dgRound2Messages[fromPIdx] = msg
	case *DGRound3Message1: // P2P
		if msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("DGRound3Message1 expected P2P but got broadcast"), msg.GetFrom())
		}
		if p.temp.dgRound3Message1s[fromPIdx] != nil {
			common.Logger.Warningf("duplicate DGRound3Message1 from %d ignored", fromPIdx)
			return true, nil
		}
		p.temp.dgRound3Message1s[fromPIdx] = msg
	case *DGRound3Message2: // broadcast
		if !msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("DGRound3Message2 expected broadcast but got P2P"), msg.GetFrom())
		}
		if p.temp.dgRound3Message2s[fromPIdx] != nil {
			common.Logger.Warningf("duplicate DGRound3Message2 from %d ignored", fromPIdx)
			return true, nil
		}
		p.temp.dgRound3Message2s[fromPIdx] = msg
	case *DGRound4Message: // broadcast
		if !msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("DGRound4Message expected broadcast but got P2P"), msg.GetFrom())
		}
		if p.temp.dgRound4Messages[fromPIdx] != nil {
			common.Logger.Warningf("duplicate DGRound4Message from %d ignored", fromPIdx)
			return true, nil
		}
		p.temp.dgRound4Messages[fromPIdx] = msg
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
