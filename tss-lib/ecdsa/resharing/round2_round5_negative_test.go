// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package resharing

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/hemilabs/x/tss-lib/v3/crypto"
	"github.com/hemilabs/x/tss-lib/v3/ecdsa/keygen"
	"github.com/hemilabs/x/tss-lib/v3/testutil"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// reshareFixture holds everything produced by rounds 1-4 of an honest
// reshare, so that negative tests can corrupt individual messages before
// calling the target round.
type reshareFixture struct {
	n, threshold int
	oldKeys      []keygen.LocalPartySaveData
	oldStates    []*ReshareState
	newStates    []*ReshareState
	oldR1Msgs    []*tss.Message
	newR2Msg1s   []*tss.Message
	newR2Msg2s   []*tss.Message
	oldR3P2P     [][]*tss.Message
	oldR3Bcast   []*tss.Message
	newR4P2P     [][]*tss.Message
	newR4Bcast   []*tss.Message
	newPIDs      tss.SortedPartyIDs
	oldPIDs      tss.SortedPartyIDs
}

// buildReshareFixture runs an honest keygen(3) followed by reshare
// rounds 1-4 with all proofs disabled (SNARK mode), returning the
// intermediate state needed to test Round2 and Round5 error paths.
func buildReshareFixture(t *testing.T) *reshareFixture {
	t.Helper()
	const n = 3
	const threshold = 1 // 2-of-3

	// ---- Keygen ----
	preParamsOld := testutil.LoadPreParams(t, n)
	oldPIDs := tss.GenerateTestPartyIDs(n)
	oldCtx := tss.NewPeerContext(oldPIDs)

	kgStates := make([]*keygen.KeygenState, n)
	kgR1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.S256(), oldCtx, oldPIDs[i], n, threshold)
		st, out, err := keygen.Round1(context.Background(), params, preParamsOld[i])
		if err != nil {
			t.Fatalf("keygen Round1[%d]: %v", i, err)
		}
		kgStates[i] = st
		kgR1[i] = out.Messages[0]
	}

	kgR2P2P := make([][]*tss.Message, n)
	kgR2Bcast := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		kgR2P2P[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := keygen.Round2(context.Background(), kgStates[i], kgR1)
		if err != nil {
			t.Fatalf("keygen Round2[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			pm := msg
			if pm.To == nil {
				kgR2Bcast[i] = pm
			} else {
				for _, to := range pm.To {
					kgR2P2P[to.Index][i] = pm
				}
			}
		}
		kgR2P2P[i][i] = kgStates[i].ExportR2P2PSelf()
		if kgR2Bcast[i] == nil {
			kgR2Bcast[i] = kgStates[i].ExportR2BcastSelf()
		}
	}

	kgR3 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := keygen.Round3(context.Background(), kgStates[i], kgR2P2P[i], kgR2Bcast)
		if err != nil {
			t.Fatalf("keygen Round3[%d]: %v", i, err)
		}
		kgR3[i] = out.Messages[0]
	}

	oldKeys := make([]keygen.LocalPartySaveData, n)
	for i := 0; i < n; i++ {
		out, err := keygen.Round4(context.Background(), kgStates[i], kgR3)
		if err != nil {
			t.Fatalf("keygen Round4[%d]: %v", i, err)
		}
		oldKeys[i] = *out.Save
	}

	// ---- Reshare: 3 old -> 3 new (disjoint), all proofs disabled ----
	newPIDs := tss.GenerateTestPartyIDs(n)
	newCtx := tss.NewPeerContext(newPIDs)

	preParamsNew := testutil.LoadPreParamsFrom(t, n, n)

	oldStates := make([]*ReshareState, n)
	newStates := make([]*ReshareState, n)
	oldR1Msgs := make([]*tss.Message, n)

	for i := 0; i < n; i++ {
		params := tss.NewReSharingParameters(tss.S256(), oldCtx, newCtx, oldPIDs[i], n, threshold, n, threshold)
		params.SetNoProofDLN()
		params.SetNoProofMod()
		params.SetNoProofFac()
		st, out, err := ReshareRound1(params, oldKeys[i], keygen.LocalPreParams{})
		if err != nil {
			t.Fatalf("ReshareRound1(old)[%d]: %v", i, err)
		}
		oldStates[i] = st
		if len(out.Messages) > 0 {
			oldR1Msgs[i] = out.Messages[0]
		}
	}
	for i := 0; i < n; i++ {
		params := tss.NewReSharingParameters(tss.S256(), oldCtx, newCtx, newPIDs[i], n, threshold, n, threshold)
		params.SetNoProofDLN()
		params.SetNoProofMod()
		params.SetNoProofFac()
		st, _, err := ReshareRound1(params, keygen.NewLocalPartySaveData(n), preParamsNew[i])
		if err != nil {
			t.Fatalf("ReshareRound1(new)[%d]: %v", i, err)
		}
		newStates[i] = st
	}

	// Round 2
	newR2Msg1s := make([]*tss.Message, n)
	newR2Msg2s := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := ReshareRound2(newStates[i], oldR1Msgs)
		if err != nil {
			t.Fatalf("ReshareRound2(new)[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			pm := msg
			switch pm.Content.(type) {
			case *DGRound2Message1:
				newR2Msg1s[i] = pm
			case *DGRound2Message2:
				newR2Msg2s[i] = pm
			}
		}
	}
	for i := 0; i < n; i++ {
		_, err := ReshareRound2(oldStates[i], oldR1Msgs)
		if err != nil {
			t.Fatalf("ReshareRound2(old)[%d]: %v", i, err)
		}
	}
	for i := 0; i < n; i++ {
		if newR2Msg1s[i] == nil {
			newR2Msg1s[i] = newStates[i].temp.dgRound2Message1s[i]
		}
		if newR2Msg2s[i] == nil {
			newR2Msg2s[i] = newStates[i].temp.dgRound2Message2s[i]
		}
	}

	// Round 3
	oldR3P2P := make([][]*tss.Message, n)
	oldR3Bcast := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		oldR3P2P[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := ReshareRound3(oldStates[i], newR2Msg2s)
		if err != nil {
			t.Fatalf("ReshareRound3(old)[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			pm := msg
			switch pm.Content.(type) {
			case *DGRound3Message1:
				for _, to := range pm.To {
					oldR3P2P[to.Index][i] = pm
				}
			case *DGRound3Message2:
				oldR3Bcast[i] = pm
			}
		}
	}
	for i := 0; i < n; i++ {
		_, _ = ReshareRound3(newStates[i], newR2Msg2s)
	}

	// Round 4
	newR4P2P := make([][]*tss.Message, n)
	newR4Bcast := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		newR4P2P[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := ReshareRound4(context.Background(), newStates[i], newR2Msg1s, oldR3P2P[i], oldR3Bcast)
		if err != nil {
			t.Fatalf("ReshareRound4(new)[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			pm := msg
			switch pm.Content.(type) {
			case *DGRound4Message1:
				for _, to := range pm.To {
					newR4P2P[to.Index][i] = pm
				}
			case *DGRound4Message2:
				newR4Bcast[i] = pm
			}
		}
	}
	for i := 0; i < n; i++ {
		_, _ = ReshareRound4(context.Background(), oldStates[i], newR2Msg1s, nil, nil)
	}
	for i := 0; i < n; i++ {
		if newR4Bcast[i] == nil {
			newR4Bcast[i] = newStates[i].temp.dgRound4Message2s[i]
		}
	}

	return &reshareFixture{
		n:          n,
		threshold:  threshold,
		oldKeys:    oldKeys,
		oldStates:  oldStates,
		newStates:  newStates,
		oldR1Msgs:  oldR1Msgs,
		newR2Msg1s: newR2Msg1s,
		newR2Msg2s: newR2Msg2s,
		oldR3P2P:   oldR3P2P,
		oldR3Bcast: oldR3Bcast,
		newR4P2P:   newR4P2P,
		newR4Bcast: newR4Bcast,
		newPIDs:    newPIDs,
		oldPIDs:    oldPIDs,
	}
}

// cloneR4P2PSlice returns a deep copy of the P2P message slice for
// one new party so that corruption in negative tests is isolated.
func cloneR4P2PSlice(src []*tss.Message) []*tss.Message {
	dst := make([]*tss.Message, len(src))
	for j, m := range src {
		if m == nil {
			continue
		}
		orig := m.Content.(*DGRound4Message1)
		dst[j] = &tss.Message{
			From: m.From,
			To:   m.To,
			Content: &DGRound4Message1{
				FacProof:   orig.FacProof,
				ReceiverID: append([]byte(nil), orig.ReceiverID...),
			},
		}
	}
	return dst
}

// ---------------------------------------------------------------------------
// Round 5 negative tests
// ---------------------------------------------------------------------------

// TestReshareRound5RejectsReceiverIDMismatch corrupts the ReceiverID
// in a DGRound4Message1 and verifies that Round5 rejects it with the
// expected "receiverId mismatch" error.
func TestReshareRound5RejectsReceiverIDMismatch(t *testing.T) {
	fix := buildReshareFixture(t)

	// Target: new party 0. Corrupt the ReceiverID from party 1.
	target := 0
	corrupted := cloneR4P2PSlice(fix.newR4P2P[target])

	// Find the first non-self, non-nil message to corrupt.
	corruptIdx := -1
	for j := 0; j < fix.n; j++ {
		if j == target && corrupted[j] == nil {
			continue
		}
		if j != target && corrupted[j] != nil {
			corruptIdx = j
			break
		}
	}
	if corruptIdx < 0 {
		t.Fatal("no P2P message to corrupt")
	}
	// Replace ReceiverID with garbage bytes.
	corrupted[corruptIdx].Content.(*DGRound4Message1).ReceiverID = []byte("wrong-receiver-id")

	_, err := ReshareRound5(fix.newStates[target], corrupted, fix.newR4Bcast)
	if err == nil {
		t.Fatal("expected error for corrupted ReceiverID, got nil")
	}
	if !strings.Contains(err.Error(), "receiverId mismatch") {
		t.Fatalf("expected 'receiverId mismatch' error, got: %v", err)
	}
	requireCulprit(t, err, corruptIdx)
	t.Logf("correctly rejected corrupted ReceiverID: %v", err)
}

// TestReshareRound5RejectsNilFacProof sets the FacProof to nil in a
// DGRound4Message1 and verifies that Round5 rejects it when FacProof
// verification is enabled.
func TestReshareRound5RejectsNilFacProof(t *testing.T) {
	// We need a fixture with FacProof enabled (NoProofFac = false).
	// Rebuild with proofs enabled so the path through proof verification
	// is exercised.
	fix := buildReshareFixtureWithFacProof(t)

	target := 0
	corrupted := cloneR4P2PSlice(fix.newR4P2P[target])

	corruptIdx := -1
	for j := 0; j < fix.n; j++ {
		if j != target && corrupted[j] != nil {
			corruptIdx = j
			break
		}
	}
	if corruptIdx < 0 {
		t.Fatal("no P2P message to corrupt")
	}
	corrupted[corruptIdx].Content.(*DGRound4Message1).FacProof = nil

	_, err := ReshareRound5(fix.newStates[target], corrupted, fix.newR4Bcast)
	if err == nil {
		t.Fatal("expected error for nil FacProof, got nil")
	}
	if !strings.Contains(err.Error(), "facProof missing") {
		t.Fatalf("expected 'facProof missing' error, got: %v", err)
	}
	requireCulprit(t, err, corruptIdx)
	t.Logf("correctly rejected nil FacProof: %v", err)
}

// TestReshareRound5RejectsBadFacProof corrupts a FacProof field in a
// DGRound4Message1 and verifies that Round5 rejects it when FacProof
// verification is enabled.
func TestReshareRound5RejectsBadFacProof(t *testing.T) {
	fix := buildReshareFixtureWithFacProof(t)

	target := 0
	corrupted := cloneR4P2PSlice(fix.newR4P2P[target])

	corruptIdx := -1
	for j := 0; j < fix.n; j++ {
		if j != target && corrupted[j] != nil {
			corruptIdx = j
			break
		}
	}
	if corruptIdx < 0 {
		t.Fatal("no P2P message to corrupt")
	}
	proof := corrupted[corruptIdx].Content.(*DGRound4Message1).FacProof
	if proof == nil {
		t.Fatal("expected non-nil FacProof in fixture with proofs enabled")
	}
	// Corrupt the proof by flipping a field. The P field is part of the
	// proof verification equation; adding 1 invalidates it.
	proof.P = new(big.Int).Add(proof.P, big.NewInt(1))

	_, err := ReshareRound5(fix.newStates[target], corrupted, fix.newR4Bcast)
	if err == nil {
		t.Fatal("expected error for corrupted FacProof, got nil")
	}
	if !strings.Contains(err.Error(), "facProof verify failed") {
		t.Fatalf("expected 'facProof verify failed' error, got: %v", err)
	}
	requireCulprit(t, err, corruptIdx)
	t.Logf("correctly rejected corrupted FacProof: %v", err)
}

// buildReshareFixtureWithFacProof is identical to buildReshareFixture
// except FacProof generation and verification are enabled (only DLN
// and Mod proofs are disabled). This is needed because the default
// fixture disables all proofs.
func buildReshareFixtureWithFacProof(t *testing.T) *reshareFixture {
	t.Helper()
	const n = 3
	const threshold = 1

	// ---- Keygen ----
	preParamsOld := testutil.LoadPreParams(t, n)
	oldPIDs := tss.GenerateTestPartyIDs(n)
	oldCtx := tss.NewPeerContext(oldPIDs)

	kgStates := make([]*keygen.KeygenState, n)
	kgR1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.S256(), oldCtx, oldPIDs[i], n, threshold)
		st, out, err := keygen.Round1(context.Background(), params, preParamsOld[i])
		if err != nil {
			t.Fatalf("keygen Round1[%d]: %v", i, err)
		}
		kgStates[i] = st
		kgR1[i] = out.Messages[0]
	}
	kgR2P2P := make([][]*tss.Message, n)
	kgR2Bcast := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		kgR2P2P[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := keygen.Round2(context.Background(), kgStates[i], kgR1)
		if err != nil {
			t.Fatalf("keygen Round2[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			pm := msg
			if pm.To == nil {
				kgR2Bcast[i] = pm
			} else {
				for _, to := range pm.To {
					kgR2P2P[to.Index][i] = pm
				}
			}
		}
		kgR2P2P[i][i] = kgStates[i].ExportR2P2PSelf()
		if kgR2Bcast[i] == nil {
			kgR2Bcast[i] = kgStates[i].ExportR2BcastSelf()
		}
	}
	kgR3 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := keygen.Round3(context.Background(), kgStates[i], kgR2P2P[i], kgR2Bcast)
		if err != nil {
			t.Fatalf("keygen Round3[%d]: %v", i, err)
		}
		kgR3[i] = out.Messages[0]
	}
	oldKeys := make([]keygen.LocalPartySaveData, n)
	for i := 0; i < n; i++ {
		out, err := keygen.Round4(context.Background(), kgStates[i], kgR3)
		if err != nil {
			t.Fatalf("keygen Round4[%d]: %v", i, err)
		}
		oldKeys[i] = *out.Save
	}

	// ---- Reshare with FacProof ENABLED (DLN+Mod disabled) ----
	newPIDs := tss.GenerateTestPartyIDs(n)
	newCtx := tss.NewPeerContext(newPIDs)
	preParamsNew := testutil.LoadPreParamsFrom(t, n, n)

	oldStates := make([]*ReshareState, n)
	newStates := make([]*ReshareState, n)
	oldR1Msgs := make([]*tss.Message, n)

	for i := 0; i < n; i++ {
		params := tss.NewReSharingParameters(tss.S256(), oldCtx, newCtx, oldPIDs[i], n, threshold, n, threshold)
		params.SetNoProofDLN()
		params.SetNoProofMod()
		// FacProof enabled (no SetNoProofFac call)
		st, out, err := ReshareRound1(params, oldKeys[i], keygen.LocalPreParams{})
		if err != nil {
			t.Fatalf("ReshareRound1(old)[%d]: %v", i, err)
		}
		oldStates[i] = st
		if len(out.Messages) > 0 {
			oldR1Msgs[i] = out.Messages[0]
		}
	}
	for i := 0; i < n; i++ {
		params := tss.NewReSharingParameters(tss.S256(), oldCtx, newCtx, newPIDs[i], n, threshold, n, threshold)
		params.SetNoProofDLN()
		params.SetNoProofMod()
		// FacProof enabled
		st, _, err := ReshareRound1(params, keygen.NewLocalPartySaveData(n), preParamsNew[i])
		if err != nil {
			t.Fatalf("ReshareRound1(new)[%d]: %v", i, err)
		}
		newStates[i] = st
	}

	// Round 2
	newR2Msg1s := make([]*tss.Message, n)
	newR2Msg2s := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := ReshareRound2(newStates[i], oldR1Msgs)
		if err != nil {
			t.Fatalf("ReshareRound2(new)[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			pm := msg
			switch pm.Content.(type) {
			case *DGRound2Message1:
				newR2Msg1s[i] = pm
			case *DGRound2Message2:
				newR2Msg2s[i] = pm
			}
		}
	}
	for i := 0; i < n; i++ {
		_, err := ReshareRound2(oldStates[i], oldR1Msgs)
		if err != nil {
			t.Fatalf("ReshareRound2(old)[%d]: %v", i, err)
		}
	}
	for i := 0; i < n; i++ {
		if newR2Msg1s[i] == nil {
			newR2Msg1s[i] = newStates[i].temp.dgRound2Message1s[i]
		}
		if newR2Msg2s[i] == nil {
			newR2Msg2s[i] = newStates[i].temp.dgRound2Message2s[i]
		}
	}

	// Round 3
	oldR3P2P := make([][]*tss.Message, n)
	oldR3Bcast := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		oldR3P2P[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := ReshareRound3(oldStates[i], newR2Msg2s)
		if err != nil {
			t.Fatalf("ReshareRound3(old)[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			pm := msg
			switch pm.Content.(type) {
			case *DGRound3Message1:
				for _, to := range pm.To {
					oldR3P2P[to.Index][i] = pm
				}
			case *DGRound3Message2:
				oldR3Bcast[i] = pm
			}
		}
	}
	for i := 0; i < n; i++ {
		_, _ = ReshareRound3(newStates[i], newR2Msg2s)
	}

	// Round 4 (FacProof enabled — generates real proofs)
	newR4P2P := make([][]*tss.Message, n)
	newR4Bcast := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		newR4P2P[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := ReshareRound4(context.Background(), newStates[i], newR2Msg1s, oldR3P2P[i], oldR3Bcast)
		if err != nil {
			t.Fatalf("ReshareRound4(new)[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			pm := msg
			switch pm.Content.(type) {
			case *DGRound4Message1:
				for _, to := range pm.To {
					newR4P2P[to.Index][i] = pm
				}
			case *DGRound4Message2:
				newR4Bcast[i] = pm
			}
		}
	}
	for i := 0; i < n; i++ {
		_, _ = ReshareRound4(context.Background(), oldStates[i], newR2Msg1s, nil, nil)
	}
	for i := 0; i < n; i++ {
		if newR4Bcast[i] == nil {
			newR4Bcast[i] = newStates[i].temp.dgRound4Message2s[i]
		}
	}

	return &reshareFixture{
		n:          n,
		threshold:  threshold,
		oldKeys:    oldKeys,
		oldStates:  oldStates,
		newStates:  newStates,
		oldR1Msgs:  oldR1Msgs,
		newR2Msg1s: newR2Msg1s,
		newR2Msg2s: newR2Msg2s,
		oldR3P2P:   oldR3P2P,
		oldR3Bcast: oldR3Bcast,
		newR4P2P:   newR4P2P,
		newR4Bcast: newR4Bcast,
		newPIDs:    newPIDs,
		oldPIDs:    oldPIDs,
	}
}

// ---------------------------------------------------------------------------
// Round 2 negative tests
// ---------------------------------------------------------------------------

// TestReshareRound2RejectsECDSAPubMismatch corrupts the ECDSAPub in
// one old party's R1 message so that it differs from the others, and
// verifies that Round2 rejects it with an "ecdsa pub key mismatch"
// error.
func TestReshareRound2RejectsECDSAPubMismatch(t *testing.T) {
	const n = 3
	const threshold = 1

	// ---- Keygen ----
	preParamsOld := testutil.LoadPreParams(t, n)
	oldPIDs := tss.GenerateTestPartyIDs(n)
	oldCtx := tss.NewPeerContext(oldPIDs)

	kgStates := make([]*keygen.KeygenState, n)
	kgR1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.S256(), oldCtx, oldPIDs[i], n, threshold)
		st, out, err := keygen.Round1(context.Background(), params, preParamsOld[i])
		if err != nil {
			t.Fatalf("keygen Round1[%d]: %v", i, err)
		}
		kgStates[i] = st
		kgR1[i] = out.Messages[0]
	}
	kgR2P2P := make([][]*tss.Message, n)
	kgR2Bcast := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		kgR2P2P[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := keygen.Round2(context.Background(), kgStates[i], kgR1)
		if err != nil {
			t.Fatalf("keygen Round2[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			pm := msg
			if pm.To == nil {
				kgR2Bcast[i] = pm
			} else {
				for _, to := range pm.To {
					kgR2P2P[to.Index][i] = pm
				}
			}
		}
		kgR2P2P[i][i] = kgStates[i].ExportR2P2PSelf()
		if kgR2Bcast[i] == nil {
			kgR2Bcast[i] = kgStates[i].ExportR2BcastSelf()
		}
	}
	kgR3 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := keygen.Round3(context.Background(), kgStates[i], kgR2P2P[i], kgR2Bcast)
		if err != nil {
			t.Fatalf("keygen Round3[%d]: %v", i, err)
		}
		kgR3[i] = out.Messages[0]
	}
	oldKeys := make([]keygen.LocalPartySaveData, n)
	for i := 0; i < n; i++ {
		out, err := keygen.Round4(context.Background(), kgStates[i], kgR3)
		if err != nil {
			t.Fatalf("keygen Round4[%d]: %v", i, err)
		}
		oldKeys[i] = *out.Save
	}

	// ---- Reshare Round 1 ----
	newPIDs := tss.GenerateTestPartyIDs(n)
	newCtx := tss.NewPeerContext(newPIDs)
	preParamsNew := testutil.LoadPreParamsFrom(t, n, n)

	oldStates := make([]*ReshareState, n)
	newStates := make([]*ReshareState, n)
	oldR1Msgs := make([]*tss.Message, n)

	for i := 0; i < n; i++ {
		params := tss.NewReSharingParameters(tss.S256(), oldCtx, newCtx, oldPIDs[i], n, threshold, n, threshold)
		params.SetNoProofDLN()
		params.SetNoProofMod()
		params.SetNoProofFac()
		st, out, err := ReshareRound1(params, oldKeys[i], keygen.LocalPreParams{})
		if err != nil {
			t.Fatalf("ReshareRound1(old)[%d]: %v", i, err)
		}
		oldStates[i] = st
		if len(out.Messages) > 0 {
			oldR1Msgs[i] = out.Messages[0]
		}
	}
	for i := 0; i < n; i++ {
		params := tss.NewReSharingParameters(tss.S256(), oldCtx, newCtx, newPIDs[i], n, threshold, n, threshold)
		params.SetNoProofDLN()
		params.SetNoProofMod()
		params.SetNoProofFac()
		st, _, err := ReshareRound1(params, keygen.NewLocalPartySaveData(n), preParamsNew[i])
		if err != nil {
			t.Fatalf("ReshareRound1(new)[%d]: %v", i, err)
		}
		newStates[i] = st
	}

	// Corrupt ECDSAPub in the second old party's R1 message:
	// Replace it with a different valid curve point (generator * 42).
	fakePoint := crypto.ScalarBaseMult(tss.S256(), big.NewInt(42))
	corruptedR1Msgs := make([]*tss.Message, n)
	copy(corruptedR1Msgs, oldR1Msgs)

	// Pick a message index to corrupt (index 1 — the second old party).
	// We need to create a new message with the corrupted ECDSAPub.
	origMsg := oldR1Msgs[1]
	origContent := origMsg.Content.(*DGRound1Message)
	corruptedR1Msgs[1] = &tss.Message{
		From:        origMsg.From,
		To:          origMsg.To,
		IsBroadcast: origMsg.IsBroadcast,
		Content: &DGRound1Message{
			ECDSAPub:    fakePoint,
			VCommitment: origContent.VCommitment,
			SSID:        origContent.SSID,
		},
	}

	// Round 2 on new party 0 should reject the mismatch.
	_, err := ReshareRound2(newStates[0], corruptedR1Msgs)
	if err == nil {
		t.Fatal("expected error for ECDSAPub mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "ecdsa pub key mismatch") {
		t.Fatalf("expected 'ecdsa pub key mismatch' error, got: %v", err)
	}
	t.Logf("correctly rejected ECDSAPub mismatch: %v", err)
}

// TestReshareRound2RejectsNilECDSAPub sets ECDSAPub to nil in one old
// party's R1 message.  Exercises the nil guard at round_fn.go:205-206.
func TestReshareRound2RejectsNilECDSAPub(t *testing.T) {
	fix := setupThroughRound1ForRound2(t)

	corruptedR1 := make([]*tss.Message, len(fix.oldR1Msgs))
	copy(corruptedR1, fix.oldR1Msgs)

	// Set party 1's ECDSAPub to nil.
	orig := fix.oldR1Msgs[1]
	origContent := orig.Content.(*DGRound1Message)
	corruptedR1[1] = &tss.Message{
		From:        orig.From,
		To:          orig.To,
		IsBroadcast: orig.IsBroadcast,
		Content: &DGRound1Message{
			ECDSAPub:    nil,
			VCommitment: origContent.VCommitment,
			SSID:        origContent.SSID,
		},
	}

	_, err := ReshareRound2(fix.newStates[0], corruptedR1)
	if err == nil {
		t.Fatal("expected error for nil ECDSAPub, got nil")
	}
	if !strings.Contains(err.Error(), "ecdsa pub nil") {
		t.Fatalf("expected 'ecdsa pub nil' error, got: %v", err)
	}
	t.Logf("correctly rejected nil ECDSAPub: %v", err)
}

// TestReshareRound2RejectsSSIDMismatch corrupts the SSID in one old
// party's R1 message so it differs from the others.  Exercises the
// SSID consistency check at round_fn.go:195-197.
func TestReshareRound2RejectsSSIDMismatch(t *testing.T) {
	fix := setupThroughRound1ForRound2(t)

	corruptedR1 := make([]*tss.Message, len(fix.oldR1Msgs))
	copy(corruptedR1, fix.oldR1Msgs)

	// Corrupt party 1's SSID to differ from party 0's.
	orig := fix.oldR1Msgs[1]
	origContent := orig.Content.(*DGRound1Message)
	corruptedR1[1] = &tss.Message{
		From:        orig.From,
		To:          orig.To,
		IsBroadcast: orig.IsBroadcast,
		Content: &DGRound1Message{
			ECDSAPub:    origContent.ECDSAPub,
			VCommitment: origContent.VCommitment,
			SSID:        []byte("wrong-ssid-value"),
		},
	}

	_, err := ReshareRound2(fix.newStates[0], corruptedR1)
	if err == nil {
		t.Fatal("expected error for SSID mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "ssid mismatch") {
		t.Fatalf("expected 'ssid mismatch' error, got: %v", err)
	}
	requireCulprit(t, err, 1) // corrupted old party 1
	t.Logf("correctly rejected SSID mismatch: %v", err)
}

// TestReshareRound2RejectsStalePreParams corrupts the new party's pre-params
// so that Validate() passes but ValidateWithProof() fails (nil Alpha).
// Exercises the guard at round_fn.go:222-223.
func TestReshareRound2RejectsStalePreParams(t *testing.T) {
	fix := setupThroughRound1ForRound2(t)

	// Corrupt new party 0's save data: nil out Alpha so Validate() passes
	// (it only checks PaillierSK, NTilde, H1, H2) but ValidateWithProof()
	// fails (it additionally checks Alpha, Beta, P, Q).
	fix.newStates[0].save.Alpha = nil

	_, err := ReshareRound2(fix.newStates[0], fix.oldR1Msgs)
	if err == nil {
		t.Fatal("expected error for stale preParams, got nil")
	}
	if !strings.Contains(err.Error(), "preParams failed validation") {
		t.Fatalf("expected 'preParams failed validation' error, got: %v", err)
	}
	t.Logf("correctly rejected stale preParams: %v", err)
}

// round2SetupFixture holds state for Round2 negative tests.
type round2SetupFixture struct {
	oldR1Msgs []*tss.Message
	newStates []*ReshareState
}

// setupThroughRound1ForRound2 runs keygen(3) + reshare Round1 for 3 old → 3
// new parties and returns the R1 messages + new-committee states ready for
// Round2 corruption tests.
func setupThroughRound1ForRound2(t *testing.T) *round2SetupFixture {
	t.Helper()
	const n = 3
	const threshold = 1

	// Keygen
	preParamsOld := testutil.LoadPreParams(t, n)
	oldPIDs := tss.GenerateTestPartyIDs(n)
	oldCtx := tss.NewPeerContext(oldPIDs)

	kgStates := make([]*keygen.KeygenState, n)
	kgR1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.S256(), oldCtx, oldPIDs[i], n, threshold)
		st, out, err := keygen.Round1(context.Background(), params, preParamsOld[i])
		if err != nil {
			t.Fatalf("keygen Round1[%d]: %v", i, err)
		}
		kgStates[i] = st
		kgR1[i] = out.Messages[0]
	}
	kgR2P2P := make([][]*tss.Message, n)
	kgR2Bcast := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		kgR2P2P[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := keygen.Round2(context.Background(), kgStates[i], kgR1)
		if err != nil {
			t.Fatalf("keygen Round2[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			pm := msg
			if pm.To == nil {
				kgR2Bcast[i] = pm
			} else {
				for _, to := range pm.To {
					kgR2P2P[to.Index][i] = pm
				}
			}
		}
		kgR2P2P[i][i] = kgStates[i].ExportR2P2PSelf()
		if kgR2Bcast[i] == nil {
			kgR2Bcast[i] = kgStates[i].ExportR2BcastSelf()
		}
	}
	kgR3 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := keygen.Round3(context.Background(), kgStates[i], kgR2P2P[i], kgR2Bcast)
		if err != nil {
			t.Fatalf("keygen Round3[%d]: %v", i, err)
		}
		kgR3[i] = out.Messages[0]
	}
	oldKeys := make([]keygen.LocalPartySaveData, n)
	for i := 0; i < n; i++ {
		out, err := keygen.Round4(context.Background(), kgStates[i], kgR3)
		if err != nil {
			t.Fatalf("keygen Round4[%d]: %v", i, err)
		}
		oldKeys[i] = *out.Save
	}

	// Reshare Round 1
	newPIDs := tss.GenerateTestPartyIDs(n)
	newCtx := tss.NewPeerContext(newPIDs)
	preParamsNew := testutil.LoadPreParamsFrom(t, n, n)

	newStates := make([]*ReshareState, n)
	oldR1Msgs := make([]*tss.Message, n)

	for i := 0; i < n; i++ {
		params := tss.NewReSharingParameters(tss.S256(), oldCtx, newCtx, oldPIDs[i], n, threshold, n, threshold)
		params.SetNoProofDLN()
		params.SetNoProofMod()
		params.SetNoProofFac()
		st, out, err := ReshareRound1(params, oldKeys[i], keygen.LocalPreParams{})
		if err != nil {
			t.Fatalf("ReshareRound1(old)[%d]: %v", i, err)
		}
		_ = st
		if len(out.Messages) > 0 {
			oldR1Msgs[i] = out.Messages[0]
		}
	}
	for i := 0; i < n; i++ {
		params := tss.NewReSharingParameters(tss.S256(), oldCtx, newCtx, newPIDs[i], n, threshold, n, threshold)
		params.SetNoProofDLN()
		params.SetNoProofMod()
		params.SetNoProofFac()
		st, _, err := ReshareRound1(params, keygen.NewLocalPartySaveData(n), preParamsNew[i])
		if err != nil {
			t.Fatalf("ReshareRound1(new)[%d]: %v", i, err)
		}
		newStates[i] = st
	}

	return &round2SetupFixture{
		oldR1Msgs: oldR1Msgs,
		newStates: newStates,
	}
}
