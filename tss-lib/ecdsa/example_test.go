// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

//go:build tssexamples

// Package ecdsa_test contains the canonical usage example for the
// tss-lib v3 ECDSA round function API.
//
// Run with: go test -tags tssexamples -v -run TestECDSAKeygenAndSign ./ecdsa/ -timeout 10m
package ecdsa_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"math/big"
	"testing"
	"time"

	"github.com/hemilabs/x/tss-lib/v3/ecdsa/keygen"
	"github.com/hemilabs/x/tss-lib/v3/ecdsa/signing"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// TestECDSAKeygenAndSign demonstrates end-to-end ECDSA threshold
// key generation and signing using the v3 round function API.
//
// This is a 3-party, 2-of-3 threshold scheme: 3 parties generate
// a shared key, then all 3 cooperate to sign a message.  The
// resulting ECDSA signature is verified against the distributed
// public key.
//
// The v3 API replaces the old channel-based NewLocalParty / Start /
// outCh / endCh pattern with explicit round functions: each round
// takes state + inbound messages and returns outbound messages.
// The caller owns the event loop.
func TestECDSAKeygenAndSign(t *testing.T) {
	const n = 3
	const threshold = 1 // t+1 = 2 signers needed
	ctx := context.Background()

	// ------------------------------------------------------------------
	// Phase 1: Pre-parameters
	//
	// Generate Paillier pre-parameters for each party.  This is CPU-
	// intensive (safe-prime generation) and should be done out-of-band
	// in production, not during a ceremony.
	// ------------------------------------------------------------------
	preParams := make([]keygen.LocalPreParams, n)
	for i := range preParams {
		pp, err := keygen.GeneratePreParams(5 * time.Minute)
		if err != nil {
			t.Fatalf("GeneratePreParams[%d]: %v", i, err)
		}
		preParams[i] = *pp
	}

	// ------------------------------------------------------------------
	// Phase 2: Party IDs + peer context
	//
	// In production, each party's ID is derived from its identity
	// (e.g. a public key hash).  For testing, GenerateTestPartyIDs
	// creates deterministic IDs.
	// ------------------------------------------------------------------
	pIDs := tss.GenerateTestPartyIDs(n)
	peerCtx := tss.NewPeerContext(pIDs)

	// ------------------------------------------------------------------
	// Phase 3: Distributed key generation (4 rounds)
	// ------------------------------------------------------------------
	saves := ecdsaKeygen(t, ctx, n, threshold, pIDs, peerCtx, preParams)

	t.Logf("keygen complete: ECDSAPub = (%x, %x)",
		saves[0].ECDSAPub.X(), saves[0].ECDSAPub.Y())

	// ------------------------------------------------------------------
	// Phase 4: Threshold signing (9 rounds + finalize)
	// ------------------------------------------------------------------
	msgHash := sha256.Sum256([]byte("hello v3 round functions"))
	m := new(big.Int).SetBytes(msgHash[:])

	sig := ecdsaSign(t, ctx, n, threshold, pIDs, peerCtx, saves, m)

	// ------------------------------------------------------------------
	// Phase 5: Verify the ECDSA signature
	// ------------------------------------------------------------------
	pk := ecdsa.PublicKey{
		Curve: tss.S256(),
		X:     saves[0].ECDSAPub.X(),
		Y:     saves[0].ECDSAPub.Y(),
	}
	r := new(big.Int).SetBytes(sig.R)
	s := new(big.Int).SetBytes(sig.S)
	if !ecdsa.Verify(&pk, msgHash[:], r, s) {
		t.Fatal("ECDSA signature verification failed")
	}
	t.Logf("signature verified: r=%x s=%x", sig.R, sig.S)
}

// ecdsaKeygen runs the 4-round key generation protocol for n parties.
func ecdsaKeygen(
	t *testing.T,
	ctx context.Context,
	n, threshold int,
	pIDs tss.SortedPartyIDs,
	peerCtx *tss.PeerContext,
	preParams []keygen.LocalPreParams,
) []keygen.LocalPartySaveData {
	t.Helper()

	// --- Round 1: Commitment ---
	states := make([]*keygen.KeygenState, n)
	r1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.S256(), peerCtx, pIDs[i], n, threshold)
		st, out, err := keygen.Round1(ctx, params, preParams[i])
		if err != nil {
			t.Fatalf("Round1[%d]: %v", i, err)
		}
		states[i] = st
		r1[i] = out.Messages[0]
	}

	// --- Round 2: VSS shares (P2P) + decommitments (broadcast) ---
	r2p2p := make([][]*tss.Message, n)
	r2bcast := make([]*tss.Message, n)
	for i := range r2p2p {
		r2p2p[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := keygen.Round2(ctx, states[i], r1)
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

	// --- Round 3: Feldman VSS verification ---
	r3 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := keygen.Round3(ctx, states[i], r2p2p[i], r2bcast)
		if err != nil {
			t.Fatalf("Round3[%d]: %v", i, err)
		}
		r3[i] = out.Messages[0]
	}

	// --- Round 4: Paillier proof verification + save ---
	saves := make([]keygen.LocalPartySaveData, n)
	for i := 0; i < n; i++ {
		out, err := keygen.Round4(ctx, states[i], r3)
		if err != nil {
			t.Fatalf("Round4[%d]: %v", i, err)
		}
		saves[i] = *out.Save
	}
	return saves
}

// ecdsaSign runs the 9-round + finalize signing protocol.
func ecdsaSign(
	t *testing.T,
	ctx context.Context,
	n, threshold int,
	pIDs tss.SortedPartyIDs,
	peerCtx *tss.PeerContext,
	saves []keygen.LocalPartySaveData,
	m *big.Int,
) *signing.SignatureData {
	t.Helper()

	// --- Round 1: k, gamma, commitment ---
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

	// --- Round 2: MtA (multiplicative-to-additive) ---
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

	// --- Round 3: theta, sigma ---
	r3 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := signing.SignRound3(ctx, states[i], r2p2p[i])
		if err != nil {
			t.Fatalf("SignRound3[%d]: %v", i, err)
		}
		r3[i] = out.Messages[0]
	}

	// --- Round 4: Schnorr proof for gamma ---
	r4 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := signing.SignRound4(states[i], r3)
		if err != nil {
			t.Fatalf("SignRound4[%d]: %v", i, err)
		}
		r4[i] = out.Messages[0]
	}

	// --- Round 5: verify commitments, compute R ---
	r5 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := signing.SignRound5(states[i], r4)
		if err != nil {
			t.Fatalf("SignRound5[%d]: %v", i, err)
		}
		r5[i] = out.Messages[0]
	}

	// --- Round 6: Schnorr proof for blinding ---
	r6 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := signing.SignRound6(states[i])
		if err != nil {
			t.Fatalf("SignRound6[%d]: %v", i, err)
		}
		r6[i] = out.Messages[0]
	}

	// --- Round 7: verify blinding, commit Ui/Ti ---
	r7 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := signing.SignRound7(states[i], r5, r6)
		if err != nil {
			t.Fatalf("SignRound7[%d]: %v", i, err)
		}
		r7[i] = out.Messages[0]
	}

	// --- Round 8: decommit Ui/Ti ---
	r8 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := signing.SignRound8(states[i])
		if err != nil {
			t.Fatalf("SignRound8[%d]: %v", i, err)
		}
		r8[i] = out.Messages[0]
	}

	// --- Round 9: verify Ui==Ti, reveal si ---
	r9 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := signing.SignRound9(states[i], r7, r8)
		if err != nil {
			t.Fatalf("SignRound9[%d]: %v", i, err)
		}
		r9[i] = out.Messages[0]
	}

	// --- Finalize: sum partial sigs ---
	out, err := signing.SignFinalize(states[0], r9)
	if err != nil {
		t.Fatalf("SignFinalize: %v", err)
	}
	return out.Signature
}
