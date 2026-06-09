// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

// Command tss-ecdsa-demo is a reference implementation of the tss-lib
// v3 ECDSA round function API.  It runs the full lifecycle in a single
// process: keygen → sign → reshare (overlapping committees) → sign.
//
// Note: Paillier safe-prime generation takes 10-60 seconds per party.
//
// Usage:
//
//	go run ./cmd/tss-ecdsa-demo

package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/hemilabs/x/tss-lib/v3/ecdsa/keygen"
	"github.com/hemilabs/x/tss-lib/v3/ecdsa/resharing"
	"github.com/hemilabs/x/tss-lib/v3/ecdsa/signing"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	const minSigners = 2 // 2-of-3
	// tss-lib threshold parameter is t where t+1 parties sign.
	const threshold = minSigners - 1
	ctx := context.Background()

	// ----------------------------------------------------------------
	// Party setup.  4 parties total — 3 old, 1 new joiner.
	//
	// Old committee: [P0, P1, P2]
	// New committee: [P1, P2, P3]   (P1, P2 overlap)
	// ----------------------------------------------------------------
	allPIDs := tss.GenerateTestPartyIDs(4)
	copyPID := func(src *tss.PartyID) *tss.PartyID {
		return tss.NewPartyID(src.Id, src.Moniker,
			new(big.Int).SetBytes(src.Key))
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

	// ================================================================
	// PRE-PARAMETERS — Paillier safe primes (slow, do out-of-band)
	// ================================================================
	fmt.Println("=== Generating Paillier pre-parameters (4 parties) ===")
	allPreParams := make([]keygen.LocalPreParams, 4)
	for i := range allPreParams {
		start := time.Now()
		pp, err := keygen.GeneratePreParams(5 * time.Minute)
		if err != nil {
			return fmt.Errorf("pre-params[%d]: %w", i, err)
		}
		allPreParams[i] = *pp
		fmt.Printf("  party %d: %.1fs\n", i, time.Since(start).Seconds())
	}
	oldPP := allPreParams[:3]
	newPP := []keygen.LocalPreParams{allPreParams[1], allPreParams[2], allPreParams[3]}

	// ================================================================
	// KEYGEN — 4 rounds
	// ================================================================
	fmt.Println("\n=== ECDSA Keygen (4 rounds) ===")
	fmt.Printf("  parties: %d, threshold: %d-of-%d\n",
		oldN, minSigners, oldN)

	saves, err := ecdsaKeygen(ctx, oldN, threshold, oldPIDs, oldCtx, oldPP)
	if err != nil {
		return fmt.Errorf("keygen: %w", err)
	}
	pubKey := saves[0].ECDSAPub
	fmt.Printf("  public key: (%x...)\n", pubKey.X().Bytes()[:8])

	// ================================================================
	// SIGN — 9 rounds + finalize
	// ================================================================
	fmt.Println("\n=== ECDSA Sign (9 rounds + finalize) ===")
	msg1 := sha256.Sum256([]byte("pre-reshare message"))
	sig1, err := ecdsaSign(ctx, oldN, threshold, oldPIDs, oldCtx, saves,
		new(big.Int).SetBytes(msg1[:]))
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}
	if err := verifyECDSA(pubKey, msg1[:], sig1); err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	fmt.Printf("  message:   %x\n", msg1[:8])
	fmt.Printf("  signature: R=%x S=%x\n", sig1.R[:8], sig1.S[:8])
	fmt.Println("  verified:  OK")

	// ================================================================
	// RESHARE — 5 rounds, overlapping committees
	// ================================================================
	fmt.Println("\n=== ECDSA Reshare (5 rounds, overlapping) ===")
	fmt.Printf("  old committee: [%s, %s, %s]\n",
		oldPIDs[0].Id, oldPIDs[1].Id, oldPIDs[2].Id)
	fmt.Printf("  new committee: [%s, %s, %s]\n",
		newPIDs[0].Id, newPIDs[1].Id, newPIDs[2].Id)

	newSaves, err := ecdsaReshare(ctx, oldPIDs, newPIDs, oldCtx, newCtx,
		saves, oldPP, newPP, threshold, threshold)
	if err != nil {
		return fmt.Errorf("reshare: %w", err)
	}
	if !newSaves[0].ECDSAPub.Equals(pubKey) {
		return fmt.Errorf("public key changed after reshare")
	}
	fmt.Printf("  public key preserved: (%x...)\n", pubKey.X().Bytes()[:8])

	// ================================================================
	// SIGN AGAIN — with new committee
	// ================================================================
	fmt.Println("\n=== ECDSA Sign (new committee) ===")
	msg2 := sha256.Sum256([]byte("post-reshare message"))
	sig2, err := ecdsaSign(ctx, newN, threshold, newPIDs, newCtx, newSaves,
		new(big.Int).SetBytes(msg2[:]))
	if err != nil {
		return fmt.Errorf("sign2: %w", err)
	}
	if err := verifyECDSA(pubKey, msg2[:], sig2); err != nil {
		return fmt.Errorf("verify2: %w", err)
	}
	fmt.Printf("  message:   %x\n", msg2[:8])
	fmt.Printf("  signature: R=%x S=%x\n", sig2.R[:8], sig2.S[:8])
	fmt.Println("  verified:  OK")

	fmt.Println("\n=== SUCCESS ===")
	return nil
}

// -------------------------------------------------------------------
// Keygen: 4 rounds
//
// Round 1: VSS polynomial + commitment, Paillier pub key
// Round 2: P2P shares + decommitment + Schnorr proof + DLN proofs
// Round 3: verify decommitments, Schnorr proofs, shares
// Round 4: verify Paillier/mod/fac proofs, save
// -------------------------------------------------------------------

func ecdsaKeygen(ctx context.Context, n, threshold int, pIDs tss.SortedPartyIDs, peerCtx *tss.PeerContext, preParams []keygen.LocalPreParams) ([]keygen.LocalPartySaveData, error) {
	// -- Round 1 --
	states := make([]*keygen.KeygenState, n)
	r1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.S256(), peerCtx, pIDs[i], n, threshold)
		st, out, err := keygen.Round1(ctx, params, preParams[i])
		if err != nil {
			return nil, fmt.Errorf("round1[%d]: %w", i, err)
		}
		states[i] = st
		r1[i] = out.Messages[0]
	}

	// -- Round 2 --
	r2p2p := make([][]*tss.Message, n)
	r2bcast := make([]*tss.Message, n)
	for i := range r2p2p {
		r2p2p[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := keygen.Round2(ctx, states[i], r1)
		if err != nil {
			return nil, fmt.Errorf("round2[%d]: %w", i, err)
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

	// -- Round 3 --
	r3 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := keygen.Round3(ctx, states[i], r2p2p[i], r2bcast)
		if err != nil {
			return nil, fmt.Errorf("round3[%d]: %w", i, err)
		}
		r3[i] = out.Messages[0]
	}

	// -- Round 4 --
	saves := make([]keygen.LocalPartySaveData, n)
	for i := 0; i < n; i++ {
		out, err := keygen.Round4(ctx, states[i], r3)
		if err != nil {
			return nil, fmt.Errorf("round4[%d]: %w", i, err)
		}
		saves[i] = *out.Save
	}
	return saves, nil
}

// -------------------------------------------------------------------
// Sign: 9 rounds + finalize
//
// Round 1: k, gamma, MtA ciphertext (P2P) + commitment (broadcast)
// Round 2: MtA response (P2P)
// Round 3: theta, sigma (broadcast)
// Round 4: Schnorr proof for gamma (broadcast)
// Round 5: verify commitments, compute R (broadcast)
// Round 6: Schnorr proof for blinding (broadcast)
// Round 7: verify blinding, commit Ui/Ti (broadcast)
// Round 8: decommit Ui/Ti (broadcast)
// Round 9: verify Ui==Ti, reveal si (broadcast)
// Finalize: sum partial sigs, verify ECDSA signature
// -------------------------------------------------------------------

func ecdsaSign(ctx context.Context, n, threshold int, pIDs tss.SortedPartyIDs, peerCtx *tss.PeerContext, saves []keygen.LocalPartySaveData, m *big.Int) (*signing.SignatureData, error) {
	// -- Round 1: P2P + broadcast --
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
			return nil, fmt.Errorf("signR1[%d]: %w", i, err)
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

	// -- Round 2: P2P --
	r2p2p := make([][]*tss.Message, n)
	for i := range r2p2p {
		r2p2p[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := signing.SignRound2(ctx, states[i], r1p2p[i], r1bcast)
		if err != nil {
			return nil, fmt.Errorf("signR2[%d]: %w", i, err)
		}
		for _, msg := range out.Messages {
			for _, to := range msg.To {
				r2p2p[to.Index][i] = msg
			}
		}
	}

	// -- Round 3: broadcast --
	r3 := bcastRound(n, states, func(i int) (*signing.SignRoundOutput, error) {
		return signing.SignRound3(ctx, states[i], r2p2p[i])
	}, "signR3")
	if r3 == nil {
		return nil, fmt.Errorf("signR3 failed")
	}

	// -- Rounds 4-9: all broadcast --
	r4 := bcastRound(n, states, func(i int) (*signing.SignRoundOutput, error) {
		return signing.SignRound4(states[i], r3)
	}, "signR4")
	r5 := bcastRound(n, states, func(i int) (*signing.SignRoundOutput, error) {
		return signing.SignRound5(states[i], r4)
	}, "signR5")
	r6 := bcastRound(n, states, func(i int) (*signing.SignRoundOutput, error) {
		return signing.SignRound6(states[i])
	}, "signR6")
	r7 := bcastRound(n, states, func(i int) (*signing.SignRoundOutput, error) {
		return signing.SignRound7(states[i], r5, r6)
	}, "signR7")
	r8 := bcastRound(n, states, func(i int) (*signing.SignRoundOutput, error) {
		return signing.SignRound8(states[i])
	}, "signR8")
	r9 := bcastRound(n, states, func(i int) (*signing.SignRoundOutput, error) {
		return signing.SignRound9(states[i], r7, r8)
	}, "signR9")

	// -- Finalize --
	out, err := signing.SignFinalize(states[0], r9)
	if err != nil {
		return nil, fmt.Errorf("signFinalize: %w", err)
	}
	return out.Signature, nil
}

func bcastRound(n int, states []*signing.SigningState, fn func(int) (*signing.SignRoundOutput, error), name string) []*tss.Message {
	msgs := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := fn(i)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s[%d]: %v\n", name, i, err)
			return nil
		}
		msgs[i] = out.Messages[0]
	}
	return msgs
}

// -------------------------------------------------------------------
// Reshare: 5 rounds, overlapping committees
//
// Round 1: old → VSS commitment + ECDSA pub (broadcast to new)
// Round 2: new → Paillier/DLN params (broadcast to new) + ACK (to old)
// Round 3: old → P2P shares + decommitment (to new)
// Round 4: new → verify shares, FacProof P2P + ACK broadcast
// Round 5: save new key material, zero old Xi
// -------------------------------------------------------------------

func ecdsaReshare(ctx context.Context, oldPIDs, newPIDs tss.SortedPartyIDs, oldCtx, newCtx *tss.PeerContext, oldSaves []keygen.LocalPartySaveData, oldPP, newPP []keygen.LocalPreParams, oldT, newT int) ([]keygen.LocalPartySaveData, error) {
	oldN := len(oldPIDs)
	newN := len(newPIDs)

	type party struct {
		pid    *tss.PartyID
		oldIdx int
		newIdx int
		state  *resharing.ReshareState
	}
	seen := make(map[string]*party)
	all := make([]*party, 0, oldN+1) // at most oldN + new-only parties
	for i, pid := range oldPIDs {
		key := fmt.Sprintf("%x", pid.Key)
		p := &party{pid: pid, oldIdx: i, newIdx: -1}
		seen[key] = p
		all = append(all, p)
	}
	for i, pid := range newPIDs {
		key := fmt.Sprintf("%x", pid.Key)
		if p, ok := seen[key]; ok {
			p.newIdx = i
		} else {
			p := &party{pid: pid, oldIdx: -1, newIdx: i}
			seen[key] = p
			all = append(all, p)
		}
	}

	// -- Round 1 --
	r1Msgs := make([]*tss.Message, oldN)
	for _, p := range all {
		params := tss.NewReSharingParameters(
			tss.S256(), oldCtx, newCtx, p.pid,
			oldN, oldT, newN, newT)
		params.SetNoProofMod()
		params.SetNoProofFac()
		params.SetNoProofDLN()
		var key keygen.LocalPartySaveData
		var pp keygen.LocalPreParams
		if p.oldIdx >= 0 {
			key = oldSaves[p.oldIdx]
		} else {
			key = keygen.NewLocalPartySaveData(oldN)
		}
		if p.newIdx >= 0 {
			pp = newPP[p.newIdx]
		}
		st, out, err := resharing.ReshareRound1(params, key, pp)
		if err != nil {
			return nil, fmt.Errorf("reshareR1[%s]: %w", p.pid.Id, err)
		}
		p.state = st
		if p.oldIdx >= 0 && len(out.Messages) > 0 {
			r1Msgs[p.oldIdx] = out.Messages[0]
		}
	}

	// -- Round 2 --
	r2Msg1s := make([]*tss.Message, newN) // DGRound2Message1 (to new)
	r2Msg2s := make([]*tss.Message, newN) // DGRound2Message2 (ACK to old)
	for _, p := range all {
		out, err := resharing.ReshareRound2(p.state, r1Msgs)
		if err != nil {
			return nil, fmt.Errorf("reshareR2[%s]: %w", p.pid.Id, err)
		}
		if p.newIdx >= 0 {
			for _, msg := range out.Messages {
				switch msg.Content.(type) {
				case *resharing.DGRound2Message1:
					r2Msg1s[p.newIdx] = msg
				case *resharing.DGRound2Message2:
					r2Msg2s[p.newIdx] = msg
				}
			}
		}
	}

	// -- Round 3 --
	r3P2P := make([][]*tss.Message, newN)
	r3Bcast := make([]*tss.Message, oldN)
	for i := range r3P2P {
		r3P2P[i] = make([]*tss.Message, oldN)
	}
	for _, p := range all {
		out, err := resharing.ReshareRound3(p.state, r2Msg2s)
		if err != nil {
			return nil, fmt.Errorf("reshareR3[%s]: %w", p.pid.Id, err)
		}
		if p.oldIdx >= 0 {
			for _, msg := range out.Messages {
				switch msg.Content.(type) {
				case *resharing.DGRound3Message2:
					r3Bcast[p.oldIdx] = msg
				case *resharing.DGRound3Message1:
					for _, to := range msg.To {
						r3P2P[to.Index][p.oldIdx] = msg
					}
				}
			}
		}
	}

	// -- Round 4 --
	r4P2P := make([][]*tss.Message, newN)
	r4Bcast := make([]*tss.Message, newN)
	for i := range r4P2P {
		r4P2P[i] = make([]*tss.Message, newN)
	}
	for _, p := range all {
		var myR3P2P []*tss.Message
		if p.newIdx >= 0 {
			myR3P2P = r3P2P[p.newIdx]
		}
		out, err := resharing.ReshareRound4(ctx, p.state, r2Msg1s, myR3P2P, r3Bcast)
		if err != nil {
			return nil, fmt.Errorf("reshareR4[%s]: %w", p.pid.Id, err)
		}
		if p.newIdx >= 0 {
			for _, msg := range out.Messages {
				switch msg.Content.(type) {
				case *resharing.DGRound4Message1:
					for _, to := range msg.To {
						r4P2P[to.Index][p.newIdx] = msg
					}
				case *resharing.DGRound4Message2:
					r4Bcast[p.newIdx] = msg
				}
			}
		}
	}

	// -- Round 5 --
	newSaves := make([]keygen.LocalPartySaveData, newN)
	for _, p := range all {
		var myR4P2P []*tss.Message
		if p.newIdx >= 0 {
			myR4P2P = r4P2P[p.newIdx]
		}
		out, err := resharing.ReshareRound5(p.state, myR4P2P, r4Bcast)
		if err != nil {
			return nil, fmt.Errorf("reshareR5[%s]: %w", p.pid.Id, err)
		}
		if p.newIdx >= 0 {
			newSaves[p.newIdx] = *out.Save
		}
	}
	return newSaves, nil
}

// ecPoint is a point on an elliptic curve with X and Y coordinates.
type ecPoint interface {
	X() *big.Int
	Y() *big.Int
}

func verifyECDSA(pub ecPoint, msg []byte, sig *signing.SignatureData) error {
	pk := &ecdsa.PublicKey{Curve: tss.S256(), X: pub.X(), Y: pub.Y()}
	r := new(big.Int).SetBytes(sig.R)
	s := new(big.Int).SetBytes(sig.S)
	if !ecdsa.Verify(pk, msg, r, s) {
		return fmt.Errorf("ECDSA signature verification failed")
	}
	return nil
}
