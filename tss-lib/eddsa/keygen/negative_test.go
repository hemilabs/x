// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss-lib/v3/crypto/schnorr"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// runEdDSAKeygen runs a full 3-party keygen, returning states, round messages.
func runEdDSAKeygen(t *testing.T) (
	states []*KeygenState,
	r1 []*tss.Message,
	r2p2p [][]*tss.Message,
	r2bcast []*tss.Message,
	pIDs tss.SortedPartyIDs,
) {
	t.Helper()
	const n = 3
	const threshold = 1

	pIDs = tss.GenerateTestPartyIDs(n)
	peerCtx := tss.NewPeerContext(pIDs)

	states = make([]*KeygenState, n)
	r1 = make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.Edwards(), peerCtx, pIDs[i], n, threshold)
		st, out, err := Round1(params)
		if err != nil {
			t.Fatalf("Round1[%d]: %v", i, err)
		}
		states[i] = st
		r1[i] = out.Messages[0]
	}

	r2p2p = make([][]*tss.Message, n)
	r2bcast = make([]*tss.Message, n)
	for i := range r2p2p {
		r2p2p[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := Round2(states[i], r1)
		if err != nil {
			t.Fatalf("Round2[%d]: %v", i, err)
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
	return
}

func TestRound2InvalidR1Message(t *testing.T) {
	const n = 3
	const threshold = 1
	pIDs := tss.GenerateTestPartyIDs(n)
	peerCtx := tss.NewPeerContext(pIDs)

	states := make([]*KeygenState, n)
	r1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.Edwards(), peerCtx, pIDs[i], n, threshold)
		st, out, err := Round1(params)
		if err != nil {
			t.Fatalf("Round1[%d]: %v", i, err)
		}
		states[i] = st
		r1[i] = out.Messages[0]
	}

	// Corrupt r1[1]: nil commitment
	r1Bad := make([]*tss.Message, n)
	copy(r1Bad, r1)
	r1Bad[1] = &tss.Message{
		From:    r1[1].From,
		Content: &KGRound1Message{Commitment: nil}, // invalid
	}

	_, err := Round2(states[0], r1Bad)
	if err == nil {
		t.Fatal("expected error for invalid round 1 message")
	}
}

func TestRound3BadDeCommitment(t *testing.T) {
	states, _, r2p2p, r2bcast, _ := runEdDSAKeygen(t)

	// Corrupt decommitment for party 1 → should fail party 0's Round3
	badBcast := make([]*tss.Message, len(r2bcast))
	copy(badBcast, r2bcast)
	badContent := *r2bcast[1].Content.(*KGRound2Message2)
	badContent.DeCommitment = nil
	badBcast[1] = &tss.Message{From: r2bcast[1].From, Content: &badContent}

	_, err := Round3(states[0], r2p2p[0], badBcast)
	if err == nil {
		t.Fatal("expected error for bad decommitment")
	}
}

func TestRound3MissingSchnorrProof(t *testing.T) {
	states, _, r2p2p, r2bcast, _ := runEdDSAKeygen(t)

	badBcast := make([]*tss.Message, len(r2bcast))
	copy(badBcast, r2bcast)
	badContent := *r2bcast[1].Content.(*KGRound2Message2)
	badContent.ZKProof = nil
	badBcast[1] = &tss.Message{From: r2bcast[1].From, Content: &badContent}

	_, err := Round3(states[0], r2p2p[0], badBcast)
	if err == nil {
		t.Fatal("expected error for missing schnorr proof")
	}
}

func TestRound3WrongSchnorrProof(t *testing.T) {
	states, _, r2p2p, r2bcast, _ := runEdDSAKeygen(t)

	badBcast := make([]*tss.Message, len(r2bcast))
	copy(badBcast, r2bcast)
	badContent := *r2bcast[1].Content.(*KGRound2Message2)
	// Replace with a random proof that won't verify
	badContent.ZKProof = &schnorr.ZKProof{Alpha: nil, T: big.NewInt(42)}
	badBcast[1] = &tss.Message{From: r2bcast[1].From, Content: &badContent}

	_, err := Round3(states[0], r2p2p[0], badBcast)
	if err == nil {
		t.Fatal("expected error for wrong schnorr proof")
	}
}

func TestRound3WrongReceiverID(t *testing.T) {
	states, _, r2p2p, r2bcast, _ := runEdDSAKeygen(t)

	// Corrupt receiverID on the P2P message from party 1 to party 0
	badP2P := make([]*tss.Message, len(r2p2p[0]))
	copy(badP2P, r2p2p[0])
	badContent := *r2p2p[0][1].Content.(*KGRound2Message1)
	badContent.ReceiverID = []byte("wrong")
	badP2P[1] = &tss.Message{From: r2p2p[0][1].From, Content: &badContent}

	_, err := Round3(states[0], badP2P, r2bcast)
	if err == nil {
		t.Fatal("expected error for wrong receiverID")
	}
}

func TestRound3BadVSSShare(t *testing.T) {
	states, _, r2p2p, r2bcast, _ := runEdDSAKeygen(t)

	// Corrupt share value from party 1 → party 0
	badP2P := make([]*tss.Message, len(r2p2p[0]))
	copy(badP2P, r2p2p[0])
	badContent := *r2p2p[0][1].Content.(*KGRound2Message1)
	badContent.Share = new(big.Int).Add(badContent.Share, big.NewInt(999))
	badP2P[1] = &tss.Message{From: r2p2p[0][1].From, Content: &badContent}

	_, err := Round3(states[0], badP2P, r2bcast)
	if err == nil {
		t.Fatal("expected error for bad VSS share")
	}
}

func TestValidateSaveDataEdDSA(t *testing.T) {
	states, _, r2p2p, r2bcast, _ := runEdDSAKeygen(t)

	out, err := Round3(states[0], r2p2p[0], r2bcast)
	if err != nil {
		t.Fatalf("Round3: %v", err)
	}
	if err := out.Save.ValidateSaveData(); err != nil {
		t.Fatalf("ValidateSaveData should pass: %v", err)
	}

	// Test each failure path
	bad := *out.Save
	bad.EDDSAPub = nil
	if err := bad.ValidateSaveData(); err == nil {
		t.Fatal("expected error for nil EDDSAPub")
	}
	bad = *out.Save
	bad.Xi = nil
	if err := bad.ValidateSaveData(); err == nil {
		t.Fatal("expected error for nil Xi")
	}
	bad = *out.Save
	bad.Xi = big.NewInt(0)
	if err := bad.ValidateSaveData(); err == nil {
		t.Fatal("expected error for zero Xi")
	}
	bad = *out.Save
	bad.ShareID = nil
	if err := bad.ValidateSaveData(); err == nil {
		t.Fatal("expected error for nil ShareID")
	}
	bad = *out.Save
	bad.Ks = nil
	if err := bad.ValidateSaveData(); err == nil {
		t.Fatal("expected error for nil Ks")
	}
	bad = *out.Save
	bad.BigXj = nil
	if err := bad.ValidateSaveData(); err == nil {
		t.Fatal("expected error for nil BigXj")
	}
}

func TestBuildLocalSaveDataSubsetEdDSA(t *testing.T) {
	states, _, r2p2p, r2bcast, pIDs := runEdDSAKeygen(t)

	out, err := Round3(states[0], r2p2p[0], r2bcast)
	if err != nil {
		t.Fatalf("Round3: %v", err)
	}

	// Subset to 2-of-3
	subset := BuildLocalSaveDataSubset(*out.Save, pIDs[:2])
	if len(subset.Ks) != 2 {
		t.Fatalf("expected 2 Ks, got %d", len(subset.Ks))
	}

	// Missing key should panic
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic for missing signer key")
			}
		}()
		fakeIDs := tss.GenerateTestPartyIDs(1)
		BuildLocalSaveDataSubset(*out.Save, fakeIDs)
	}()
}
