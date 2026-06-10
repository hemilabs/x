// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

//go:build tssexamples
// +build tssexamples

// Package ecdsa_test contains the canonical usage example for the
// tss-lib v3 ECDSA round function API.
//
// Run with: go test -tags tssexamples -v ./ecdsa/ -timeout 15m
package ecdsa_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/hemilabs/x/tss/v3/ecdsa/keygen"
	"github.com/hemilabs/x/tss/v3/ecdsa/resharing"
	"github.com/hemilabs/x/tss/v3/ecdsa/signing"
	"github.com/hemilabs/x/tss/v3/tss"
)

// TestECDSAKeygenSignReshare demonstrates the full ECDSA lifecycle:
// keygen → sign → reshare (with overlapping committees) → sign again.
//
// Old committee: [P0, P1, P2], threshold=1 (2-of-3)
// New committee: [P1, P2, P3], threshold=1 (2-of-3)
// P1 and P2 are in both committees.
// P0 drops out, P3 joins.
// The ECDSA public key is preserved across the reshare.
func TestECDSAKeygenSignReshare(t *testing.T) {
	const threshold = 1
	ctx := context.Background()

	// ------------------------------------------------------------------
	// Phase 1: Paillier pre-parameters
	// ------------------------------------------------------------------
	t.Log("generating Paillier pre-params for 4 parties...")
	allPreParams := make([]keygen.LocalPreParams, 4)
	for i := range allPreParams {
		pp, err := keygen.GeneratePreParams(5 * time.Minute)
		if err != nil {
			t.Fatalf("GeneratePreParams[%d]: %v", i, err)
		}
		allPreParams[i] = *pp
	}
	t.Log("pre-params ready")

	// ------------------------------------------------------------------
	// Phase 2: Party IDs with separate copies per committee.
	// SortPartyIDs assigns Index by position — shared PartyID objects
	// would get their Index mutated by the second sort.
	// ------------------------------------------------------------------
	allPIDs := tss.GenerateTestPartyIDs(4)
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

	// Map allPIDs index → pre-params for each party.
	// oldPIDs uses allPIDs[0,1,2], newPIDs uses allPIDs[1,2,3].
	oldPreParams := []keygen.LocalPreParams{allPreParams[0], allPreParams[1], allPreParams[2]}
	newPreParams := []keygen.LocalPreParams{allPreParams[1], allPreParams[2], allPreParams[3]}

	// ------------------------------------------------------------------
	// Phase 3: Keygen (4 rounds)
	// ------------------------------------------------------------------
	oldSaves := ecdsaKeygen(t, ctx, oldN, threshold, oldPIDs, oldCtx, oldPreParams)
	pubKey := oldSaves[0].ECDSAPub
	t.Logf("keygen: ECDSAPub = (%x...)", pubKey.X().Bytes()[:8])

	// ------------------------------------------------------------------
	// Phase 4: Sign with old committee
	// ------------------------------------------------------------------
	msg1 := sha256.Sum256([]byte("pre-reshare message"))
	sig1 := ecdsaSign(t, ctx, oldN, threshold, oldPIDs, oldCtx, oldSaves, new(big.Int).SetBytes(msg1[:]))
	verifyECDSA(t, pubKey, msg1[:], sig1)
	t.Log("pre-reshare signature verified")

	// ------------------------------------------------------------------
	// Phase 5: Reshare — old [P0,P1,P2] → new [P1,P2,P3]
	// ------------------------------------------------------------------
	newSaves := ecdsaReshare(t, ctx, oldPIDs, newPIDs, oldCtx, newCtx,
		oldSaves, oldPreParams, newPreParams, threshold, threshold)

	for i := 0; i < newN; i++ {
		if !newSaves[i].ECDSAPub.Equals(pubKey) {
			t.Fatalf("new party %d: pub key changed", i)
		}
	}
	if oldSaves[0].Xi.Sign() != 0 {
		t.Fatal("P0 Xi not zeroed")
	}
	t.Log("reshare complete, pub key preserved")

	// ------------------------------------------------------------------
	// Phase 6: Sign with new committee
	// ------------------------------------------------------------------
	msg2 := sha256.Sum256([]byte("post-reshare message"))
	sig2 := ecdsaSign(t, ctx, newN, threshold, newPIDs, newCtx, newSaves, new(big.Int).SetBytes(msg2[:]))
	verifyECDSA(t, pubKey, msg2[:], sig2)
	t.Log("post-reshare signature verified")
}

// --- helpers ---

// ecPoint is a point on an elliptic curve with X and Y coordinates.
type ecPoint interface {
	X() *big.Int
	Y() *big.Int
}

func verifyECDSA(t *testing.T, pub ecPoint, msgHash []byte, sig *signing.SignatureData) {
	t.Helper()
	pk := &ecdsa.PublicKey{Curve: tss.S256(), X: pub.X(), Y: pub.Y()}
	r := new(big.Int).SetBytes(sig.R)
	s := new(big.Int).SetBytes(sig.S)
	if !ecdsa.Verify(pk, msgHash, r, s) {
		t.Fatal("ECDSA signature verification failed")
	}
}

func ecdsaKeygen(t *testing.T, ctx context.Context, n, threshold int, pIDs tss.SortedPartyIDs, peerCtx *tss.PeerContext, preParams []keygen.LocalPreParams) []keygen.LocalPartySaveData {
	t.Helper()

	states := make([]*keygen.KeygenState, n)
	r1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.S256(), peerCtx, pIDs[i], n, threshold)
		st, out, err := keygen.Round1(ctx, params, preParams[i])
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
		params := tss.NewParameters(tss.S256(), peerCtx, pIDs[i], n, threshold)
		params.SetNoProofMod()
		params.SetNoProofFac()
		params.SetNoProofDLN()
		out, err := keygen.Round2(ctx, states[i], r1)
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

	r3 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := keygen.Round3(ctx, states[i], r2p2p[i], r2bcast)
		if err != nil {
			t.Fatalf("keygen.Round3[%d]: %v", i, err)
		}
		r3[i] = out.Messages[0]
	}

	saves := make([]keygen.LocalPartySaveData, n)
	for i := 0; i < n; i++ {
		out, err := keygen.Round4(ctx, states[i], r3)
		if err != nil {
			t.Fatalf("keygen.Round4[%d]: %v", i, err)
		}
		saves[i] = *out.Save
	}
	return saves
}

func ecdsaSign(t *testing.T, ctx context.Context, n, threshold int, pIDs tss.SortedPartyIDs, peerCtx *tss.PeerContext, saves []keygen.LocalPartySaveData, m *big.Int) *signing.SignatureData {
	t.Helper()

	states := make([]*signing.SigningState, n)
	r1p2p := make([][]*tss.Message, n)
	r1bcast := make([]*tss.Message, n)
	for i := range r1p2p {
		r1p2p[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.S256(), peerCtx, pIDs[i], n, threshold)
		st, out, err := signing.SignRound1(params, saves[i], m, nil, 0)
		if err != nil {
			t.Fatalf("SignRound1[%d]: %v", i, err)
		}
		states[i] = st
		for _, msg := range out.Messages {
			if msg.To == nil {
				r1bcast[i] = msg
			} else {
				for _, to := range msg.To {
					r1p2p[to.Index][i] = msg
				}
			}
		}
	}

	// Round 2 (MtA — P2P)
	r2p2p := make([][]*tss.Message, n)
	for i := range r2p2p {
		r2p2p[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := signing.SignRound2(ctx, states[i], r1p2p[i], r1bcast)
		if err != nil {
			t.Fatalf("SignRound2[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			for _, to := range msg.To {
				r2p2p[to.Index][i] = msg
			}
		}
	}

	// Round 3 (broadcast)
	r3 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := signing.SignRound3(ctx, states[i], r2p2p[i])
		if err != nil {
			t.Fatalf("SignRound3[%d]: %v", i, err)
		}
		r3[i] = out.Messages[0]
	}

	// Rounds 4-9 + finalize (all broadcast)
	r4 := bcastRound(t, n, states, func(i int) (*signing.SignRoundOutput, error) {
		return signing.SignRound4(states[i], r3)
	}, "Round4")
	r5 := bcastRound(t, n, states, func(i int) (*signing.SignRoundOutput, error) {
		return signing.SignRound5(states[i], r4)
	}, "Round5")
	r6 := bcastRound(t, n, states, func(i int) (*signing.SignRoundOutput, error) {
		return signing.SignRound6(states[i])
	}, "Round6")
	r7 := bcastRound(t, n, states, func(i int) (*signing.SignRoundOutput, error) {
		return signing.SignRound7(states[i], r5, r6)
	}, "Round7")
	r8 := bcastRound(t, n, states, func(i int) (*signing.SignRoundOutput, error) {
		return signing.SignRound8(states[i])
	}, "Round8")
	r9 := bcastRound(t, n, states, func(i int) (*signing.SignRoundOutput, error) {
		return signing.SignRound9(states[i], r7, r8)
	}, "Round9")

	// Finalize
	out, err := signing.SignFinalize(states[0], r9)
	if err != nil {
		t.Fatalf("SignFinalize: %v", err)
	}
	return out.Signature
}

func bcastRound(t *testing.T, n int, states []*signing.SigningState, fn func(int) (*signing.SignRoundOutput, error), name string) []*tss.Message {
	t.Helper()
	msgs := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := fn(i)
		if err != nil {
			t.Fatalf("%s[%d]: %v", name, i, err)
		}
		msgs[i] = out.Messages[0]
	}
	return msgs
}

func ecdsaReshare(t *testing.T, ctx context.Context, oldPIDs, newPIDs tss.SortedPartyIDs, oldCtx, newCtx *tss.PeerContext, oldSaves []keygen.LocalPartySaveData, oldPreParams, newPreParams []keygen.LocalPreParams, oldT, newT int) []keygen.LocalPartySaveData {
	t.Helper()

	oldN := len(oldPIDs)
	newN := len(newPIDs)

	type partyRole struct {
		pid    *tss.PartyID
		oldIdx int
		newIdx int
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
			pr.newIdx = i
		} else {
			pr := &partyRole{pid: pid, oldIdx: -1, newIdx: i}
			seen[key] = pr
			allParties = append(allParties, pr)
		}
	}

	type stateEntry struct {
		state *resharing.ReshareState
		role  *partyRole
	}
	entries := make([]stateEntry, len(allParties))

	// --- Round 1 ---
	r1Msgs := make([]*tss.Message, oldN)
	for idx, pr := range allParties {
		params := tss.NewReSharingParameters(
			tss.S256(), oldCtx, newCtx, pr.pid, oldN, oldT, newN, newT)
		params.SetNoProofMod()
		params.SetNoProofFac()
		params.SetNoProofDLN()
		var key keygen.LocalPartySaveData
		var pp keygen.LocalPreParams
		if pr.oldIdx >= 0 {
			key = oldSaves[pr.oldIdx]
		} else {
			key = keygen.NewLocalPartySaveData(oldN)
		}
		if pr.newIdx >= 0 {
			pp = newPreParams[pr.newIdx]
		}
		st, out, err := resharing.ReshareRound1(params, key, pp)
		if err != nil {
			t.Fatalf("ReshareRound1[%s]: %v", pr.pid.Id, err)
		}
		entries[idx] = stateEntry{state: st, role: pr}
		if pr.oldIdx >= 0 && len(out.Messages) > 0 {
			r1Msgs[pr.oldIdx] = out.Messages[0]
		}
	}

	// --- Round 2 ---
	r2Msg1s := make([]*tss.Message, newN)
	r2Msg2s := make([]*tss.Message, newN)
	for idx := range entries {
		pr := entries[idx].role
		out, err := resharing.ReshareRound2(entries[idx].state, r1Msgs)
		if err != nil {
			t.Fatalf("ReshareRound2[%s]: %v", pr.pid.Id, err)
		}
		if pr.newIdx >= 0 {
			for _, msg := range out.Messages {
				switch msg.Content.(type) {
				case *resharing.DGRound2Message1:
					r2Msg1s[pr.newIdx] = msg
				case *resharing.DGRound2Message2:
					r2Msg2s[pr.newIdx] = msg
				}
			}
		}
	}

	// --- Round 3 ---
	r3P2P := make([][]*tss.Message, newN)
	r3Bcast := make([]*tss.Message, oldN)
	for i := range r3P2P {
		r3P2P[i] = make([]*tss.Message, oldN)
	}
	for idx := range entries {
		pr := entries[idx].role
		out, err := resharing.ReshareRound3(entries[idx].state, r2Msg2s)
		if err != nil {
			t.Fatalf("ReshareRound3[%s]: %v", pr.pid.Id, err)
		}
		if pr.oldIdx >= 0 {
			for _, msg := range out.Messages {
				switch msg.Content.(type) {
				case *resharing.DGRound3Message2:
					r3Bcast[pr.oldIdx] = msg
				case *resharing.DGRound3Message1:
					for _, to := range msg.To {
						r3P2P[to.Index][pr.oldIdx] = msg
					}
				}
			}
		}
	}

	// --- Round 4 ---
	r4P2P := make([][]*tss.Message, newN)
	r4Bcast := make([]*tss.Message, newN)
	for i := range r4P2P {
		r4P2P[i] = make([]*tss.Message, newN)
	}
	for idx := range entries {
		pr := entries[idx].role
		var myR3P2P []*tss.Message
		if pr.newIdx >= 0 {
			myR3P2P = r3P2P[pr.newIdx]
		}
		out, err := resharing.ReshareRound4(ctx, entries[idx].state, r2Msg1s, myR3P2P, r3Bcast)
		if err != nil {
			t.Fatalf("ReshareRound4[%s]: %v", pr.pid.Id, err)
		}
		if pr.newIdx >= 0 {
			for _, msg := range out.Messages {
				switch msg.Content.(type) {
				case *resharing.DGRound4Message1:
					for _, to := range msg.To {
						r4P2P[to.Index][pr.newIdx] = msg
					}
				case *resharing.DGRound4Message2:
					r4Bcast[pr.newIdx] = msg
				}
			}
		}
	}

	// --- Round 5 ---
	newSaves := make([]keygen.LocalPartySaveData, newN)
	for idx := range entries {
		pr := entries[idx].role
		var myR4P2P []*tss.Message
		if pr.newIdx >= 0 {
			myR4P2P = r4P2P[pr.newIdx]
		}
		out, err := resharing.ReshareRound5(entries[idx].state, myR4P2P, r4Bcast)
		if err != nil {
			t.Fatalf("ReshareRound5[%s]: %v", pr.pid.Id, err)
		}
		if pr.newIdx >= 0 {
			newSaves[pr.newIdx] = *out.Save
		}
	}

	return newSaves
}
