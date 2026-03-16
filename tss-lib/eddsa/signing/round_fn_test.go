// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package signing

import (
	"crypto/sha256"
	"math/big"
	"testing"

	"github.com/decred/dcrd/dcrec/edwards/v2"

	"github.com/hemilabs/x/tss-lib/v3/eddsa/keygen"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

func TestRoundFnEdDSASignThreeParties(t *testing.T) {
	const n = 3
	const threshold = 1

	pIDs := tss.GenerateTestPartyIDs(n)
	peerCtx := tss.NewPeerContext(pIDs)

	// --- Keygen ---
	saves := doKeygen(t, n, threshold, pIDs, peerCtx)

	// --- Sign ---
	msgHash := sha256.Sum256([]byte("hello eddsa v3"))
	m := new(big.Int).SetBytes(msgHash[:])

	// -- SignRound1 --
	sigStates := make([]*SigningState, n)
	r1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.Edwards(), peerCtx, pIDs[i], n, threshold)
		st, out, err := SignRound1(params, saves[i], m, 0)
		if err != nil {
			t.Fatalf("SignRound1[%d]: %v", i, err)
		}
		sigStates[i] = st
		r1[i] = out.Messages[0]
	}

	// -- SignRound2 --
	r2 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := SignRound2(sigStates[i], r1)
		if err != nil {
			t.Fatalf("SignRound2[%d]: %v", i, err)
		}
		r2[i] = out.Messages[0]
	}

	// -- SignRound3 --
	r3 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := SignRound3(sigStates[i], r2)
		if err != nil {
			t.Fatalf("SignRound3[%d]: %v", i, err)
		}
		r3[i] = out.Messages[0]
	}

	// -- Finalize --
	for i := 0; i < n; i++ {
		out, err := SignFinalize(sigStates[i], r3)
		if err != nil {
			t.Fatalf("SignFinalize[%d]: %v", i, err)
		}

		pk := edwards.PublicKey{
			Curve: tss.Edwards(),
			X:     saves[0].EDDSAPub.X(),
			Y:     saves[0].EDDSAPub.Y(),
		}
		r := new(big.Int).SetBytes(out.Signature.R)
		s := new(big.Int).SetBytes(out.Signature.S)
		if !edwards.Verify(&pk, msgHash[:], r, s) {
			t.Fatalf("party %d: EdDSA signature verification failed", i)
		}
		t.Logf("party %d: sig verified (r=%x, s=%x)", i,
			out.Signature.R[:8], out.Signature.S[:8])
	}
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
