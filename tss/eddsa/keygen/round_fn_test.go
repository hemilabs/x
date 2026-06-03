// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"testing"

	"github.com/hemilabs/x/tss-lib/v3/tss"
)

func TestRoundFnEdDSAKeygenThreeParties(t *testing.T) {
	const n = 3
	const threshold = 1

	pIDs := tss.GenerateTestPartyIDs(n)
	peerCtx := tss.NewPeerContext(pIDs)

	// --- Round 1 ---
	states := make([]*KeygenState, n)
	r1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.Edwards(), peerCtx, pIDs[i], n, threshold)
		st, out, err := Round1(params)
		if err != nil {
			t.Fatalf("Round1[%d]: %v", i, err)
		}
		states[i] = st
		r1[i] = out.Messages[0]
		if out.Poly == nil {
			t.Fatal("Round1 should return Poly")
		}
	}

	// --- Round 2 ---
	r2p2p := make([][]*tss.Message, n)
	r2bcast := make([]*tss.Message, n)
	for i := range r2p2p {
		r2p2p[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := Round2(states[i], r1)
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

	// --- Round 3 ---
	for i := 0; i < n; i++ {
		out, err := Round3(states[i], r2p2p[i], r2bcast)
		if err != nil {
			t.Fatalf("Round3[%d]: %v", i, err)
		}
		if out.Save == nil {
			t.Fatal("Round3 should return Save")
		}
		if err := out.Save.ValidateSaveData(); err != nil {
			t.Fatalf("ValidateSaveData[%d]: %v", i, err)
		}
		t.Logf("party %d: EDDSAPub = (%x, %x)", i,
			out.Save.EDDSAPub.X(), out.Save.EDDSAPub.Y())
	}
}
