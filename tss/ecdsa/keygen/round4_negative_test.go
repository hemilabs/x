// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/hemilabs/x/tss/v3/crypto/paillier"
	"github.com/hemilabs/x/tss/v3/tss"
)

// fullKeygenFixture holds state after running rounds 1-3 for all n parties,
// with all expensive proofs disabled (DLN, MOD, FAC) for test speed.
// The caller can corrupt allR3 entries before calling Round4.
type fullKeygenFixture struct {
	states []*KeygenState
	allR3  []*tss.Message // allR3[j] = party j's round 3 broadcast
	n      int
}

// setupRound1Through3 runs a complete 3-party keygen through rounds 1-3
// with all proof flags disabled.  Returns a fixture ready for Round4.
func setupRound1Through3(t *testing.T) *fullKeygenFixture {
	t.Helper()
	const n = 3
	const threshold = 1 // 2-of-3

	preParams := loadTestPreParams(t, n)

	pIDs := tss.GenerateTestPartyIDs(n)
	peerCtx := tss.NewPeerContext(pIDs)

	// -- Round 1 --
	states := make([]*KeygenState, n)
	allR1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.S256(), peerCtx, pIDs[i], n, threshold)
		params.SetNoProofDLN()
		params.SetNoProofMod()
		params.SetNoProofFac()
		st, out, err := Round1(context.Background(), params, preParams[i])
		if err != nil {
			t.Fatalf("Round1[%d]: %v", i, err)
		}
		states[i] = st
		allR1[i] = out.Messages[0]
	}

	// -- Round 2 --
	r2P2P := make([][]*tss.Message, n)
	r2Bcast := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		r2P2P[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := Round2(context.Background(), states[i], allR1)
		if err != nil {
			t.Fatalf("Round2[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			if msg.To == nil {
				r2Bcast[i] = msg
			} else {
				for _, to := range msg.To {
					r2P2P[to.Index][i] = msg
				}
			}
		}
		r2P2P[i][i] = states[i].ExportR2P2PSelf()
		if r2Bcast[i] == nil {
			r2Bcast[i] = states[i].ExportR2BcastSelf()
		}
	}

	// -- Round 3 --
	allR3 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := Round3(context.Background(), states[i], r2P2P[i], r2Bcast)
		if err != nil {
			t.Fatalf("Round3[%d]: %v", i, err)
		}
		allR3[i] = out.Messages[0]
	}

	return &fullKeygenFixture{
		states: states,
		allR3:  allR3,
		n:      n,
	}
}

// TestRound4RejectsBadPaillierProof runs rounds 1-3 honestly, then
// corrupts party 1's Paillier proof in the round 3 message and
// asserts that Round4 (run by party 0) returns a "paillier verify
// failed" error with party 1 identified as the culprit.
func TestRound4RejectsBadPaillierProof(t *testing.T) {
	fix := setupRound1Through3(t)

	// Corrupt party 1's Paillier proof: replace each proof element
	// with a random big.Int that will not satisfy pi^N == xi mod N.
	corruptIdx := 1
	msgs := make([]*tss.Message, fix.n)
	copy(msgs, fix.allR3)

	origMsg := msgs[corruptIdx]
	origContent := origMsg.Content.(*KGRound3Message)

	// Create a corrupted proof by flipping bits in each element.
	var badProof paillier.Proof
	for i := 0; i < paillier.ProofIters; i++ {
		if origContent.PaillierProof[i] != nil {
			corrupted := new(big.Int).Set(origContent.PaillierProof[i])
			corrupted.Add(corrupted, big.NewInt(1)) // shift by 1 to break the proof
			badProof[i] = corrupted
		} else {
			badProof[i] = big.NewInt(42)
		}
	}

	msgs[corruptIdx] = &tss.Message{
		From:        origMsg.From,
		IsBroadcast: true,
		Content: &KGRound3Message{
			PaillierProof: badProof,
		},
	}

	// Run Round4 as party 0.
	_, err := Round4(context.Background(), fix.states[0], msgs)
	if err == nil {
		t.Fatal("expected Round4 to reject corrupted Paillier proof, got nil error")
	}
	if !strings.Contains(err.Error(), "paillier verify failed") {
		t.Fatalf("expected 'paillier verify failed' error, got: %v", err)
	}

	// Verify the culprit is party 1.
	var tssErr *tss.Error
	if ok := isError(err, &tssErr); !ok {
		t.Fatal("expected a *tss.Error with culprit information")
	}
	culprits := tssErr.Culprits()
	if len(culprits) == 0 {
		t.Fatal("expected at least one culprit in the error")
	}
	foundCulprit := false
	for _, c := range culprits {
		if c.Index == corruptIdx {
			foundCulprit = true
			break
		}
	}
	if !foundCulprit {
		t.Fatalf("expected party %d as culprit, got culprits: %v", corruptIdx, culprits)
	}
}

// TestRound4RejectsAllNilPaillierProof verifies that Round4 rejects a
// Paillier proof where all elements are nil (malformed message).
func TestRound4RejectsAllNilPaillierProof(t *testing.T) {
	fix := setupRound1Through3(t)

	corruptIdx := 1
	msgs := make([]*tss.Message, fix.n)
	copy(msgs, fix.allR3)

	origMsg := msgs[corruptIdx]

	// All-nil proof elements.
	var nilProof paillier.Proof // zero value: all nils

	msgs[corruptIdx] = &tss.Message{
		From:        origMsg.From,
		IsBroadcast: true,
		Content: &KGRound3Message{
			PaillierProof: nilProof,
		},
	}

	_, err := Round4(context.Background(), fix.states[0], msgs)
	if err == nil {
		t.Fatal("expected Round4 to reject all-nil Paillier proof, got nil error")
	}
	// The error may be "paillier verify failed" (culprit error) or contain
	// an inner error about nil proof elements from paillier.Proof.Verify.
	if !strings.Contains(err.Error(), "paillier") {
		t.Fatalf("expected paillier-related error, got: %v", err)
	}

	// Verify the culprit is party 1 (corruptIdx).
	var tssErr *tss.Error
	if ok := isError(err, &tssErr); !ok {
		t.Fatal("expected a *tss.Error with culprit information")
	}
	culprits := tssErr.Culprits()
	if len(culprits) == 0 {
		t.Fatal("expected at least one culprit in the error")
	}
	foundCulprit := false
	for _, c := range culprits {
		if c.Index == corruptIdx {
			foundCulprit = true
			break
		}
	}
	if !foundCulprit {
		t.Fatalf("expected party %d as culprit, got culprits: %v", corruptIdx, culprits)
	}
}

// TestRound4ContextCancellation verifies that Round4 respects context
// cancellation and returns the context error.
func TestRound4ContextCancellation(t *testing.T) {
	fix := setupRound1Through3(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	_, err := Round4(ctx, fix.states[0], fix.allR3)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected 'context canceled' error, got: %v", err)
	}
}

// TestRound4HonestPassesForAllParties is a positive sanity check:
// after an honest round 1-3, Round4 should succeed for every party.
func TestRound4HonestPassesForAllParties(t *testing.T) {
	fix := setupRound1Through3(t)

	for i := 0; i < fix.n; i++ {
		out, err := Round4(context.Background(), fix.states[i], fix.allR3)
		if err != nil {
			t.Fatalf("Round4[%d]: %v", i, err)
		}
		if out.Save == nil {
			t.Fatalf("Round4[%d]: Save is nil", i)
		}
		if out.Save.ECDSAPub == nil {
			t.Fatalf("Round4[%d]: ECDSAPub is nil", i)
		}
	}
}

// isError is a helper that unwraps err to a *tss.Error via direct type assertion.
func isError(err error, target interface{}) bool {
	tssErr := &tss.Error{}
	ok := errors.As(err, &tssErr)
	if ok {
		*(target.(**tss.Error)) = tssErr
		return true
	}
	return false
}
