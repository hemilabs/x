// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package signing

import (
	"context"
	"strings"
	"testing"
)

// TestSignRound3RejectsReceiverIDMismatch verifies that SignRound3 returns an
// error when a Round 2 P2P message has a ReceiverID that does not match the
// processing party's key.
func TestSignRound3RejectsReceiverIDMismatch(t *testing.T) {
	f := setupThroughRound2(t)

	victim := 0
	corrupted := CloneBcastSlice(f.R2P2P[victim], CloneR2P2PMsg)

	// Corrupt the ReceiverID in the first non-nil message from another party.
	for j := 0; j < testN; j++ {
		if j == victim || corrupted[j] == nil {
			continue
		}
		corrupted[j].Content.(*SignRound2Message).ReceiverID = []byte("wrong-receiver-id")
		break
	}

	_, err := SignRound3(context.Background(), f.States[victim], corrupted)
	if err == nil {
		t.Fatal("expected error for ReceiverID mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "receiverId mismatch") {
		t.Fatalf("expected 'receiverId mismatch' in error, got: %v", err)
	}
	requireCulprit(t, err, 1)
	t.Logf("correctly rejected ReceiverID mismatch: %v", err)
}

// TestSignRound3RejectsNilProofBob verifies that SignRound3 returns an error
// when a Round 2 P2P message has a nil ProofBob field.
func TestSignRound3RejectsNilProofBob(t *testing.T) {
	f := setupThroughRound2(t)

	victim := 0
	corrupted := CloneBcastSlice(f.R2P2P[victim], CloneR2P2PMsg)

	for j := 0; j < testN; j++ {
		if j == victim || corrupted[j] == nil {
			continue
		}
		corrupted[j].Content.(*SignRound2Message).ProofBob = nil
		break
	}

	_, err := SignRound3(context.Background(), f.States[victim], corrupted)
	if err == nil {
		t.Fatal("expected error for nil ProofBob, got nil")
	}
	if !strings.Contains(err.Error(), "ProofBob missing") {
		t.Fatalf("expected 'ProofBob missing' in error, got: %v", err)
	}
	requireCulprit(t, err, 1)
	t.Logf("correctly rejected nil ProofBob: %v", err)
}

// TestSignRound3RejectsNilProofBobWC verifies that SignRound3 returns an error
// when a Round 2 P2P message has a nil ProofBobWC field.
func TestSignRound3RejectsNilProofBobWC(t *testing.T) {
	f := setupThroughRound2(t)

	victim := 0
	corrupted := CloneBcastSlice(f.R2P2P[victim], CloneR2P2PMsg)

	for j := 0; j < testN; j++ {
		if j == victim || corrupted[j] == nil {
			continue
		}
		corrupted[j].Content.(*SignRound2Message).ProofBobWC = nil
		break
	}

	_, err := SignRound3(context.Background(), f.States[victim], corrupted)
	if err == nil {
		t.Fatal("expected error for nil ProofBobWC, got nil")
	}
	if !strings.Contains(err.Error(), "ProofBobWC missing") {
		t.Fatalf("expected 'ProofBobWC missing' in error, got: %v", err)
	}
	requireCulprit(t, err, 1)
	t.Logf("correctly rejected nil ProofBobWC: %v", err)
}

// TestSignRound3ContextCancellation verifies that SignRound3 returns an error
// when the context is already cancelled before invocation.
func TestSignRound3ContextCancellation(t *testing.T) {
	f := setupThroughRound2(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := SignRound3(ctx, f.States[0], f.R2P2P[0])
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected 'context canceled' in error, got: %v", err)
	}
	t.Logf("correctly rejected cancelled context: %v", err)
}
