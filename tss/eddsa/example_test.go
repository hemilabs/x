// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

//go:build tssexamples
// +build tssexamples

// Package eddsa_test contains the canonical usage examples for the
// tss-lib v3 EdDSA round function API.
//
// Run with: go test -tags tssexamples -v ./eddsa/ -timeout 5m
package eddsa_test

import (
	"crypto/sha256"
	"fmt"
	"math/big"
	"testing"

	"github.com/decred/dcrd/dcrec/edwards/v2"

	"github.com/hemilabs/x/tss-lib/v3/eddsa/keygen"
	"github.com/hemilabs/x/tss-lib/v3/eddsa/resharing"
	"github.com/hemilabs/x/tss-lib/v3/eddsa/signing"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// TestEdDSAKeygenSignReshare demonstrates the full lifecycle:
// keygen → sign → reshare (with overlapping committees) → sign again.
//
// Old committee: [P0, P1, P2], threshold=1 (2-of-3)
// New committee: [P1, P2, P3], threshold=1 (2-of-3)
// P1 and P2 are in both committees (overlap).
// P0 drops out, P3 joins.
// The EdDSA public key is preserved across the reshare.
func TestEdDSAKeygenSignReshare(t *testing.T) {
	const threshold = 1

	// ------------------------------------------------------------------
	// Step 1: Create 4 party IDs — 3 old, 1 new joiner.
	// P1 and P2 overlap between old and new.
	// P0 drops out, P3 joins.
	//
	// Each committee needs its own *PartyID copies because
	// SortPartyIDs assigns Index based on sort position within
	// the committee, and the same key may have different indices
	// in different committees.
	// ------------------------------------------------------------------
	allPIDs := tss.GenerateTestPartyIDs(4)
	// Copy keys — each committee gets its own PartyID instances.
	copyPID := func(src *tss.PartyID) *tss.PartyID {
		return tss.NewPartyID(src.Id, src.Moniker, new(big.Int).SetBytes(src.Key))
	}
	oldPIDs := tss.SortPartyIDs(tss.UnSortedPartyIDs{
		copyPID(allPIDs[0]), copyPID(allPIDs[1]), copyPID(allPIDs[2]),
	})
	newPIDs := tss.SortPartyIDs(tss.UnSortedPartyIDs{
		copyPID(allPIDs[1]), copyPID(allPIDs[2]), copyPID(allPIDs[3]),
	})
	oldCtx := tss.NewPeerContext(oldPIDs)
	newCtx := tss.NewPeerContext(newPIDs)

	oldN := len(oldPIDs)
	newN := len(newPIDs)

	// ------------------------------------------------------------------
	// Step 2: Keygen (3 rounds, no Paillier needed for EdDSA)
	// ------------------------------------------------------------------
	oldSaves := eddsaKeygen(t, oldN, threshold, oldPIDs, oldCtx)
	pubKey := oldSaves[0].EDDSAPub
	t.Logf("keygen: EDDSAPub = (%x, %x)", pubKey.X(), pubKey.Y())

	// ------------------------------------------------------------------
	// Step 3: Sign with old committee
	// ------------------------------------------------------------------
	msg1 := sha256.Sum256([]byte("pre-reshare message"))
	sig1 := eddsaSign(t, oldN, threshold, oldPIDs, oldCtx, oldSaves, new(big.Int).SetBytes(msg1[:]))
	verifyEdDSA(t, pubKey, msg1[:], sig1)
	t.Log("pre-reshare signature verified")

	// ------------------------------------------------------------------
	// Step 4: Reshare — old [P0,P1,P2] → new [P1,P2,P3]
	//
	// 5 rounds.  Each party participates based on which committee(s)
	// it belongs to.  P1 and P2 are in both (dual-committee).
	// ------------------------------------------------------------------
	newSaves := eddsaReshare(t, oldPIDs, newPIDs, oldCtx, newCtx,
		oldSaves, threshold, threshold)

	// Verify: same public key, all new saves valid.
	for i := 0; i < newN; i++ {
		if !newSaves[i].EDDSAPub.Equals(pubKey) {
			t.Fatalf("new party %d: pub key changed after reshare", i)
		}
		if err := newSaves[i].ValidateSaveData(); err != nil {
			t.Fatalf("new party %d: %v", i, err)
		}
	}
	t.Log("reshare complete, pub key preserved")

	// Verify: old-only party (P0) had Xi zeroed.
	if oldSaves[0].Xi.Sign() != 0 {
		t.Fatal("P0 Xi not zeroed after reshare")
	}

	// ------------------------------------------------------------------
	// Step 5: Sign with new committee
	// ------------------------------------------------------------------
	msg2 := sha256.Sum256([]byte("post-reshare message"))
	sig2 := eddsaSign(t, newN, threshold, newPIDs, newCtx, newSaves, new(big.Int).SetBytes(msg2[:]))
	verifyEdDSA(t, pubKey, msg2[:], sig2)
	t.Log("post-reshare signature verified")
}

// --- helpers ---

// ecPoint is a point on an elliptic curve with X and Y coordinates.
type ecPoint interface {
	X() *big.Int
	Y() *big.Int
}

func verifyEdDSA(t *testing.T, pub ecPoint, msg []byte, sig *signing.SignatureData) {
	t.Helper()
	pk := edwards.PublicKey{Curve: tss.Edwards(), X: pub.X(), Y: pub.Y()}
	r := new(big.Int).SetBytes(sig.R)
	s := new(big.Int).SetBytes(sig.S)
	if !edwards.Verify(&pk, msg, r, s) {
		t.Fatal("EdDSA signature verification failed")
	}
}

func eddsaKeygen(t *testing.T, n, threshold int, pIDs tss.SortedPartyIDs, ctx *tss.PeerContext) []keygen.LocalPartySaveData {
	t.Helper()
	states := make([]*keygen.KeygenState, n)
	r1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.Edwards(), ctx, pIDs[i], n, threshold)
		st, out, err := keygen.Round1(params)
		if err != nil {
			t.Fatalf("keygen.Round1[%d]: %v", i, err)
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
			t.Fatalf("keygen.Round2[%d]: %v", i, err)
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
			t.Fatalf("keygen.Round3[%d]: %v", i, err)
		}
		saves[i] = *out.Save
	}
	return saves
}

func eddsaSign(t *testing.T, n, threshold int, pIDs tss.SortedPartyIDs, ctx *tss.PeerContext, saves []keygen.LocalPartySaveData, m *big.Int) *signing.SignatureData {
	t.Helper()
	states := make([]*signing.SigningState, n)
	r1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.Edwards(), ctx, pIDs[i], n, threshold)
		st, out, err := signing.SignRound1(params, saves[i], m, 0)
		if err != nil {
			t.Fatalf("SignRound1[%d]: %v", i, err)
		}
		states[i] = st
		r1[i] = out.Messages[0]
	}
	r2 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := signing.SignRound2(states[i], r1)
		if err != nil {
			t.Fatalf("SignRound2[%d]: %v", i, err)
		}
		r2[i] = out.Messages[0]
	}
	r3 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := signing.SignRound3(states[i], r2)
		if err != nil {
			t.Fatalf("SignRound3[%d]: %v", i, err)
		}
		r3[i] = out.Messages[0]
	}
	out, err := signing.SignFinalize(states[0], r3)
	if err != nil {
		t.Fatalf("SignFinalize: %v", err)
	}
	return out.Signature
}

func eddsaReshare(t *testing.T, oldPIDs, newPIDs tss.SortedPartyIDs, oldCtx, newCtx *tss.PeerContext, oldSaves []keygen.LocalPartySaveData, oldT, newT int) []keygen.LocalPartySaveData {
	t.Helper()

	oldN := len(oldPIDs)
	newN := len(newPIDs)

	// Build a combined party list: every unique party participates.
	type partyRole struct {
		pid    *tss.PartyID
		oldIdx int // -1 if not in old committee
		newIdx int // -1 if not in new committee
	}
	seen := make(map[string]*partyRole)
	var allParties []*partyRole
	for i, pid := range oldPIDs {
		key := fmt.Sprintf("%x", pid.Key)
		pr := &partyRole{pid: pid, oldIdx: i, newIdx: -1}
		seen[key] = pr
		allParties = append(allParties, pr)
	}
	for i, pid := range newPIDs {
		key := fmt.Sprintf("%x", pid.Key)
		if pr, ok := seen[key]; ok {
			pr.newIdx = i // dual-committee
		} else {
			pr := &partyRole{pid: pid, oldIdx: -1, newIdx: i}
			seen[key] = pr
			allParties = append(allParties, pr)
		}
	}

	// --- Round 1 (old committee produces, new committee no-ops) ---
	type stateEntry struct {
		state *resharing.ReshareState
		role  *partyRole
	}
	entries := make([]stateEntry, len(allParties))
	r1Msgs := make([]*tss.Message, oldN)

	for idx, pr := range allParties {
		params := tss.NewReSharingParameters(
			tss.Edwards(), oldCtx, newCtx, pr.pid, oldN, oldT, newN, newT)
		var input *keygen.LocalPartySaveData
		if pr.oldIdx >= 0 {
			input = &oldSaves[pr.oldIdx]
		}
		st, out, err := resharing.ReshareRound1(params, input)
		if err != nil {
			t.Fatalf("ReshareRound1[%s]: %v", pr.pid.Id, err)
		}
		entries[idx] = stateEntry{state: st, role: pr}
		if pr.oldIdx >= 0 && len(out.Messages) > 0 {
			r1Msgs[pr.oldIdx] = out.Messages[0]
		}
	}

	// --- Round 2 (new committee sends ACK) ---
	r2Msgs := make([]*tss.Message, newN)
	for idx := range entries {
		pr := entries[idx].role
		out, err := resharing.ReshareRound2(entries[idx].state, r1Msgs)
		if err != nil {
			t.Fatalf("ReshareRound2[%s]: %v", pr.pid.Id, err)
		}
		if pr.newIdx >= 0 && len(out.Messages) > 0 {
			r2Msgs[pr.newIdx] = out.Messages[0]
		}
	}

	// --- Round 3 (old committee sends shares + decommitment) ---
	r3p2p := make([][]*tss.Message, newN)
	r3bcast := make([]*tss.Message, oldN)
	for i := range r3p2p {
		r3p2p[i] = make([]*tss.Message, oldN)
	}
	for idx := range entries {
		pr := entries[idx].role
		out, err := resharing.ReshareRound3(entries[idx].state, r2Msgs)
		if err != nil {
			t.Fatalf("ReshareRound3[%s]: %v", pr.pid.Id, err)
		}
		if pr.oldIdx >= 0 {
			for _, msg := range out.Messages {
				switch msg.Content.(type) {
				case *resharing.DGRound3Message2:
					r3bcast[pr.oldIdx] = msg
				case *resharing.DGRound3Message1:
					for _, to := range msg.To {
						r3p2p[to.Index][pr.oldIdx] = msg
					}
				}
			}
		}
	}

	// --- Round 4 (new committee verifies + ACK) ---
	r4Msgs := make([]*tss.Message, newN)
	for idx := range entries {
		pr := entries[idx].role
		var myP2P []*tss.Message
		if pr.newIdx >= 0 {
			myP2P = r3p2p[pr.newIdx]
		}
		out, err := resharing.ReshareRound4(entries[idx].state, r1Msgs, myP2P, r3bcast)
		if err != nil {
			t.Fatalf("ReshareRound4[%s]: %v", pr.pid.Id, err)
		}
		if pr.newIdx >= 0 && len(out.Messages) > 0 {
			r4Msgs[pr.newIdx] = out.Messages[0]
		}
	}

	// --- Round 5 (save) ---
	newSaves := make([]keygen.LocalPartySaveData, newN)
	for idx := range entries {
		pr := entries[idx].role
		out, err := resharing.ReshareRound5(entries[idx].state, r4Msgs)
		if err != nil {
			t.Fatalf("ReshareRound5[%s]: %v", pr.pid.Id, err)
		}
		if pr.newIdx >= 0 {
			newSaves[pr.newIdx] = *out.Save
		}
	}

	return newSaves
}
