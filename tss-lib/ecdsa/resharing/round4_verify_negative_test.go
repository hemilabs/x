// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package resharing

import (
	"context"
	"crypto/rand"
	"math/big"
	"strings"
	"testing"

	"github.com/hemilabs/x/tss-lib/v3/crypto"
	cmt "github.com/hemilabs/x/tss-lib/v3/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// TestReshareRound4RejectsBadDecommitment corrupts the VDeCommitment from
// old party 0 and verifies that Round4 rejects it with a "v decommit failed"
// error.
func TestReshareRound4RejectsBadDecommitment(t *testing.T) {
	fix := setupThroughRound3(t)

	const target = 0 // corrupt old party 0's decommitment

	// Build a corrupted r3Bcast: flip a value in the decommitment.
	r3Bcast := copyR3BcastSlice(fix.OldR3Bcast)
	orig := r3Bcast[target].Content.(*DGRound3Message2)
	corruptD := make(cmt.HashDeCommitment, len(orig.VDeCommitment))
	for i, v := range orig.VDeCommitment {
		if v != nil {
			corruptD[i] = new(big.Int).Set(v)
		}
	}
	// Corrupt the second element (first secret coordinate after randomness).
	corruptD[1] = new(big.Int).Add(corruptD[1], big.NewInt(1))
	r3Bcast[target] = &tss.Message{
		From:        r3Bcast[target].From,
		To:          r3Bcast[target].To,
		IsBroadcast: true,
		Content:     &DGRound3Message2{VDeCommitment: corruptD},
	}

	_, err := ReshareRound4(context.Background(), fix.NewStates[0], fix.NewR2Msg1s, fix.OldR3P2P[0], r3Bcast)
	if err == nil {
		t.Fatal("expected error from corrupted decommitment, got nil")
	}
	if !strings.Contains(err.Error(), "decommit") {
		t.Fatalf("expected decommit error, got: %v", err)
	}
	requireCulprit(t, err, target)
	t.Logf("correctly rejected bad decommitment: %v", err)
}

// TestReshareRound4RejectsReceiverIDMismatch corrupts the ReceiverID in
// old party 1's P2P share message addressed to new party 0, and verifies
// that Round4 rejects it.
func TestReshareRound4RejectsReceiverIDMismatch(t *testing.T) {
	fix := setupThroughRound3(t)

	const corruptFrom = 1 // corrupt message from old party 1

	r3P2P := copyR3P2PSlice(fix.OldR3P2P[0])
	orig := r3P2P[corruptFrom].Content.(*DGRound3Message1)
	// Replace ReceiverID with a garbage value.
	r3P2P[corruptFrom] = &tss.Message{
		From: r3P2P[corruptFrom].From,
		To:   r3P2P[corruptFrom].To,
		Content: &DGRound3Message1{
			Share:      orig.Share,
			ReceiverID: []byte("wrong-receiver-id"),
		},
	}

	_, err := ReshareRound4(context.Background(), fix.NewStates[0], fix.NewR2Msg1s, r3P2P, fix.OldR3Bcast)
	if err == nil {
		t.Fatal("expected error from ReceiverID mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "receiverId mismatch") {
		t.Fatalf("expected receiverId mismatch error, got: %v", err)
	}
	requireCulprit(t, err, corruptFrom)
	t.Logf("correctly rejected ReceiverID mismatch: %v", err)
}

// TestReshareRound4RejectsBadVSSShare corrupts the VSS share value from
// old party 2 and verifies that the Feldman VSS verification fails.
func TestReshareRound4RejectsBadVSSShare(t *testing.T) {
	fix := setupThroughRound3(t)

	const corruptFrom = 2 // corrupt share from old party 2

	r3P2P := copyR3P2PSlice(fix.OldR3P2P[0])
	orig := r3P2P[corruptFrom].Content.(*DGRound3Message1)
	// Corrupt the share by adding 1.
	badShare := new(big.Int).Add(orig.Share, big.NewInt(1))
	r3P2P[corruptFrom] = &tss.Message{
		From: r3P2P[corruptFrom].From,
		To:   r3P2P[corruptFrom].To,
		Content: &DGRound3Message1{
			Share:      badShare,
			ReceiverID: orig.ReceiverID,
		},
	}

	_, err := ReshareRound4(context.Background(), fix.NewStates[0], fix.NewR2Msg1s, r3P2P, fix.OldR3Bcast)
	if err == nil {
		t.Fatal("expected error from corrupted VSS share, got nil")
	}
	if !strings.Contains(err.Error(), "vss share verify failed") {
		t.Fatalf("expected vss share verify error, got: %v", err)
	}
	requireCulprit(t, err, corruptFrom)
	t.Logf("correctly rejected bad VSS share: %v", err)
}

// TestReshareRound4ContextCancellation passes a pre-cancelled context
// and verifies that Round4 returns an error (either context.Canceled
// or a wrapped form of it).
func TestReshareRound4ContextCancellation(t *testing.T) {
	fix := setupThroughRound3(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	_, err := ReshareRound4(ctx, fix.NewStates[0], fix.NewR2Msg1s, fix.OldR3P2P[0], fix.OldR3Bcast)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if !strings.Contains(err.Error(), "cancel") && !strings.Contains(err.Error(), "context") {
		t.Fatalf("expected context cancellation error, got: %v", err)
	}
	t.Logf("correctly rejected with cancelled context: %v", err)
}

// TestReshareRound4RejectsNilModProof runs with ModProof enabled (not
// disabled via SetNoProofMod) and sets one new party's ModProof to nil.
// Exercises the proof-nil culprit path at round_fn.go:417-420.
func TestReshareRound4RejectsNilModProof(t *testing.T) {
	fix := setupThroughRound3WithModProof(t)

	// Clone the R2 messages and nil out party 1's ModProof.
	msgs := copyR2Msg1Slice(fix.NewR2Msg1s)
	clone := cloneDGRound2Message1(msgs[r4VictimIdx])
	clone.Content.(*DGRound2Message1).ModProof = nil
	msgs[r4VictimIdx] = clone

	_, err := ReshareRound4(context.Background(), fix.NewStates[0], msgs, fix.OldR3P2P[0], fix.OldR3Bcast)
	if err == nil {
		t.Fatal("expected error for nil ModProof, got nil")
	}
	if !strings.Contains(err.Error(), "proof verification failed") {
		t.Fatalf("expected 'proof verification failed', got: %v", err)
	}
	requireCulprit(t, err, r4VictimIdx)
	t.Logf("correctly rejected nil ModProof: %v", err)
}

// TestReshareRound4RejectsV0NotECDSAPub mutates the new party's stored
// ECDSAPub to a different valid curve point, so the reconstructed Vc[0]
// (sum of old parties' first VSS coefficients) does not match. All
// decommitments and VSS shares are legitimate — only the final
// comparison at round_fn.go:523 fails.
func TestReshareRound4RejectsV0NotECDSAPub(t *testing.T) {
	fix := setupThroughRound3(t)

	// Create a different valid curve point (42·G on secp256k1).
	fakeKey := crypto.ScalarBaseMult(tss.S256(), big.NewInt(42))
	if fakeKey.Equals(fix.NewStates[0].save.ECDSAPub) {
		t.Fatal("precondition: fakeKey == real ECDSAPub")
	}
	fix.NewStates[0].save.ECDSAPub = fakeKey

	_, err := ReshareRound4(context.Background(), fix.NewStates[0], fix.NewR2Msg1s, fix.OldR3P2P[0], fix.OldR3Bcast)
	if err == nil {
		t.Fatal("expected error from V_0 != ECDSAPub, got nil")
	}
	if !strings.Contains(err.Error(), "V_0 != ECDSAPub") {
		t.Fatalf("expected 'V_0 != ECDSAPub' error, got: %v", err)
	}
	t.Logf("correctly rejected V_0 != ECDSAPub: %v", err)
}

// TestReshareRound4RejectsWrongLengthDecommitment creates a valid
// commitment/decommitment pair (hash matches) with the wrong number of
// secrets. For threshold=1, Round4 expects (threshold+1)*2 = 4 values
// (two EC points flattened). This test provides 6, triggering the
// len(flatVs) branch at round_fn.go:482.
func TestReshareRound4RejectsWrongLengthDecommitment(t *testing.T) {
	fix := setupThroughRound3(t)

	const target = 0

	// 6 secrets instead of 4 → len(flatVs) == 6, expected 4.
	wrongCmt := cmt.NewHashCommitment(rand.Reader,
		big.NewInt(1), big.NewInt(2), big.NewInt(3),
		big.NewInt(4), big.NewInt(5), big.NewInt(6))

	// Patch the stored R1 commitment hash.
	fix.NewStates[0].temp.dgRound1Messages[target].Content.(*DGRound1Message).VCommitment = wrongCmt.C

	// Patch the R3 broadcast decommitment.
	r3Bcast := copyR3BcastSlice(fix.OldR3Bcast)
	r3Bcast[target] = &tss.Message{
		From:        fix.OldR3Bcast[target].From,
		To:          fix.OldR3Bcast[target].To,
		IsBroadcast: true,
		Content:     &DGRound3Message2{VDeCommitment: wrongCmt.D},
	}

	_, err := ReshareRound4(context.Background(), fix.NewStates[0], fix.NewR2Msg1s, fix.OldR3P2P[0], r3Bcast)
	if err == nil {
		t.Fatal("expected error from wrong-length decommitment, got nil")
	}
	if !strings.Contains(err.Error(), "decommit") {
		t.Fatalf("expected decommit error, got: %v", err)
	}
	requireCulprit(t, err, target)
	t.Logf("correctly rejected wrong-length decommitment: %v", err)
}

// TestReshareRound4RejectsOffCurveDecommitment creates a valid
// commitment/decommitment pair with the correct number of secrets (4)
// but where the coordinate pairs (1,1) and (2,2) are not on secp256k1.
// Exercises the UnFlattenECPoints error at round_fn.go:485-488.
func TestReshareRound4RejectsOffCurveDecommitment(t *testing.T) {
	fix := setupThroughRound3(t)

	const target = 0

	// (1,1) and (2,2) are not on secp256k1 (y^2 != x^3 + 7 mod p).
	offCmt := cmt.NewHashCommitment(rand.Reader,
		big.NewInt(1), big.NewInt(1), big.NewInt(2), big.NewInt(2))

	// Patch the stored R1 commitment hash.
	fix.NewStates[0].temp.dgRound1Messages[target].Content.(*DGRound1Message).VCommitment = offCmt.C

	// Patch the R3 broadcast decommitment.
	r3Bcast := copyR3BcastSlice(fix.OldR3Bcast)
	r3Bcast[target] = &tss.Message{
		From:        fix.OldR3Bcast[target].From,
		To:          fix.OldR3Bcast[target].To,
		IsBroadcast: true,
		Content:     &DGRound3Message2{VDeCommitment: offCmt.D},
	}

	_, err := ReshareRound4(context.Background(), fix.NewStates[0], fix.NewR2Msg1s, fix.OldR3P2P[0], r3Bcast)
	if err == nil {
		t.Fatal("expected error from off-curve decommitment, got nil")
	}
	if !strings.Contains(err.Error(), "not on the elliptic curve") {
		t.Fatalf("expected curve error, got: %v", err)
	}
	requireCulprit(t, err, target)
	t.Logf("correctly rejected off-curve decommitment: %v", err)
}
