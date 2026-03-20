// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hemilabs/x/tss/v3/tss"
)

const (
	testN         = 3
	testThreshold = 1 // 2-of-3
)

// makeTestParams creates a standard 3-party, threshold=1 parameter set for
// party 0 with all expensive proofs disabled.  Returns params, sorted
// party IDs, and the peer context so callers can create params for
// other parties or re-create params with the same party set.
func makeTestParams() (*tss.Parameters, tss.SortedPartyIDs, *tss.PeerContext) {
	pIDs := tss.GenerateTestPartyIDs(testN)
	peerCtx := tss.NewPeerContext(pIDs)
	params := tss.NewParameters(tss.S256(), peerCtx, pIDs[0], testN, testThreshold)
	params.SetNoProofDLN()
	params.SetNoProofMod()
	params.SetNoProofFac()
	return params, pIDs, peerCtx
}

// TestRound1WithInvalidPreParamsGeneratesFresh passes zero-value
// pre-params (which fail both Validate and ValidateWithProof) and
// verifies that Round1 still succeeds by generating fresh safe primes.
//
// NOTE: This test is slow (~30-120s) because it generates safe primes
// on the fly.  Run with -timeout 5m.
func TestRound1WithInvalidPreParamsGeneratesFresh(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow test in short mode")
	}

	params, _, _ := makeTestParams()

	// Zero-value LocalPreParams: all fields nil, so Validate() returns false
	// and ValidateWithProof() returns false.  Round1 should fall through to
	// the fresh-generation branch.
	var emptyPreParams LocalPreParams

	// Sanity: confirm our zero-value pre-params are indeed invalid.
	if emptyPreParams.Validate() {
		t.Fatal("expected zero-value preParams to fail Validate()")
	}
	if emptyPreParams.ValidateWithProof() {
		t.Fatal("expected zero-value preParams to fail ValidateWithProof()")
	}

	state, out, err := Round1(context.Background(), params, emptyPreParams)
	if err != nil {
		t.Fatalf("Round1 with empty preParams should generate fresh ones, got error: %v", err)
	}
	if state == nil {
		t.Fatal("expected non-nil state")
	}
	if out == nil || len(out.Messages) == 0 {
		t.Fatal("expected non-nil output with messages")
	}

	// Verify the generated pre-params are valid.
	if !state.save.Validate() {
		t.Fatal("freshly generated preParams should pass Validate()")
	}
	if !state.save.ValidateWithProof() {
		t.Fatal("freshly generated preParams should pass ValidateWithProof()")
	}

	// Verify the round 1 message is well-formed.
	r1msg := out.Messages[0].Content.(*KGRound1Message)
	if !r1msg.ValidateBasic() {
		t.Fatal("round 1 message should pass ValidateBasic()")
	}
}

// TestRound1RejectsStalePreParams tests that pre-params which pass the
// basic Validate() but fail ValidateWithProof() (simulating old-format
// pre-params) cause Round1 to return an error rather than silently
// proceeding with bad parameters.
func TestRound1RejectsStalePreParams(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow test in short mode")
	}

	params, _, _ := makeTestParams()

	// Generate valid pre-params, then corrupt them so that Validate()
	// passes but ValidateWithProof() fails.  The easiest way: zero out
	// the Alpha field (needed by ValidateWithProof's H2 = H1^Alpha check).
	pp, err := GeneratePreParams(5 * time.Minute)
	if err != nil {
		t.Fatalf("GeneratePreParams: %v", err)
	}

	// Confirm it's initially valid.
	if !pp.Validate() || !pp.ValidateWithProof() {
		t.Fatal("generated preParams should be valid")
	}

	// Corrupt: nil out Alpha so ValidateWithProof fails but Validate passes.
	stale := *pp
	stale.Alpha = nil

	if !stale.Validate() {
		t.Fatal("stale preParams should still pass basic Validate()")
	}
	if stale.ValidateWithProof() {
		t.Fatal("stale preParams should fail ValidateWithProof()")
	}

	_, _, err = Round1(context.Background(), params, stale)
	if err == nil {
		t.Fatal("expected error for stale preParams, got nil")
	}
	if !strings.Contains(err.Error(), "preParams failed validation") {
		t.Fatalf("expected 'preParams failed validation' error, got: %v", err)
	}
}

// TestRound1RejectsContextCancellation passes a pre-cancelled context
// and verifies that Round1 returns an error when it needs to generate
// fresh pre-params (the only code path that checks ctx).
func TestRound1RejectsContextCancellation(t *testing.T) {
	params, _, _ := makeTestParams()

	// Use a very short timeout to trigger the generation timeout path.
	params.SetSafePrimeGenTimeout(1 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	// Zero-value pre-params forces the generation branch.
	var emptyPreParams LocalPreParams

	_, _, err := Round1(ctx, params, emptyPreParams)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	// The error message comes from Round1 wrapping the generation failure.
	if !strings.Contains(err.Error(), "pre-params generation failed") {
		t.Fatalf("expected 'pre-params generation failed' error, got: %v", err)
	}
}

// TestRound1ExportsCorrectSSIDNonce sets a specific SSID nonce on
// the parameters and verifies that two different nonces produce
// different SSIDs.  Uses the same party IDs and pre-params for
// both runs so the only variable is the nonce.
func TestRound1ExportsCorrectSSIDNonce(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow test in short mode")
	}

	// Generate shared pre-params and party IDs (reuse across both runs).
	pp, err := GeneratePreParams(5 * time.Minute)
	if err != nil {
		t.Fatalf("GeneratePreParams: %v", err)
	}

	pIDs := tss.GenerateTestPartyIDs(testN)
	peerCtx := tss.NewPeerContext(pIDs)

	runRound1WithNonce := func(nonce uint) *KeygenState {
		params := tss.NewParameters(tss.S256(), peerCtx, pIDs[0], testN, testThreshold)
		params.SetNoProofDLN()
		params.SetNoProofMod()
		params.SetNoProofFac()
		params.SetSSIDNonce(nonce)

		state, _, err := Round1(context.Background(), params, *pp)
		if err != nil {
			t.Fatalf("Round1 with nonce=%d: %v", nonce, err)
		}
		return state
	}

	state0 := runRound1WithNonce(0)
	state1 := runRound1WithNonce(1)

	// Access internal SSID via temp.ssid (package-level test access).
	ssid0 := state0.temp.ssid
	ssid1 := state1.temp.ssid

	if ssid0 == nil || ssid1 == nil {
		t.Fatal("SSID should be non-nil after Round1")
	}
	if bytes.Equal(ssid0, ssid1) {
		t.Fatalf("different SSIDNonce values (0 vs 1) should produce different SSIDs, got same: %x", ssid0)
	}

	// Verify the ssidNonce temp field was set correctly.
	if state0.temp.ssidNonce.Uint64() != 0 {
		t.Fatalf("expected ssidNonce=0, got %d", state0.temp.ssidNonce.Uint64())
	}
	if state1.temp.ssidNonce.Uint64() != 1 {
		t.Fatalf("expected ssidNonce=1, got %d", state1.temp.ssidNonce.Uint64())
	}
}

// TestRound1WithCeremonyID sets a CeremonyID on params, runs Round1,
// and verifies that the SSID includes the ceremony ID by comparing
// SSIDs with and without a CeremonyID set.  Uses the same party IDs
// and pre-params for all runs so the only variable is the CeremonyID.
func TestRound1WithCeremonyID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow test in short mode")
	}

	// Generate shared pre-params and party IDs.
	pp, err := GeneratePreParams(5 * time.Minute)
	if err != nil {
		t.Fatalf("GeneratePreParams: %v", err)
	}

	pIDs := tss.GenerateTestPartyIDs(testN)
	peerCtx := tss.NewPeerContext(pIDs)

	runRound1WithCeremonyID := func(ceremonyID []byte) *KeygenState {
		params := tss.NewParameters(tss.S256(), peerCtx, pIDs[0], testN, testThreshold)
		params.SetNoProofDLN()
		params.SetNoProofMod()
		params.SetNoProofFac()
		if ceremonyID != nil {
			params.SetCeremonyID(ceremonyID)
		}

		state, _, err := Round1(context.Background(), params, *pp)
		if err != nil {
			t.Fatalf("Round1 with ceremonyID=%x: %v", ceremonyID, err)
		}
		return state
	}

	// Run without CeremonyID.
	stateNoCID := runRound1WithCeremonyID(nil)

	// Run with CeremonyID = "test-ceremony-42".
	stateWithCID := runRound1WithCeremonyID([]byte("test-ceremony-42"))

	// Run with a different CeremonyID.
	stateWithCID2 := runRound1WithCeremonyID([]byte("test-ceremony-99"))

	ssidNone := stateNoCID.temp.ssid
	ssidCID := stateWithCID.temp.ssid
	ssidCID2 := stateWithCID2.temp.ssid

	if ssidNone == nil || ssidCID == nil || ssidCID2 == nil {
		t.Fatal("SSIDs should be non-nil after Round1")
	}

	// CeremonyID is included in the SSID hash.  Different CeremonyIDs
	// (including nil vs non-nil) must produce different SSIDs.
	if bytes.Equal(ssidNone, ssidCID) {
		t.Fatal("SSID with no CeremonyID should differ from SSID with CeremonyID")
	}
	if bytes.Equal(ssidCID, ssidCID2) {
		t.Fatal("SSIDs with different CeremonyIDs should differ")
	}
	if bytes.Equal(ssidNone, ssidCID2) {
		t.Fatal("SSID with no CeremonyID should differ from SSID with CeremonyID 2")
	}
}

// TestRound1CeremonyIDCausesCrossPartyMismatch verifies that if two
// parties use different CeremonyIDs, their SSIDs diverge.  This is
// the functional proof that CeremonyID is correctly bound into the
// protocol transcript.
func TestRound1CeremonyIDCausesCrossPartyMismatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow test in short mode")
	}

	preParams := make([]LocalPreParams, testN)
	for i := 0; i < testN; i++ {
		pp, err := GeneratePreParams(5 * time.Minute)
		if err != nil {
			t.Fatalf("GeneratePreParams[%d]: %v", i, err)
		}
		preParams[i] = *pp
	}

	pIDs := tss.GenerateTestPartyIDs(testN)
	peerCtx := tss.NewPeerContext(pIDs)

	// Party 0 and 1 use CeremonyID "A", party 2 uses CeremonyID "B".
	states := make([]*KeygenState, testN)
	for i := 0; i < testN; i++ {
		params := tss.NewParameters(tss.S256(), peerCtx, pIDs[i], testN, testThreshold)
		params.SetNoProofDLN()
		params.SetNoProofMod()
		params.SetNoProofFac()
		if i < 2 {
			params.SetCeremonyID([]byte("ceremony-A"))
		} else {
			params.SetCeremonyID([]byte("ceremony-B"))
		}

		st, _, err := Round1(context.Background(), params, preParams[i])
		if err != nil {
			t.Fatalf("Round1[%d]: %v", i, err)
		}
		states[i] = st
	}

	// Parties with different CeremonyIDs should have different SSIDs.
	if bytes.Equal(states[0].temp.ssid, states[2].temp.ssid) {
		t.Fatal("parties with different CeremonyIDs should have different SSIDs")
	}

	// Parties with the same CeremonyID should have the same SSID,
	// because getSSID does not include the individual party index --
	// only the full sorted party key list, which is shared.
	if !bytes.Equal(states[0].temp.ssid, states[1].temp.ssid) {
		t.Fatal("parties with the same CeremonyID should have the same SSID")
	}
}
