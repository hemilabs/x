// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package signing

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// ---------------------------------------------------------------------------
// SignRound1 negative tests
// ---------------------------------------------------------------------------

// TestSignRound1RejectsNegativeMessage verifies that SignRound1 rejects a
// message hash with a negative value (msg.Sign() < 0).
// Exercises the guard at round_fn.go:99.
func TestSignRound1RejectsNegativeMessage(t *testing.T) {
	keys := doKeygen(t)
	pIDs, peerCtx := setupPartyIDs()
	params := tss.NewParameters(tss.S256(), peerCtx, pIDs[0], testN, testThreshold)

	negMsg := big.NewInt(-1)
	_, _, err := SignRound1(params, keys[0], negMsg, nil, 0)
	if err == nil {
		t.Fatal("expected error for negative message, got nil")
	}
	if !strings.Contains(err.Error(), "hashed message is not valid") {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("correctly rejected negative message: %v", err)
}

// TestSignRound1RejectsMessageEqualToN verifies that SignRound1 rejects a
// message hash equal to the curve order N (must be strictly < N).
// Also tests msg = N+1 for completeness.
// Exercises the guard at round_fn.go:99.
func TestSignRound1RejectsMessageEqualToN(t *testing.T) {
	keys := doKeygen(t)
	pIDs, peerCtx := setupPartyIDs()
	params := tss.NewParameters(tss.S256(), peerCtx, pIDs[0], testN, testThreshold)
	curveOrder := tss.S256().Params().N

	// msg == N
	msgN := new(big.Int).Set(curveOrder)
	_, _, err := SignRound1(params, keys[0], msgN, nil, 0)
	if err == nil {
		t.Fatal("expected error for msg == N, got nil")
	}
	if !strings.Contains(err.Error(), "hashed message is not valid") {
		t.Fatalf("unexpected error for msg == N: %v", err)
	}
	t.Logf("correctly rejected msg == N: %v", err)

	// msg == N+1
	msgN1 := new(big.Int).Add(curveOrder, big.NewInt(1))
	_, _, err = SignRound1(params, keys[0], msgN1, nil, 0)
	if err == nil {
		t.Fatal("expected error for msg == N+1, got nil")
	}
	if !strings.Contains(err.Error(), "hashed message is not valid") {
		t.Fatalf("unexpected error for msg == N+1: %v", err)
	}
	t.Logf("correctly rejected msg == N+1: %v", err)
}

// TestSignRound1RejectsKeyCountMismatch verifies that SignRound1 rejects
// when key.Ks has fewer entries than params.PartyCount().
// Exercises the guard at round_fn.go:110-111.
func TestSignRound1RejectsKeyCountMismatch(t *testing.T) {
	keys := doKeygen(t)
	pIDs, peerCtx := setupPartyIDs()
	params := tss.NewParameters(tss.S256(), peerCtx, pIDs[0], testN, testThreshold)
	m := testMsg()

	// Truncate key data so len(Ks) = 2 but params.PartyCount() = 3.
	// Since 2 < 3, the auto-subset branch is skipped and the
	// "key count != party count" check triggers.
	truncated := keys[0]
	truncated.Ks = truncated.Ks[:2]
	truncated.BigXj = truncated.BigXj[:2]
	truncated.NTildej = truncated.NTildej[:2]
	truncated.H1j = truncated.H1j[:2]
	truncated.H2j = truncated.H2j[:2]
	truncated.PaillierPKs = truncated.PaillierPKs[:2]

	_, _, err := SignRound1(params, truncated, m, nil, 0)
	if err == nil {
		t.Fatal("expected error for key count mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "key count") || !strings.Contains(err.Error(), "party count") {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("correctly rejected key count mismatch: %v", err)
}

// ---------------------------------------------------------------------------
// SignRound2 negative tests
// ---------------------------------------------------------------------------

// TestSignRound2RejectsReceiverIDMismatch verifies that SignRound2 rejects
// a P2P message whose ReceiverID does not match the receiving party's key.
// Exercises the guard at round_fn.go:189-191.
func TestSignRound2RejectsReceiverIDMismatch(t *testing.T) {
	keys := doKeygen(t)
	f := setupSignRound1(t, keys)

	// Deep-clone the P2P messages destined for party 0 so we can corrupt one.
	corruptP2P := CloneP2PSlice(f.R1P2P, CloneR1P2PMsg)

	// Corrupt ReceiverID in the message from sender=1 to recipient=0.
	corruptContent := corruptP2P[0][1].Content.(*SignRound1Message1)
	corruptContent.ReceiverID = []byte("wrong-receiver-id")

	_, err := SignRound2(context.Background(), f.States[0], corruptP2P[0], f.R1Bcast)
	if err == nil {
		t.Fatal("expected error for receiverId mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "receiverId mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
	requireCulprit(t, err, 1)
	t.Logf("correctly rejected receiverId mismatch: %v", err)
}

// TestSignRound2RejectsNilRangeProofAlice verifies that SignRound2 rejects
// a P2P message where RangeProofAlice is nil.
// Exercises the guard at round_fn.go:212-216 and :243-247 (both goroutines).
func TestSignRound2RejectsNilRangeProofAlice(t *testing.T) {
	keys := doKeygen(t)
	f := setupSignRound1(t, keys)

	// Deep-clone the P2P messages destined for party 0 so we can corrupt one.
	corruptP2P := CloneP2PSlice(f.R1P2P, CloneR1P2PMsg)

	// Set RangeProofAlice to nil in the message from sender=1 to recipient=0.
	corruptContent := corruptP2P[0][1].Content.(*SignRound1Message1)
	corruptContent.RangeProofAlice = nil

	_, err := SignRound2(context.Background(), f.States[0], corruptP2P[0], f.R1Bcast)
	if err == nil {
		t.Fatal("expected error for nil RangeProofAlice, got nil")
	}
	if !strings.Contains(err.Error(), "RangeProofAlice missing") {
		t.Fatalf("unexpected error: %v", err)
	}
	requireCulprit(t, err, 1)
	t.Logf("correctly rejected nil RangeProofAlice: %v", err)
}

// TestSignRound2ContextCancellation verifies that SignRound2 returns an
// error when the context is already cancelled before the MtA goroutines run.
// Exercises the guard at round_fn.go:269-271.
func TestSignRound2ContextCancellation(t *testing.T) {
	keys := doKeygen(t)
	f := setupSignRound1(t, keys)

	// Create an already-cancelled context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := SignRound2(ctx, f.States[0], f.R1P2P[0], f.R1Bcast)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
	// The error should be context.Canceled, either directly or wrapped.
	if !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("correctly rejected cancelled context: %v", err)
}
