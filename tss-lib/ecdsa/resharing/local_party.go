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
	"github.com/hemilabs/x/tss-lib/v2/ecdsa/keygen"
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
		dgRound2Message1s,
		dgRound2Message2s,
		dgRound3Message1s,
		dgRound3Message2s,
		dgRound4Message1s,
		dgRound4Message2s []tss.ParsedMessage
	}

	localTempData struct {
		localMessageStore

		// temp data (thrown away after rounds)
		NewVs     vss.Vs
		NewShares vss.Shares
		// [FORK] Store VSS polynomial coefficients for SNARK witness extraction.
		// Upstream does not expose the polynomial; we need it so the SP1 per-participant
		// prover can reconstruct the party's secret share commitment during resharing.
		Poly []*big.Int
		VD        cmt.HashDeCommitment

		// temporary storage of data that is persisted by the new party in round 5 if all "ACK" messages are received
		newXi     *big.Int
		newKs     []*big.Int
		newBigXjs []*crypto.ECPoint // Xj to save in round 5

		ssid      []byte
		ssidNonce *big.Int
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
	p.temp.dgRound1Messages = make([]tss.ParsedMessage, oldPartyCount)           // from t+1 of Old Committee
	p.temp.dgRound2Message1s = make([]tss.ParsedMessage, params.NewPartyCount()) // from n of New Committee
	p.temp.dgRound2Message2s = make([]tss.ParsedMessage, params.NewPartyCount()) // "
	p.temp.dgRound3Message1s = make([]tss.ParsedMessage, oldPartyCount)          // from t+1 of Old Committee
	p.temp.dgRound3Message2s = make([]tss.ParsedMessage, oldPartyCount)          // "
	p.temp.dgRound4Message1s = make([]tss.ParsedMessage, params.NewPartyCount()) // from n of New Committee
	p.temp.dgRound4Message2s = make([]tss.ParsedMessage, params.NewPartyCount()) // from n of New Committee
	// save data init
	if key.LocalPreParams.ValidateWithProof() {
		p.save.LocalPreParams = key.LocalPreParams
	}
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
	case *DGRound2Message1, *DGRound2Message2, *DGRound4Message1, *DGRound4Message2:
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
	// verify that the sender's Key matches the party registered at the claimed Index. Without
	// this, an attacker could impersonate another party by sending a valid index with a
	// different Key, causing messages to be stored under the wrong party's slot.
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
	// Upstream would silently overwrite stored messages, which breaks commit-then-reveal
	// guarantees (an attacker could replace a commitment after seeing the decommitment).
	// We also validate the broadcast/P2P flag at storage time to prevent slot poisoning
	// (a P2P message stored in a broadcast slot or vice versa).
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
	case *DGRound2Message1: // broadcast
		if !msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("DGRound2Message1 expected broadcast but got P2P"), msg.GetFrom())
		}
		if p.temp.dgRound2Message1s[fromPIdx] != nil {
			common.Logger.Warningf("duplicate DGRound2Message1 from %d ignored", fromPIdx)
			return true, nil
		}
		p.temp.dgRound2Message1s[fromPIdx] = msg
	case *DGRound2Message2: // broadcast
		if !msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("DGRound2Message2 expected broadcast but got P2P"), msg.GetFrom())
		}
		if p.temp.dgRound2Message2s[fromPIdx] != nil {
			common.Logger.Warningf("duplicate DGRound2Message2 from %d ignored", fromPIdx)
			return true, nil
		}
		p.temp.dgRound2Message2s[fromPIdx] = msg
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
	case *DGRound4Message1: // P2P
		if msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("DGRound4Message1 expected P2P but got broadcast"), msg.GetFrom())
		}
		if p.temp.dgRound4Message1s[fromPIdx] != nil {
			common.Logger.Warningf("duplicate DGRound4Message1 from %d ignored", fromPIdx)
			return true, nil
		}
		p.temp.dgRound4Message1s[fromPIdx] = msg
	case *DGRound4Message2: // broadcast
		if !msg.IsBroadcast() {
			return false, p.WrapError(fmt.Errorf("DGRound4Message2 expected broadcast but got P2P"), msg.GetFrom())
		}
		if p.temp.dgRound4Message2s[fromPIdx] != nil {
			common.Logger.Warningf("duplicate DGRound4Message2 from %d ignored", fromPIdx)
			return true, nil
		}
		p.temp.dgRound4Message2s[fromPIdx] = msg
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

// [FORK] GetPoly returns the VSS polynomial coefficients stored during Round 1.
// Only populated for old committee members after Round 1 completes.
// Returns nil for new committee members or if Round 1 has not run.
// This method does not exist in upstream; it is used by the SP1 per-participant
// prover for witness extraction during resharing ceremonies.
func (p *LocalParty) GetPoly() []*big.Int {
	return p.temp.Poly
}

// [FORK] GetNewVs returns the Feldman VSS commitments (V[0..t_new]) stored during Round 1.
// Only populated for old committee members after Round 1 completes. This method does not
// exist in upstream; it is used alongside GetPoly() for SNARK witness construction.
func (p *LocalParty) GetNewVs() []*crypto.ECPoint {
	return p.temp.NewVs
}
