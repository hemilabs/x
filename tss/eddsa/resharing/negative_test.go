// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package resharing

import (
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss/v3/crypto"
	"github.com/hemilabs/x/tss/v3/eddsa/keygen"
	"github.com/hemilabs/x/tss/v3/tss"
)

// runKeygenForReshare does a 3-party EdDSA keygen and returns saves + party IDs.
func runKeygenForReshare(t *testing.T) ([]keygen.LocalPartySaveData, tss.SortedPartyIDs) {
	t.Helper()
	const n = 3
	const threshold = 1

	pIDs := tss.GenerateTestPartyIDs(n)
	peerCtx := tss.NewPeerContext(pIDs)

	states := make([]*keygen.KeygenState, n)
	r1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.Edwards(), peerCtx, pIDs[i], n, threshold)
		st, out, err := keygen.Round1(params)
		if err != nil {
			t.Fatalf("keygen Round1[%d]: %v", i, err)
		}
		states[i] = st
		r1[i] = out.Messages[0]
	}

	r2p2p := make([][]*tss.Message, n)
	r2bcast := make([]*tss.Message, n)
	for i := range r2p2p {
		r2p2p[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := keygen.Round2(states[i], r1)
		if err != nil {
			t.Fatalf("keygen Round2[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			if msg.To == nil {
				r2bcast[i] = msg
			} else {
				for _, to := range msg.To {
					r2p2p[to.Index][i] = msg
				}
			}
		}
		r2p2p[i][i] = states[i].ExportR2P2PSelf()
		if r2bcast[i] == nil {
			r2bcast[i] = states[i].ExportR2BcastSelf()
		}
	}

	saves := make([]keygen.LocalPartySaveData, n)
	for i := 0; i < n; i++ {
		out, err := keygen.Round3(states[i], r2p2p[i], r2bcast)
		if err != nil {
			t.Fatalf("keygen Round3[%d]: %v", i, err)
		}
		saves[i] = *out.Save
	}
	return saves, pIDs
}

// --- ReshareRound2 error paths ---

func TestReshareRound2InvalidR1Message(t *testing.T) {
	saves, oldPIDs := runKeygenForReshare(t)
	newPIDs := tss.GenerateTestPartyIDs(3)

	oldCtx := tss.NewPeerContext(oldPIDs)
	newCtx := tss.NewPeerContext(newPIDs)

	// Run Round1 for old committee
	oldN := len(oldPIDs)
	newN := len(newPIDs)
	r1Msgs := make([]*tss.Message, oldN)
	var newState *ReshareState

	for i := 0; i < oldN; i++ {
		params := tss.NewReSharingParameters(
			tss.Edwards(), oldCtx, newCtx, oldPIDs[i], oldN, 1, newN, 1)
		st, out, err := ReshareRound1(params, &saves[i])
		if err != nil {
			t.Fatalf("ReshareRound1[%d]: %v", i, err)
		}
		if len(out.Messages) > 0 {
			r1Msgs[i] = out.Messages[0]
		}
		_ = st
	}

	// New committee party
	params := tss.NewReSharingParameters(
		tss.Edwards(), oldCtx, newCtx, newPIDs[0], oldN, 1, newN, 1)
	st, _, err := ReshareRound1(params, nil)
	if err != nil {
		t.Fatalf("ReshareRound1 new: %v", err)
	}
	newState = st

	// Corrupt r1[0]: nil EDDSAPub
	badR1 := make([]*tss.Message, oldN)
	copy(badR1, r1Msgs)
	badR1[0] = &tss.Message{
		From:    r1Msgs[0].From,
		Content: &DGRound1Message{EDDSAPub: nil, VCommitment: big.NewInt(1)},
	}

	_, err = ReshareRound2(newState, badR1)
	if err == nil {
		t.Fatal("expected error for invalid round 1 message")
	}
}

func TestReshareRound2PubKeyMismatch(t *testing.T) {
	saves, oldPIDs := runKeygenForReshare(t)
	newPIDs := tss.GenerateTestPartyIDs(3)

	oldCtx := tss.NewPeerContext(oldPIDs)
	newCtx := tss.NewPeerContext(newPIDs)

	oldN := len(oldPIDs)
	newN := len(newPIDs)
	r1Msgs := make([]*tss.Message, oldN)

	for i := 0; i < oldN; i++ {
		params := tss.NewReSharingParameters(
			tss.Edwards(), oldCtx, newCtx, oldPIDs[i], oldN, 1, newN, 1)
		_, out, err := ReshareRound1(params, &saves[i])
		if err != nil {
			t.Fatalf("ReshareRound1[%d]: %v", i, err)
		}
		if len(out.Messages) > 0 {
			r1Msgs[i] = out.Messages[0]
		}
	}

	// New party
	params := tss.NewReSharingParameters(
		tss.Edwards(), oldCtx, newCtx, newPIDs[0], oldN, 1, newN, 1)
	st, _, err := ReshareRound1(params, nil)
	if err != nil {
		t.Fatalf("ReshareRound1 new: %v", err)
	}

	// Replace r1[1]'s EDDSAPub with a different key
	differentKey := crypto.ScalarBaseMult(tss.Edwards(), big.NewInt(99999))
	badR1 := make([]*tss.Message, oldN)
	copy(badR1, r1Msgs)
	badContent := *r1Msgs[1].Content.(*DGRound1Message)
	badContent.EDDSAPub = differentKey
	badR1[1] = &tss.Message{From: r1Msgs[1].From, Content: &badContent}

	_, err = ReshareRound2(st, badR1)
	if err == nil {
		t.Fatal("expected error for pub key mismatch")
	}
}
