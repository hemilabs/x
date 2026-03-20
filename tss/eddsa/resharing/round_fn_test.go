// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package resharing

import (
	"crypto/sha256"
	"math/big"
	"testing"

	"github.com/decred/dcrd/dcrec/edwards/v2"

	"github.com/hemilabs/x/tss/v3/eddsa/keygen"
	"github.com/hemilabs/x/tss/v3/eddsa/signing"
	"github.com/hemilabs/x/tss/v3/tss"
)

// TestRoundFnEdDSAReshareAndSign runs a full reshare from a 3-party
// old committee (t=1) to a 3-party new committee (t=1), then signs
// with the new committee and verifies the signature.
func TestRoundFnEdDSAReshareAndSign(t *testing.T) {
	const oldN = 3
	const oldT = 1
	const newN = 3
	const newT = 1

	// --- Keygen with old committee ---
	oldPIDs := tss.GenerateTestPartyIDs(oldN)
	oldPeerCtx := tss.NewPeerContext(oldPIDs)
	oldSaves := doKeygen(t, oldN, oldT, oldPIDs, oldPeerCtx)

	oldPubKey := oldSaves[0].EDDSAPub
	t.Logf("old pub key: (%x, %x)", oldPubKey.X(), oldPubKey.Y())

	// --- Create new party IDs (different from old) ---
	newPIDs := tss.GenerateTestPartyIDs(newN)
	newPeerCtx := tss.NewPeerContext(newPIDs)

	// --- Reshare Round 1 (old committee) ---
	oldStates := make([]*ReshareState, oldN)
	r1Msgs := make([]*tss.Message, oldN)
	for i := 0; i < oldN; i++ {
		params := tss.NewReSharingParameters(
			tss.Edwards(), oldPeerCtx, newPeerCtx,
			oldPIDs[i], oldN, oldT, newN, newT)
		st, out, err := ReshareRound1(params, &oldSaves[i])
		if err != nil {
			t.Fatalf("ReshareRound1[old %d]: %v", i, err)
		}
		oldStates[i] = st
		if len(out.Messages) > 0 {
			r1Msgs[i] = out.Messages[0]
		}
	}

	// New committee also calls Round1 (no-op for them)
	newStates := make([]*ReshareState, newN)
	for i := 0; i < newN; i++ {
		params := tss.NewReSharingParameters(
			tss.Edwards(), oldPeerCtx, newPeerCtx,
			newPIDs[i], oldN, oldT, newN, newT)
		st, _, err := ReshareRound1(params, nil)
		if err != nil {
			t.Fatalf("ReshareRound1[new %d]: %v", i, err)
		}
		newStates[i] = st
	}

	// --- Reshare Round 2 (new committee sends ACK) ---
	r2Msgs := make([]*tss.Message, newN)
	for i := 0; i < newN; i++ {
		out, err := ReshareRound2(newStates[i], r1Msgs)
		if err != nil {
			t.Fatalf("ReshareRound2[new %d]: %v", i, err)
		}
		if len(out.Messages) > 0 {
			r2Msgs[i] = out.Messages[0]
		}
	}
	// Old committee also calls Round2 (no-op)
	for i := 0; i < oldN; i++ {
		if _, err := ReshareRound2(oldStates[i], nil); err != nil {
			t.Fatalf("ReshareRound2[old %d]: %v", i, err)
		}
	}

	// --- Reshare Round 3 (old committee sends shares + decommitment) ---
	r3p2p := make([][]*tss.Message, newN) // [new_receiver][old_sender]
	r3bcast := make([]*tss.Message, oldN) // [old_sender]
	for i := range r3p2p {
		r3p2p[i] = make([]*tss.Message, oldN)
	}
	for i := 0; i < oldN; i++ {
		out, err := ReshareRound3(oldStates[i], r2Msgs)
		if err != nil {
			t.Fatalf("ReshareRound3[old %d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			switch msg.Content.(type) {
			case *DGRound3Message2:
				// Broadcast decommitment.
				r3bcast[i] = msg
			case *DGRound3Message1:
				// P2P share to specific new party.
				for _, to := range msg.To {
					r3p2p[to.Index][i] = msg
				}
			}
		}
	}
	// New committee also calls Round3 (no-op)
	for i := 0; i < newN; i++ {
		if _, err := ReshareRound3(newStates[i], nil); err != nil {
			t.Fatalf("ReshareRound3[new %d]: %v", i, err)
		}
	}

	// --- Reshare Round 4 (new committee verifies + ACK) ---
	r4Msgs := make([]*tss.Message, newN)
	for i := 0; i < newN; i++ {
		out, err := ReshareRound4(newStates[i], r1Msgs, r3p2p[i], r3bcast)
		if err != nil {
			t.Fatalf("ReshareRound4[new %d]: %v", i, err)
		}
		if len(out.Messages) > 0 {
			r4Msgs[i] = out.Messages[0]
		}
	}
	// Old committee also calls Round4 (no-op)
	for i := 0; i < oldN; i++ {
		if _, err := ReshareRound4(oldStates[i], nil, nil, nil); err != nil {
			t.Fatalf("ReshareRound4[old %d]: %v", i, err)
		}
	}

	// --- Reshare Round 5 (save) ---
	newSaves := make([]keygen.LocalPartySaveData, newN)
	for i := 0; i < newN; i++ {
		out, err := ReshareRound5(newStates[i], r4Msgs)
		if err != nil {
			t.Fatalf("ReshareRound5[new %d]: %v", i, err)
		}
		newSaves[i] = *out.Save
	}
	for i := 0; i < oldN; i++ {
		out, err := ReshareRound5(oldStates[i], nil)
		if err != nil {
			t.Fatalf("ReshareRound5[old %d]: %v", i, err)
		}
		// Old Xi should be zeroed.
		if oldSaves[i].Xi.Sign() != 0 {
			t.Fatalf("old party %d Xi not zeroed", i)
		}
		_ = out
	}

	// Verify new saves: same pub key, valid data.
	for i := 0; i < newN; i++ {
		if !newSaves[i].EDDSAPub.Equals(oldPubKey) {
			t.Fatalf("new party %d: EDDSAPub changed after reshare", i)
		}
		if err := newSaves[i].ValidateSaveData(); err != nil {
			t.Fatalf("new party %d ValidateSaveData: %v", i, err)
		}
		t.Logf("new party %d: EDDSAPub = (%x, %x)", i,
			newSaves[i].EDDSAPub.X(), newSaves[i].EDDSAPub.Y())
	}

	// --- Sign with new committee ---
	msgHash := sha256.Sum256([]byte("hello reshared eddsa"))
	m := new(big.Int).SetBytes(msgHash[:])

	sigStates := make([]*signing.SigningState, newN)
	sr1 := make([]*tss.Message, newN)
	for i := 0; i < newN; i++ {
		params := tss.NewParameters(tss.Edwards(), newPeerCtx, newPIDs[i], newN, newT)
		st, out, err := signing.SignRound1(params, newSaves[i], m, 0)
		if err != nil {
			t.Fatalf("SignRound1[%d]: %v", i, err)
		}
		sigStates[i] = st
		sr1[i] = out.Messages[0]
	}

	sr2 := make([]*tss.Message, newN)
	for i := 0; i < newN; i++ {
		out, err := signing.SignRound2(sigStates[i], sr1)
		if err != nil {
			t.Fatalf("SignRound2[%d]: %v", i, err)
		}
		sr2[i] = out.Messages[0]
	}

	sr3 := make([]*tss.Message, newN)
	for i := 0; i < newN; i++ {
		out, err := signing.SignRound3(sigStates[i], sr2)
		if err != nil {
			t.Fatalf("SignRound3[%d]: %v", i, err)
		}
		sr3[i] = out.Messages[0]
	}

	out, err := signing.SignFinalize(sigStates[0], sr3)
	if err != nil {
		t.Fatalf("SignFinalize: %v", err)
	}

	pk := edwards.PublicKey{
		Curve: tss.Edwards(),
		X:     newSaves[0].EDDSAPub.X(),
		Y:     newSaves[0].EDDSAPub.Y(),
	}
	r := new(big.Int).SetBytes(out.Signature.R)
	s := new(big.Int).SetBytes(out.Signature.S)
	if !edwards.Verify(&pk, msgHash[:], r, s) {
		t.Fatal("EdDSA signature verification failed after reshare")
	}
	t.Logf("post-reshare signature verified: r=%x s=%x", out.Signature.R[:8], out.Signature.S[:8])
}

// doKeygen runs EdDSA keygen for the test.
func doKeygen(t *testing.T, n, threshold int, pIDs tss.SortedPartyIDs, peerCtx *tss.PeerContext) []keygen.LocalPartySaveData {
	t.Helper()
	states := make([]*keygen.KeygenState, n)
	r1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.Edwards(), peerCtx, pIDs[i], n, threshold)
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
