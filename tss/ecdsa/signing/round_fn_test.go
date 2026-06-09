// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package signing

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss/v3/ecdsa/keygen"
	"github.com/hemilabs/x/tss/v3/testutil"
	"github.com/hemilabs/x/tss/v3/tss"
)

// TestRoundFnSignThreeParties runs keygen + signing using pure round
// functions for both.  Verifies the signature with ecdsa.Verify.
func TestRoundFnSignThreeParties(t *testing.T) {
	const n = 3
	const threshold = 1 // 2-of-3

	// -- Keygen first --
	preParams := testutil.LoadPreParams(t, n)
	pIDs := tss.GenerateTestPartyIDs(n)
	peerCtx := tss.NewPeerContext(pIDs)

	kgStates := make([]*keygen.KeygenState, n)
	kgR1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.S256(), peerCtx, pIDs[i], n, threshold)
		st, out, err := keygen.Round1(context.Background(), params, preParams[i])
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
	}
	for i := 0; i < n; i++ {
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

	saves := make([]keygen.LocalPartySaveData, n)
	for i := 0; i < n; i++ {
		out, err := keygen.Round4(context.Background(), kgStates[i], kgR3)
		if err != nil {
			t.Fatalf("keygen Round4[%d]: %v", i, err)
		}
		saves[i] = *out.Save
	}
	t.Logf("keygen done: ECDSAPub = (%x, %x)", saves[0].ECDSAPub.X(), saves[0].ECDSAPub.Y())

	// -- Sign --
	msgHash := sha256.Sum256([]byte("test message"))
	m := new(big.Int).SetBytes(msgHash[:])

	sigStates := make([]*SigningState, n)
	sigR1P2P := make([][]*tss.Message, n)
	sigR1Bcast := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		sigR1P2P[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.S256(), peerCtx, pIDs[i], n, threshold)
		st, out, err := SignRound1(params, saves[i], m, nil, 0)
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

	// Round 2
	sigR2P2P := make([][]*tss.Message, n)
	for i := 0; i < n; i++ {
		sigR2P2P[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := SignRound2(context.Background(), sigStates[i], sigR1P2P[i], sigR1Bcast)
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

	// Round 3
	sigR3 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := SignRound3(context.Background(), sigStates[i], sigR2P2P[i])
		if err != nil {
			t.Fatalf("SignRound3[%d]: %v", i, err)
		}
		sigR3[i] = out.Messages[0]
	}

	// Round 4
	sigR4 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := SignRound4(sigStates[i], sigR3)
		if err != nil {
			t.Fatalf("SignRound4[%d]: %v", i, err)
		}
		sigR4[i] = out.Messages[0]
	}

	// Round 5
	sigR5 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := SignRound5(sigStates[i], sigR4)
		if err != nil {
			t.Fatalf("SignRound5[%d]: %v", i, err)
		}
		sigR5[i] = out.Messages[0]
	}

	// Round 6
	sigR6 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := SignRound6(sigStates[i])
		if err != nil {
			t.Fatalf("SignRound6[%d]: %v", i, err)
		}
		sigR6[i] = out.Messages[0]
	}

	// Round 7
	sigR7 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := SignRound7(sigStates[i], sigR5, sigR6)
		if err != nil {
			t.Fatalf("SignRound7[%d]: %v", i, err)
		}
		sigR7[i] = out.Messages[0]
	}

	// Round 8
	sigR8 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := SignRound8(sigStates[i])
		if err != nil {
			t.Fatalf("SignRound8[%d]: %v", i, err)
		}
		sigR8[i] = out.Messages[0]
	}

	// Round 9
	sigR9 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := SignRound9(sigStates[i], sigR7, sigR8)
		if err != nil {
			t.Fatalf("SignRound9[%d]: %v", i, err)
		}
		sigR9[i] = out.Messages[0]
	}

	// Finalize
	for i := 0; i < n; i++ {
		out, err := SignFinalize(sigStates[i], sigR9)
		if err != nil {
			t.Fatalf("SignFinalize[%d]: %v", i, err)
		}
		if out.Signature == nil {
			t.Fatalf("SignFinalize[%d]: no signature", i)
		}
		// Verify independently
		pk := ecdsa.PublicKey{
			Curve: tss.S256(),
			X:     saves[0].ECDSAPub.X(),
			Y:     saves[0].ECDSAPub.Y(),
		}
		r := new(big.Int).SetBytes(out.Signature.R)
		s := new(big.Int).SetBytes(out.Signature.S)
		if !ecdsa.Verify(&pk, msgHash[:], r, s) {
			t.Fatalf("party %d: signature verification failed", i)
		}
		t.Logf("party %d: signature verified (r=%x, s=%x)", i,
			out.Signature.R, out.Signature.S)
	}
}

// TestRoundFnSignSubset does keygen(5) then signs with threshold+1=2
// parties — exercising the auto-subset path where len(key.Ks) >
// signing committee size.
func TestRoundFnSignSubset(t *testing.T) {
	const nKeygen = 5
	const nSign = 2
	const threshold = 1 // 2-of-5

	// -- Keygen with 5 parties --
	preParams := testutil.LoadPreParams(t, nKeygen)
	pIDs := tss.GenerateTestPartyIDs(nKeygen)
	peerCtx := tss.NewPeerContext(pIDs)

	kgStates := make([]*keygen.KeygenState, nKeygen)
	kgR1 := make([]*tss.Message, nKeygen)
	for i := 0; i < nKeygen; i++ {
		params := tss.NewParameters(tss.S256(), peerCtx, pIDs[i], nKeygen, threshold)
		st, out, err := keygen.Round1(context.Background(), params, preParams[i])
		if err != nil {
			t.Fatalf("keygen Round1[%d]: %v", i, err)
		}
		kgStates[i] = st
		kgR1[i] = out.Messages[0]
	}

	kgR2P2P := make([][]*tss.Message, nKeygen)
	kgR2Bcast := make([]*tss.Message, nKeygen)
	for i := 0; i < nKeygen; i++ {
		kgR2P2P[i] = make([]*tss.Message, nKeygen)
	}
	for i := 0; i < nKeygen; i++ {
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

	kgR3 := make([]*tss.Message, nKeygen)
	for i := 0; i < nKeygen; i++ {
		out, err := keygen.Round3(context.Background(), kgStates[i], kgR2P2P[i], kgR2Bcast)
		if err != nil {
			t.Fatalf("keygen Round3[%d]: %v", i, err)
		}
		kgR3[i] = out.Messages[0]
	}

	allKeys := make([]keygen.LocalPartySaveData, nKeygen)
	for i := 0; i < nKeygen; i++ {
		out, err := keygen.Round4(context.Background(), kgStates[i], kgR3)
		if err != nil {
			t.Fatalf("keygen Round4[%d]: %v", i, err)
		}
		allKeys[i] = *out.Save
	}
	t.Logf("keygen(5) done: ECDSAPub = (%x...)", allKeys[0].ECDSAPub.X().Bytes()[:4])

	// -- Sign with first 2 parties (subset of 5) --
	signPIDs := tss.SortPartyIDs(
		tss.UnSortedPartyIDs{pIDs[0], pIDs[1]})
	signCtx := tss.NewPeerContext(signPIDs)

	msgHash := sha256.Sum256([]byte("subset signing test"))
	m := new(big.Int).SetBytes(msgHash[:])

	sigStates := make([]*SigningState, nSign)
	sigR1P2P := make([][]*tss.Message, nSign)
	sigR1Bcast := make([]*tss.Message, nSign)
	for i := 0; i < nSign; i++ {
		sigR1P2P[i] = make([]*tss.Message, nSign)
	}

	// Find original key index for each signer
	signerOrigIdx := make([]int, nSign)
	for i, spid := range signPIDs {
		for j, kpid := range pIDs {
			if spid.KeyInt().Cmp(kpid.KeyInt()) == 0 {
				signerOrigIdx[i] = j
				break
			}
		}
	}

	for i := 0; i < nSign; i++ {
		params := tss.NewParameters(tss.S256(), signCtx, signPIDs[i], nSign, threshold)
		// Pass FULL key data — SignRound1 should auto-subset
		st, out, err := SignRound1(params, allKeys[signerOrigIdx[i]], m, nil, 0)
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

	// Rounds 2-9 + Finalize
	sigR2P2P := make([][]*tss.Message, nSign)
	for i := 0; i < nSign; i++ {
		sigR2P2P[i] = make([]*tss.Message, nSign)
	}
	for i := 0; i < nSign; i++ {
		out, err := SignRound2(context.Background(), sigStates[i], sigR1P2P[i], sigR1Bcast)
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

	sigR3 := make([]*tss.Message, nSign)
	for i := 0; i < nSign; i++ {
		out, err := SignRound3(context.Background(), sigStates[i], sigR2P2P[i])
		if err != nil {
			t.Fatalf("SignRound3[%d]: %v", i, err)
		}
		sigR3[i] = out.Messages[0]
	}

	sigR4 := make([]*tss.Message, nSign)
	for i := 0; i < nSign; i++ {
		out, err := SignRound4(sigStates[i], sigR3)
		if err != nil {
			t.Fatalf("SignRound4[%d]: %v", i, err)
		}
		sigR4[i] = out.Messages[0]
	}

	sigR5 := make([]*tss.Message, nSign)
	for i := 0; i < nSign; i++ {
		out, err := SignRound5(sigStates[i], sigR4)
		if err != nil {
			t.Fatalf("SignRound5[%d]: %v", i, err)
		}
		sigR5[i] = out.Messages[0]
	}

	sigR6 := make([]*tss.Message, nSign)
	for i := 0; i < nSign; i++ {
		out, err := SignRound6(sigStates[i])
		if err != nil {
			t.Fatalf("SignRound6[%d]: %v", i, err)
		}
		sigR6[i] = out.Messages[0]
	}

	sigR7 := make([]*tss.Message, nSign)
	for i := 0; i < nSign; i++ {
		out, err := SignRound7(sigStates[i], sigR5, sigR6)
		if err != nil {
			t.Fatalf("SignRound7[%d]: %v", i, err)
		}
		sigR7[i] = out.Messages[0]
	}

	sigR8 := make([]*tss.Message, nSign)
	for i := 0; i < nSign; i++ {
		out, err := SignRound8(sigStates[i])
		if err != nil {
			t.Fatalf("SignRound8[%d]: %v", i, err)
		}
		sigR8[i] = out.Messages[0]
	}

	sigR9 := make([]*tss.Message, nSign)
	for i := 0; i < nSign; i++ {
		out, err := SignRound9(sigStates[i], sigR7, sigR8)
		if err != nil {
			t.Fatalf("SignRound9[%d]: %v", i, err)
		}
		sigR9[i] = out.Messages[0]
	}

	for i := 0; i < nSign; i++ {
		out, err := SignFinalize(sigStates[i], sigR9)
		if err != nil {
			t.Fatalf("SignFinalize[%d]: %v", i, err)
		}
		pk := ecdsa.PublicKey{
			Curve: tss.S256(),
			X:     allKeys[0].ECDSAPub.X(),
			Y:     allKeys[0].ECDSAPub.Y(),
		}
		r := new(big.Int).SetBytes(out.Signature.R)
		s := new(big.Int).SetBytes(out.Signature.S)
		if !ecdsa.Verify(&pk, msgHash[:], r, s) {
			t.Fatalf("party %d: subset signature verification failed", i)
		}
	}
	t.Log("subset sign (2-of-5) verified")
}

// TestRoundFnSignLeadingZeroMsg exercises signing where the hash
// has leading zero bytes (tests fullBytesLen padding path).
func TestRoundFnSignLeadingZeroMsg(t *testing.T) {
	const n = 3
	const threshold = 1

	preParams := testutil.LoadPreParams(t, n)
	pIDs := tss.GenerateTestPartyIDs(n)
	peerCtx := tss.NewPeerContext(pIDs)

	// Keygen
	kgStates := make([]*keygen.KeygenState, n)
	kgR1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.S256(), peerCtx, pIDs[i], n, threshold)
		st, out, err := keygen.Round1(context.Background(), params, preParams[i])
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
	saves := make([]keygen.LocalPartySaveData, n)
	for i := 0; i < n; i++ {
		out, _ := keygen.Round4(context.Background(), kgStates[i], kgR3)
		saves[i] = *out.Save
	}

	// Sign with leading-zero message (first byte = 0x00)
	msgData := []byte{
		0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06,
		0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e,
		0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16,
		0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e,
	}
	m := new(big.Int).SetBytes(msgData)

	sigStates := make([]*SigningState, n)
	sigR1P2P := make([][]*tss.Message, n)
	sigR1Bcast := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		sigR1P2P[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.S256(), peerCtx, pIDs[i], n, threshold)
		st, out, err := SignRound1(params, saves[i], m, nil, len(msgData))
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

	// Run rounds 2-9 + finalize (same boilerplate)
	r2P2P := make([][]*tss.Message, n)
	for i := 0; i < n; i++ {
		r2P2P[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := SignRound2(context.Background(), sigStates[i], sigR1P2P[i], sigR1Bcast)
		if err != nil {
			t.Fatalf("R2[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			pm := msg
			for _, to := range pm.To {
				r2P2P[to.Index][i] = pm
			}
		}
	}
	r3 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := SignRound3(context.Background(), sigStates[i], r2P2P[i])
		if err != nil {
			t.Fatalf("R3[%d]: %v", i, err)
		}
		r3[i] = out.Messages[0]
	}
	r4 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := SignRound4(sigStates[i], r3)
		if err != nil {
			t.Fatalf("R4[%d]: %v", i, err)
		}
		r4[i] = out.Messages[0]
	}
	r5 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := SignRound5(sigStates[i], r4)
		if err != nil {
			t.Fatalf("R5[%d]: %v", i, err)
		}
		r5[i] = out.Messages[0]
	}
	r6 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := SignRound6(sigStates[i])
		if err != nil {
			t.Fatalf("R6[%d]: %v", i, err)
		}
		r6[i] = out.Messages[0]
	}
	r7 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := SignRound7(sigStates[i], r5, r6)
		if err != nil {
			t.Fatalf("R7[%d]: %v", i, err)
		}
		r7[i] = out.Messages[0]
	}
	r8 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := SignRound8(sigStates[i])
		if err != nil {
			t.Fatalf("R8[%d]: %v", i, err)
		}
		r8[i] = out.Messages[0]
	}
	r9 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := SignRound9(sigStates[i], r7, r8)
		if err != nil {
			t.Fatalf("R9[%d]: %v", i, err)
		}
		r9[i] = out.Messages[0]
	}
	for i := 0; i < n; i++ {
		out, err := SignFinalize(sigStates[i], r9)
		if err != nil {
			t.Fatalf("Finalize[%d]: %v", i, err)
		}
		// Verify M preserves leading zeros
		if len(out.Signature.M) != len(msgData) {
			t.Fatalf("party %d: M length %d != %d", i, len(out.Signature.M), len(msgData))
		}
		if out.Signature.M[0] != 0x00 || out.Signature.M[1] != 0x00 {
			t.Fatalf("party %d: leading zeros lost", i)
		}
	}
	t.Log("sign with leading zero msg: passed")
}
