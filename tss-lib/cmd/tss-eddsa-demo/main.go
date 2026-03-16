// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

// Command tss-eddsa-demo is a reference implementation of the tss-lib
// v3 EdDSA round function API.  It runs the full lifecycle in a single
// process: keygen → sign → reshare (overlapping committees) → sign.
//
// Usage:
//
//	go run ./cmd/tss-eddsa-demo
package main

import (
	"crypto/sha256"
	"fmt"
	"math/big"
	"os"

	"github.com/decred/dcrd/dcrec/edwards/v2"

	"github.com/hemilabs/x/tss-lib/v3/eddsa/keygen"
	"github.com/hemilabs/x/tss-lib/v3/eddsa/resharing"
	"github.com/hemilabs/x/tss-lib/v3/eddsa/signing"
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

	// ----------------------------------------------------------------
	// Party setup.  4 parties total — 3 old, 1 new joiner.
	//
	// Old committee: [P0, P1, P2]
	// New committee: [P1, P2, P3]   (P1, P2 overlap)
	//
	// Each committee needs its own *PartyID copies because
	// SortPartyIDs assigns .Index by sort position — sharing
	// objects between committees corrupts the indices.
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
	// KEYGEN — 3 rounds, no Paillier
	// ================================================================
	fmt.Println("=== EdDSA Keygen (3 rounds) ===")
	fmt.Printf("  parties: %d, threshold: %d-of-%d\n",
		oldN, minSigners, oldN)

	saves, err := eddsaKeygen(oldN, threshold, oldPIDs, oldCtx)
	if err != nil {
		return fmt.Errorf("keygen: %w", err)
	}
	pubKey := saves[0].EDDSAPub
	fmt.Printf("  public key: (%x, %x)\n", pubKey.X(), pubKey.Y())

	// ================================================================
	// SIGN — 3 rounds + finalize
	// ================================================================
	fmt.Println("\n=== EdDSA Sign (3 rounds + finalize) ===")
	msg1 := sha256.Sum256([]byte("pre-reshare message"))
	sig1, err := eddsaSign(oldN, threshold, oldPIDs, oldCtx, saves,
		new(big.Int).SetBytes(msg1[:]))
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}
	if err := verifyEdDSA(pubKey, msg1[:], sig1); err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	fmt.Printf("  message:   %x\n", msg1[:8])
	fmt.Printf("  signature: R=%x S=%x\n", sig1.R[:8], sig1.S[:8])
	fmt.Println("  verified:  OK")

	// ================================================================
	// RESHARE — 5 rounds, overlapping committees
	// ================================================================
	fmt.Println("\n=== EdDSA Reshare (5 rounds, overlapping) ===")
	fmt.Printf("  old committee: [%s, %s, %s]\n",
		oldPIDs[0].Id, oldPIDs[1].Id, oldPIDs[2].Id)
	fmt.Printf("  new committee: [%s, %s, %s]\n",
		newPIDs[0].Id, newPIDs[1].Id, newPIDs[2].Id)

	newSaves, err := eddsaReshare(oldPIDs, newPIDs, oldCtx, newCtx,
		saves, threshold, threshold)
	if err != nil {
		return fmt.Errorf("reshare: %w", err)
	}
	newPubKey := newSaves[0].EDDSAPub
	if !newPubKey.Equals(pubKey) {
		return fmt.Errorf("public key changed after reshare")
	}
	if saves[0].Xi.Sign() != 0 {
		return fmt.Errorf("P0 Xi not zeroed after reshare")
	}
	fmt.Printf("  public key preserved: (%x, %x)\n",
		newPubKey.X(), newPubKey.Y())
	fmt.Println("  old P0 Xi zeroed:     OK")

	// ================================================================
	// SIGN AGAIN — with new committee
	// ================================================================
	fmt.Println("\n=== EdDSA Sign (new committee) ===")
	msg2 := sha256.Sum256([]byte("post-reshare message"))
	sig2, err := eddsaSign(newN, threshold, newPIDs, newCtx, newSaves,
		new(big.Int).SetBytes(msg2[:]))
	if err != nil {
		return fmt.Errorf("sign2: %w", err)
	}
	if err := verifyEdDSA(pubKey, msg2[:], sig2); err != nil {
		return fmt.Errorf("verify2: %w", err)
	}
	fmt.Printf("  message:   %x\n", msg2[:8])
	fmt.Printf("  signature: R=%x S=%x\n", sig2.R[:8], sig2.S[:8])
	fmt.Println("  verified:  OK")

	fmt.Println("\n=== SUCCESS ===")
	return nil
}

// -------------------------------------------------------------------
// Keygen: 3 rounds
//
// Round 1: each party generates VSS polynomial, broadcasts commitment
// Round 2: each party sends P2P shares, broadcasts decommitment + Schnorr proof
// Round 3: each party verifies proofs and shares, computes public key
// -------------------------------------------------------------------

func eddsaKeygen(
	n, threshold int,
	pIDs tss.SortedPartyIDs,
	ctx *tss.PeerContext,
) ([]keygen.LocalPartySaveData, error) {
	// -- Round 1 --
	states := make([]*keygen.KeygenState, n)
	r1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.Edwards(), ctx, pIDs[i], n, threshold)
		st, out, err := keygen.Round1(params)
		if err != nil {
			return nil, fmt.Errorf("round1[%d]: %w", i, err)
		}
		states[i] = st
		r1[i] = out.Messages[0] // broadcast: commitment
	}

	// -- Round 2 --
	// Produces two kinds of messages:
	//   P2P (msg.To != nil):    VSS share for one specific party
	//   Broadcast (msg.To == nil): decommitment + Schnorr proof
	r2p2p := make([][]*tss.Message, n) // r2p2p[receiver][sender]
	r2bcast := make([]*tss.Message, n) // r2bcast[sender]
	for i := range r2p2p {
		r2p2p[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := keygen.Round2(states[i], r1)
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
		// Own P2P share and broadcast are stored in state for self.
		r2p2p[i][i] = states[i].ExportR2P2PSelf()
		if r2bcast[i] == nil {
			r2bcast[i] = states[i].ExportR2BcastSelf()
		}
	}

	// -- Round 3 --
	saves := make([]keygen.LocalPartySaveData, n)
	for i := 0; i < n; i++ {
		out, err := keygen.Round3(states[i], r2p2p[i], r2bcast)
		if err != nil {
			return nil, fmt.Errorf("round3[%d]: %w", i, err)
		}
		saves[i] = *out.Save
	}
	return saves, nil
}

// -------------------------------------------------------------------
// Sign: 3 rounds + finalize
//
// Round 1: each party picks nonce ri, broadcasts commitment to Ri
// Round 2: each party broadcasts decommitment + Schnorr proof for ri
// Round 3: each party verifies proofs, computes aggregate R,
//          produces partial signature si
// Finalize: sum partial sigs, verify EdDSA signature
// -------------------------------------------------------------------

func eddsaSign(
	n, threshold int,
	pIDs tss.SortedPartyIDs,
	ctx *tss.PeerContext,
	saves []keygen.LocalPartySaveData,
	m *big.Int,
) (*signing.SignatureData, error) {
	// -- Round 1 --
	states := make([]*signing.SigningState, n)
	r1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.Edwards(), ctx, pIDs[i], n, threshold)
		st, out, err := signing.SignRound1(params, saves[i], m, 0)
		if err != nil {
			return nil, fmt.Errorf("signRound1[%d]: %w", i, err)
		}
		states[i] = st
		r1[i] = out.Messages[0] // broadcast: commitment
	}

	// -- Round 2 --
	r2 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := signing.SignRound2(states[i], r1)
		if err != nil {
			return nil, fmt.Errorf("signRound2[%d]: %w", i, err)
		}
		r2[i] = out.Messages[0] // broadcast: decommit + proof
	}

	// -- Round 3 --
	r3 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := signing.SignRound3(states[i], r2)
		if err != nil {
			return nil, fmt.Errorf("signRound3[%d]: %w", i, err)
		}
		r3[i] = out.Messages[0] // broadcast: partial sig si
	}

	// -- Finalize --
	out, err := signing.SignFinalize(states[0], r3)
	if err != nil {
		return nil, fmt.Errorf("signFinalize: %w", err)
	}
	return out.Signature, nil
}

// -------------------------------------------------------------------
// Reshare: 5 rounds, supports overlapping committees
//
// Round 1: old committee computes Lagrange wi, creates VSS for new,
//          broadcasts commitment + EdDSA pub to new committee
// Round 2: new committee validates pub key consistency, ACKs old
// Round 3: old committee sends P2P shares + decommitment to new
// Round 4: new committee verifies shares and commitments, ACKs both
// Round 5: new committee saves new key material, old zeros Xi
// -------------------------------------------------------------------

func eddsaReshare(
	oldPIDs, newPIDs tss.SortedPartyIDs,
	oldCtx, newCtx *tss.PeerContext,
	oldSaves []keygen.LocalPartySaveData,
	oldT, newT int,
) ([]keygen.LocalPartySaveData, error) {
	oldN := len(oldPIDs)
	newN := len(newPIDs)

	// Build participant roster.  A party can be in old, new, or both.
	type party struct {
		pid    *tss.PartyID
		oldIdx int // -1 if new-only
		newIdx int // -1 if old-only
		state  *resharing.ReshareState
	}
	seen := make(map[string]*party)
	all := make([]*party, 0, oldN+1)
	for i, pid := range oldPIDs {
		key := fmt.Sprintf("%x", pid.Key)
		p := &party{pid: pid, oldIdx: i, newIdx: -1}
		seen[key] = p
		all = append(all, p)
	}
	for i, pid := range newPIDs {
		key := fmt.Sprintf("%x", pid.Key)
		if p, ok := seen[key]; ok {
			p.newIdx = i // dual-committee
		} else {
			p := &party{pid: pid, oldIdx: -1, newIdx: i}
			seen[key] = p
			all = append(all, p)
		}
	}

	// -- Round 1: old committee produces, new no-ops --
	r1Msgs := make([]*tss.Message, oldN)
	for _, p := range all {
		params := tss.NewReSharingParameters(
			tss.Edwards(), oldCtx, newCtx, p.pid,
			oldN, oldT, newN, newT)
		var input *keygen.LocalPartySaveData
		if p.oldIdx >= 0 {
			input = &oldSaves[p.oldIdx]
		}
		st, out, err := resharing.ReshareRound1(params, input)
		if err != nil {
			return nil, fmt.Errorf("reshareR1[%s]: %w", p.pid.Id, err)
		}
		p.state = st
		if p.oldIdx >= 0 && len(out.Messages) > 0 {
			r1Msgs[p.oldIdx] = out.Messages[0]
		}
	}

	// -- Round 2: new committee ACKs --
	r2Msgs := make([]*tss.Message, newN)
	for _, p := range all {
		out, err := resharing.ReshareRound2(p.state, r1Msgs)
		if err != nil {
			return nil, fmt.Errorf("reshareR2[%s]: %w", p.pid.Id, err)
		}
		if p.newIdx >= 0 && len(out.Messages) > 0 {
			r2Msgs[p.newIdx] = out.Messages[0]
		}
	}

	// -- Round 3: old sends shares + decommitment --
	r3p2p := make([][]*tss.Message, newN) // r3p2p[newReceiver][oldSender]
	r3bcast := make([]*tss.Message, oldN)
	for i := range r3p2p {
		r3p2p[i] = make([]*tss.Message, oldN)
	}
	for _, p := range all {
		out, err := resharing.ReshareRound3(p.state, r2Msgs)
		if err != nil {
			return nil, fmt.Errorf("reshareR3[%s]: %w", p.pid.Id, err)
		}
		if p.oldIdx >= 0 {
			for _, msg := range out.Messages {
				switch msg.Content.(type) {
				case *resharing.DGRound3Message2:
					r3bcast[p.oldIdx] = msg
				case *resharing.DGRound3Message1:
					for _, to := range msg.To {
						r3p2p[to.Index][p.oldIdx] = msg
					}
				}
			}
		}
	}

	// -- Round 4: new committee verifies --
	r4Msgs := make([]*tss.Message, newN)
	for _, p := range all {
		var myP2P []*tss.Message
		if p.newIdx >= 0 {
			myP2P = r3p2p[p.newIdx]
		}
		out, err := resharing.ReshareRound4(p.state, r1Msgs, myP2P, r3bcast)
		if err != nil {
			return nil, fmt.Errorf("reshareR4[%s]: %w", p.pid.Id, err)
		}
		if p.newIdx >= 0 && len(out.Messages) > 0 {
			r4Msgs[p.newIdx] = out.Messages[0]
		}
	}

	// -- Round 5: save --
	newSaves := make([]keygen.LocalPartySaveData, newN)
	for _, p := range all {
		out, err := resharing.ReshareRound5(p.state, r4Msgs)
		if err != nil {
			return nil, fmt.Errorf("reshareR5[%s]: %w", p.pid.Id, err)
		}
		if p.newIdx >= 0 {
			newSaves[p.newIdx] = *out.Save
		}
	}
	return newSaves, nil
}

func verifyEdDSA(
	pub interface {
		X() *big.Int
		Y() *big.Int
	},
	msg []byte,
	sig *signing.SignatureData,
) error {
	pk := edwards.PublicKey{Curve: tss.Edwards(), X: pub.X(), Y: pub.Y()}
	r := new(big.Int).SetBytes(sig.R)
	s := new(big.Int).SetBytes(sig.S)
	if !edwards.Verify(&pk, msg, r, s) {
		return fmt.Errorf("EdDSA signature verification failed")
	}
	return nil
}
