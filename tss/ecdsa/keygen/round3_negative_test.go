// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/hemilabs/x/tss/v3/crypto/facproof"
	"github.com/hemilabs/x/tss/v3/crypto/modproof"
	"github.com/hemilabs/x/tss/v3/tss"
)

const (
	negN         = 3
	negThreshold = 1 // 2-of-3
)

// round3TestFixture holds all state needed to call Round3 for a single
// party (party 0) with valid Round2 messages from the other parties.
type round3TestFixture struct {
	states     []*KeygenState
	allR2P2P   [][]*tss.Message // allR2P2P[receiver][sender]
	allR2Bcast []*tss.Message   // allR2Bcast[sender]
}

// setupRound3Fixture runs Round1 + Round2 for a 3-party keygen and
// returns a fixture whose allR2P2P and allR2Bcast are valid messages
// ready for Round3.  The caller can corrupt individual messages before
// invoking Round3.
func setupRound3Fixture(t *testing.T) *round3TestFixture {
	t.Helper()

	preParams := make([]LocalPreParams, negN)
	for i := 0; i < negN; i++ {
		pp, err := GeneratePreParams(5 * time.Minute)
		if err != nil {
			t.Fatalf("GeneratePreParams[%d]: %v", i, err)
		}
		preParams[i] = *pp
	}

	pIDs := tss.GenerateTestPartyIDs(negN)
	peerCtx := tss.NewPeerContext(pIDs)

	// -- Round 1 --
	states := make([]*KeygenState, negN)
	allR1 := make([]*tss.Message, negN)
	for i := 0; i < negN; i++ {
		params := tss.NewParameters(tss.S256(), peerCtx, pIDs[i], negN, negThreshold)
		st, out, err := Round1(context.Background(), params, preParams[i])
		if err != nil {
			t.Fatalf("Round1[%d]: %v", i, err)
		}
		states[i] = st
		allR1[i] = out.Messages[0]
	}

	// -- Round 2 --
	r2Outputs := make([]*RoundOutput, negN)
	for i := 0; i < negN; i++ {
		out, err := Round2(context.Background(), states[i], allR1)
		if err != nil {
			t.Fatalf("Round2[%d]: %v", i, err)
		}
		r2Outputs[i] = out
	}

	// Route Round2 messages.
	allR2P2P := make([][]*tss.Message, negN)
	allR2Bcast := make([]*tss.Message, negN)
	for i := 0; i < negN; i++ {
		allR2P2P[i] = make([]*tss.Message, negN)
	}
	for sender := 0; sender < negN; sender++ {
		for _, msg := range r2Outputs[sender].Messages {
			if msg.To == nil {
				allR2Bcast[sender] = msg
			} else {
				for _, to := range msg.To {
					allR2P2P[to.Index][sender] = msg
				}
			}
		}
		// Own P2P and broadcast self-messages from state.
		allR2P2P[sender][sender] = states[sender].ExportR2P2PSelf()
		if allR2Bcast[sender] == nil {
			allR2Bcast[sender] = states[sender].ExportR2BcastSelf()
		}
	}

	return &round3TestFixture{
		states:     states,
		allR2P2P:   allR2P2P,
		allR2Bcast: allR2Bcast,
	}
}

// cloneP2PMsg creates a shallow copy of a P2P message with a cloned
// KGRound2Message1 content, so mutations do not affect the original.
func cloneP2PMsg(orig *tss.Message) *tss.Message {
	content := orig.Content.(*KGRound2Message1)
	cloned := &KGRound2Message1{
		Share:      new(big.Int).Set(content.Share),
		FacProof:   content.FacProof,
		ReceiverID: make([]byte, len(content.ReceiverID)),
	}
	copy(cloned.ReceiverID, content.ReceiverID)
	return &tss.Message{
		From:        orig.From,
		To:          orig.To,
		IsBroadcast: orig.IsBroadcast,
		Content:     cloned,
	}
}

// cloneBcastMsg creates a shallow copy of a broadcast message with a
// cloned KGRound2Message2 content.
func cloneBcastMsg(orig *tss.Message) *tss.Message {
	content := orig.Content.(*KGRound2Message2)
	dcmt := make([]*big.Int, len(content.DeCommitment))
	for i, v := range content.DeCommitment {
		if v != nil {
			dcmt[i] = new(big.Int).Set(v)
		}
	}
	cloned := &KGRound2Message2{
		DeCommitment: dcmt,
		ModProof:     content.ModProof,
	}
	return &tss.Message{
		From:        orig.From,
		To:          orig.To,
		IsBroadcast: orig.IsBroadcast,
		Content:     cloned,
	}
}

// copyP2PSlice returns a deep copy of a []*tss.Message slice for P2P
// messages targeting a single receiver.
func copyP2PSlice(msgs []*tss.Message) []*tss.Message {
	out := make([]*tss.Message, len(msgs))
	for i, m := range msgs {
		if m != nil {
			out[i] = cloneP2PMsg(m)
		}
	}
	return out
}

// copyBcastSlice returns a deep copy of the broadcast message slice.
func copyBcastSlice(msgs []*tss.Message) []*tss.Message {
	out := make([]*tss.Message, len(msgs))
	for i, m := range msgs {
		if m != nil {
			out[i] = cloneBcastMsg(m)
		}
	}
	return out
}

// requireRound3Error calls Round3 for party 0 and asserts that it
// returns an error containing the expected substring and identifies
// the correct culprit party.
func requireRound3Error(t *testing.T, fix *round3TestFixture, p2p []*tss.Message, bcast []*tss.Message, wantSubstr string, wantCulpritIdx int) {
	t.Helper()
	_, err := Round3(context.Background(), fix.states[0], p2p, bcast)
	if err == nil {
		t.Fatalf("expected Round3 to fail with %q, but it succeeded", wantSubstr)
	}
	if !strings.Contains(err.Error(), wantSubstr) {
		t.Fatalf("expected error containing %q, got: %v", wantSubstr, err)
	}
	var tssErr *tss.Error
	if ok := isError(err, &tssErr); !ok {
		t.Fatal("expected a *tss.Error with culprit information")
	}
	culprits := tssErr.Culprits()
	if len(culprits) != 1 || culprits[0].Index != wantCulpritIdx {
		t.Fatalf("expected culprit index %d, got: %v", wantCulpritIdx, culprits)
	}
}

// ---------------------------------------------------------------------------
// Test 1: ReceiverID mismatch
// ---------------------------------------------------------------------------

func TestRound3RejectsReceiverIDMismatch(t *testing.T) {
	fix := setupRound3Fixture(t)

	// Tamper: change sender 1's P2P ReceiverID to garbage.
	p2p := copyP2PSlice(fix.allR2P2P[0])
	bcast := copyBcastSlice(fix.allR2Bcast)
	content := p2p[1].Content.(*KGRound2Message1)
	content.ReceiverID = []byte("wrong-receiver-id")

	requireRound3Error(t, fix, p2p, bcast, "receiverId mismatch", 1)
}

// ---------------------------------------------------------------------------
// Test 2: Bad decommitment
// ---------------------------------------------------------------------------

func TestRound3RejectsBadDeCommitment(t *testing.T) {
	fix := setupRound3Fixture(t)

	p2p := copyP2PSlice(fix.allR2P2P[0])
	bcast := copyBcastSlice(fix.allR2Bcast)

	// Corrupt one element of sender 1's decommitment.
	content := bcast[1].Content.(*KGRound2Message2)
	if len(content.DeCommitment) > 1 {
		content.DeCommitment[1] = new(big.Int).Add(content.DeCommitment[1], big.NewInt(1))
	} else {
		t.Fatal("decommitment too short to corrupt")
	}

	requireRound3Error(t, fix, p2p, bcast, "de-commitment verify failed", 1)
}

// ---------------------------------------------------------------------------
// Test 3: Nil ModProof (proofs enabled)
// ---------------------------------------------------------------------------

func TestRound3RejectsNilModProof(t *testing.T) {
	fix := setupRound3Fixture(t)

	p2p := copyP2PSlice(fix.allR2P2P[0])
	bcast := copyBcastSlice(fix.allR2Bcast)

	// Set sender 1's ModProof to nil.
	content := bcast[1].Content.(*KGRound2Message2)
	content.ModProof = nil

	requireRound3Error(t, fix, p2p, bcast, "modProof missing", 1)
}

// ---------------------------------------------------------------------------
// Test 4: Bad ModProof (corrupted W)
// ---------------------------------------------------------------------------

func TestRound3RejectsBadModProof(t *testing.T) {
	fix := setupRound3Fixture(t)

	p2p := copyP2PSlice(fix.allR2P2P[0])
	bcast := copyBcastSlice(fix.allR2Bcast)

	// Corrupt the W field of sender 1's ModProof.
	content := bcast[1].Content.(*KGRound2Message2)
	corrupted := &modproof.ProofMod{
		W: new(big.Int).Add(content.ModProof.W, big.NewInt(1)),
		A: content.ModProof.A,
		B: content.ModProof.B,
	}
	corrupted.X = content.ModProof.X
	corrupted.Z = content.ModProof.Z
	content.ModProof = corrupted

	requireRound3Error(t, fix, p2p, bcast, "modProof verify failed", 1)
}

// ---------------------------------------------------------------------------
// Test 5: Nil FacProof (proofs enabled)
// ---------------------------------------------------------------------------

func TestRound3RejectsNilFacProof(t *testing.T) {
	fix := setupRound3Fixture(t)

	p2p := copyP2PSlice(fix.allR2P2P[0])
	bcast := copyBcastSlice(fix.allR2Bcast)

	// Set sender 1's FacProof to nil.
	content := p2p[1].Content.(*KGRound2Message1)
	content.FacProof = nil

	requireRound3Error(t, fix, p2p, bcast, "facProof missing", 1)
}

// ---------------------------------------------------------------------------
// Test 6: Bad FacProof (corrupted P field)
// ---------------------------------------------------------------------------

func TestRound3RejectsBadFacProof(t *testing.T) {
	fix := setupRound3Fixture(t)

	p2p := copyP2PSlice(fix.allR2P2P[0])
	bcast := copyBcastSlice(fix.allR2Bcast)

	// Corrupt the P field of sender 1's FacProof.
	content := p2p[1].Content.(*KGRound2Message1)
	orig := content.FacProof
	content.FacProof = &facproof.ProofFac{
		P:     new(big.Int).Add(orig.P, big.NewInt(1)),
		Q:     orig.Q,
		A:     orig.A,
		B:     orig.B,
		T:     orig.T,
		Sigma: orig.Sigma,
		Z1:    orig.Z1,
		Z2:    orig.Z2,
		W1:    orig.W1,
		W2:    orig.W2,
		V:     orig.V,
	}

	requireRound3Error(t, fix, p2p, bcast, "facProof verify failed", 1)
}

// ---------------------------------------------------------------------------
// Test 7: Bad VSS share
// ---------------------------------------------------------------------------

func TestRound3RejectsBadVSSShare(t *testing.T) {
	fix := setupRound3Fixture(t)

	p2p := copyP2PSlice(fix.allR2P2P[0])
	bcast := copyBcastSlice(fix.allR2Bcast)

	// Corrupt the VSS share from sender 1.
	content := p2p[1].Content.(*KGRound2Message1)
	content.Share = new(big.Int).Add(content.Share, big.NewInt(1))

	requireRound3Error(t, fix, p2p, bcast, "vss verify failed", 1)
}

// TestRound3RejectsContextCancellation verifies that Round3 returns
// a context error when the context is pre-cancelled.
func TestRound3RejectsContextCancellation(t *testing.T) {
	fix := setupRound3Fixture(t)

	p2p := copyP2PSlice(fix.allR2P2P[0])
	bcast := copyBcastSlice(fix.allR2Bcast)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	_, err := Round3(ctx, fix.states[0], p2p, bcast)
	if err == nil {
		t.Fatal("expected Round3 to fail with cancelled context")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected 'context canceled' error, got: %v", err)
	}
}
