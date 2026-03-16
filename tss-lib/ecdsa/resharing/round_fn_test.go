// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package resharing

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

// TestRoundFnReshareAndSign does keygen(3) → reshare(3→3) → sign(3)
// using pure round functions throughout.
func TestRoundFnReshareAndSign(t *testing.T) {
	const n = 3
	const threshold = 1 // 2-of-3

	// ---- Keygen ----
	preParamsOld := make([]keygen.LocalPreParams, n)
	for i := 0; i < n; i++ {
		pp, err := keygen.GeneratePreParams(5 * time.Minute)
		if err != nil {
			t.Fatalf("GeneratePreParams[%d]: %v", i, err)
		}
		preParamsOld[i] = *pp
	}
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
	t.Logf("keygen done: ECDSAPub = (%x...)", oldKeys[0].ECDSAPub.X().Bytes()[:4])

	// ---- Reshare: 3 old → 3 new (disjoint) ----
	newPIDs := tss.GenerateTestPartyIDs(n) // fresh random keys, indices 0..n-1
	newCtx := tss.NewPeerContext(newPIDs)

	preParamsNew := make([]keygen.LocalPreParams, n)
	for i := 0; i < n; i++ {
		pp, err := keygen.GeneratePreParams(5 * time.Minute)
		if err != nil {
			t.Fatalf("GeneratePreParams(new)[%d]: %v", i, err)
		}
		preParamsNew[i] = *pp
	}

	// Create states for old + new parties.
	oldStates := make([]*ReshareState, n)
	newStates := make([]*ReshareState, n)

	// Round 1: old committee produces broadcasts
	oldR1Msgs := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewReSharingParameters(tss.S256(), oldCtx, newCtx, oldPIDs[i], n, threshold, n, threshold)
		st, out, err := ReshareRound1(params, oldKeys[i], keygen.LocalPreParams{})
		if err != nil {
			t.Fatalf("ReshareRound1(old)[%d]: %v", i, err)
		}
		oldStates[i] = st
		if len(out.Messages) > 0 {
			oldR1Msgs[i] = out.Messages[0]
		}
	}
	// New committee: Round1 is a no-op, just creates state
	for i := 0; i < n; i++ {
		params := tss.NewReSharingParameters(tss.S256(), oldCtx, newCtx, newPIDs[i], n, threshold, n, threshold)
		params.SetNoProofMod()
		params.SetNoProofFac()
		st, _, err := ReshareRound1(params, keygen.NewLocalPartySaveData(n), preParamsNew[i])
		if err != nil {
			t.Fatalf("ReshareRound1(new)[%d]: %v", i, err)
		}
		newStates[i] = st
	}

	// Round 2: new committee produces DGRound2Message1 (to new) + DGRound2Message2 (ACK to old)
	newR2Msg1s := make([]*tss.Message, n) // broadcast to new
	newR2Msg2s := make([]*tss.Message, n) // ACK to old
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
	// Old committee: Round2 is a no-op (they're not new committee)
	for i := 0; i < n; i++ {
		_, err := ReshareRound2(oldStates[i], oldR1Msgs)
		if err != nil {
			t.Fatalf("ReshareRound2(old)[%d]: %v", i, err)
		}
	}
	// Fill self-messages for new committee
	for i := 0; i < n; i++ {
		if newR2Msg1s[i] == nil {
			newR2Msg1s[i] = newStates[i].temp.dgRound2Message1s[i]
		}
		if newR2Msg2s[i] == nil {
			newR2Msg2s[i] = newStates[i].temp.dgRound2Message2s[i]
		}
	}

	// Round 3: old committee sends shares P2P + decommit broadcast
	oldR3P2P := make([][]*tss.Message, n) // oldR3P2P[newIdx][oldIdx]
	oldR3Bcast := make([]*tss.Message, n) // oldR3Bcast[oldIdx]
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
				// P2P to new party
				for _, to := range pm.To {
					oldR3P2P[to.Index][i] = pm
				}
			case *DGRound3Message2:
				oldR3Bcast[i] = pm
			}
		}
	}
	// New committee: Round3 is a no-op
	for i := 0; i < n; i++ {
		_, err := ReshareRound3(newStates[i], newR2Msg2s)
		if err != nil {
			t.Fatalf("ReshareRound3(new)[%d]: %v", i, err)
		}
	}

	// Round 4: new committee verifies and produces FacProof + ACK
	newR4P2P := make([][]*tss.Message, n) // newR4P2P[newIdx][senderNewIdx]
	newR4Bcast := make([]*tss.Message, n) // newR4Bcast[newIdx]
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
	// Old committee: Round4 is a no-op
	for i := 0; i < n; i++ {
		_, err := ReshareRound4(context.Background(), oldStates[i], newR2Msg1s, nil, nil)
		if err != nil {
			t.Fatalf("ReshareRound4(old)[%d]: %v", i, err)
		}
	}
	// Fill self-messages
	for i := 0; i < n; i++ {
		if newR4Bcast[i] == nil {
			newR4Bcast[i] = newStates[i].temp.dgRound4Message2s[i]
		}
	}

	// Round 5: new committee finalizes
	newKeys := make([]keygen.LocalPartySaveData, n)
	for i := 0; i < n; i++ {
		out, err := ReshareRound5(newStates[i], newR4P2P[i], newR4Bcast)
		if err != nil {
			t.Fatalf("ReshareRound5(new)[%d]: %v", i, err)
		}
		if out.Save == nil {
			t.Fatalf("ReshareRound5(new)[%d]: no Save", i)
		}
		newKeys[i] = *out.Save
	}
	// Old committee: Round5 zeros Xi
	for i := 0; i < n; i++ {
		_, err := ReshareRound5(oldStates[i], nil, newR4Bcast)
		if err != nil {
			t.Fatalf("ReshareRound5(old)[%d]: %v", i, err)
		}
	}

	// Verify new keys have same ECDSAPub
	for i := 1; i < n; i++ {
		if !newKeys[i].ECDSAPub.Equals(newKeys[0].ECDSAPub) {
			t.Fatalf("new party %d has different ECDSAPub", i)
		}
	}
	if !newKeys[0].ECDSAPub.Equals(oldKeys[0].ECDSAPub) {
		t.Fatal("new ECDSAPub != old ECDSAPub")
	}
	t.Log("reshare done: ECDSAPub preserved")

	// ---- Sign with new keys ----
	msgHash := sha256.Sum256([]byte("reshare test"))
	m := new(big.Int).SetBytes(msgHash[:])

	sigStates := make([]*signing.SigningState, n)
	sigR1P2P := make([][]*tss.Message, n)
	sigR1Bcast := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		sigR1P2P[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.S256(), newCtx, newPIDs[i], n, threshold)
		st, out, err := signing.SignRound1(params, newKeys[i], m, nil, 0)
		if err != nil {
			t.Fatalf("SignRound1[%d]: %v", i, err)
		}
		sigStates[i] = st
		for _, msg := range out.Messages {
			pm := msg
			if pm.To == nil {
				sigR1Bcast[i] = pm
			} else {
				for _, to := range pm.To {
					sigR1P2P[to.Index][i] = pm
				}
			}
		}
	}

	// Signing rounds 2-9 + finalize
	sigR2P2P := make([][]*tss.Message, n)
	for i := 0; i < n; i++ {
		sigR2P2P[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := signing.SignRound2(context.Background(), sigStates[i], sigR1P2P[i], sigR1Bcast)
		if err != nil {
			t.Fatalf("SignRound2[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			pm := msg
			for _, to := range pm.To {
				sigR2P2P[to.Index][i] = pm
			}
		}
	}
	sigR3 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := signing.SignRound3(context.Background(), sigStates[i], sigR2P2P[i])
		if err != nil {
			t.Fatalf("SignRound3[%d]: %v", i, err)
		}
		sigR3[i] = out.Messages[0]
	}
	sigR4 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := signing.SignRound4(sigStates[i], sigR3)
		if err != nil {
			t.Fatalf("SignRound4[%d]: %v", i, err)
		}
		sigR4[i] = out.Messages[0]
	}
	sigR5 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := signing.SignRound5(sigStates[i], sigR4)
		if err != nil {
			t.Fatalf("SignRound5[%d]: %v", i, err)
		}
		sigR5[i] = out.Messages[0]
	}
	sigR6 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := signing.SignRound6(sigStates[i])
		if err != nil {
			t.Fatalf("SignRound6[%d]: %v", i, err)
		}
		sigR6[i] = out.Messages[0]
	}
	sigR7 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := signing.SignRound7(sigStates[i], sigR5, sigR6)
		if err != nil {
			t.Fatalf("SignRound7[%d]: %v", i, err)
		}
		sigR7[i] = out.Messages[0]
	}
	sigR8 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := signing.SignRound8(sigStates[i])
		if err != nil {
			t.Fatalf("SignRound8[%d]: %v", i, err)
		}
		sigR8[i] = out.Messages[0]
	}
	sigR9 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := signing.SignRound9(sigStates[i], sigR7, sigR8)
		if err != nil {
			t.Fatalf("SignRound9[%d]: %v", i, err)
		}
		sigR9[i] = out.Messages[0]
	}
	for i := 0; i < n; i++ {
		out, err := signing.SignFinalize(sigStates[i], sigR9)
		if err != nil {
			t.Fatalf("SignFinalize[%d]: %v", i, err)
		}
		pk := ecdsa.PublicKey{
			Curve: tss.S256(),
			X:     newKeys[0].ECDSAPub.X(),
			Y:     newKeys[0].ECDSAPub.Y(),
		}
		r := new(big.Int).SetBytes(out.Signature.R)
		s := new(big.Int).SetBytes(out.Signature.S)
		if !ecdsa.Verify(&pk, msgHash[:], r, s) {
			t.Fatalf("party %d: signature verification failed after reshare", i)
		}
	}
	t.Log("sign after reshare: verified")
}

// TestRoundFnReshareNoProofDLN does reshare with DLN proofs disabled
// (on-chain SNARK mode).
func TestRoundFnReshareNoProofDLN(t *testing.T) {
	const n = 3
	const threshold = 1

	// Keygen (same as main test)
	preParamsOld := make([]keygen.LocalPreParams, n)
	for i := 0; i < n; i++ {
		pp, err := keygen.GeneratePreParams(5 * time.Minute)
		if err != nil {
			t.Fatalf("GeneratePreParams[%d]: %v", i, err)
		}
		preParamsOld[i] = *pp
	}
	oldPIDs := tss.GenerateTestPartyIDs(n)
	oldCtx := tss.NewPeerContext(oldPIDs)
	kgStates := make([]*keygen.KeygenState, n)
	kgR1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.S256(), oldCtx, oldPIDs[i], n, threshold)
		st, out, err := keygen.Round1(context.Background(), params, preParamsOld[i])
		if err != nil {
			t.Fatalf("kg R1[%d]: %v", i, err)
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
		out, _ := keygen.Round2(context.Background(), kgStates[i], kgR1)
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
		out, _ := keygen.Round3(context.Background(), kgStates[i], kgR2P2P[i], kgR2Bcast)
		kgR3[i] = out.Messages[0]
	}
	oldKeys := make([]keygen.LocalPartySaveData, n)
	for i := 0; i < n; i++ {
		out, _ := keygen.Round4(context.Background(), kgStates[i], kgR3)
		oldKeys[i] = *out.Save
	}

	// Reshare with no-proof flags
	newPIDs := tss.GenerateTestPartyIDs(n)
	newCtx := tss.NewPeerContext(newPIDs)
	preParamsNew := make([]keygen.LocalPreParams, n)
	for i := 0; i < n; i++ {
		pp, err := keygen.GeneratePreParams(5 * time.Minute)
		if err != nil {
			t.Fatalf("GeneratePreParams(new)[%d]: %v", i, err)
		}
		preParamsNew[i] = *pp
	}

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

	for i := 0; i < n; i++ {
		out, err := ReshareRound5(newStates[i], newR4P2P[i], newR4Bcast)
		if err != nil {
			t.Fatalf("ReshareRound5(new)[%d]: %v", i, err)
		}
		if out.Save == nil {
			t.Fatalf("ReshareRound5(new)[%d]: no Save", i)
		}
		if !out.Save.ECDSAPub.Equals(oldKeys[0].ECDSAPub) {
			t.Fatal("ECDSAPub changed after reshare")
		}
	}
	for i := 0; i < n; i++ {
		_, _ = ReshareRound5(oldStates[i], nil, newR4Bcast)
	}
	t.Log("reshare with all proofs disabled: passed")
}
