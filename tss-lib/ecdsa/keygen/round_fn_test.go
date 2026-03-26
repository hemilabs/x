// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"context"
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// TestRoundFnKeygenThreeParties runs a 3-party keygen using the pure
// round functions (no channels, no goroutines).  Verifies that the
// produced key shares are valid: Xi*G == BigXj[ownIndex] (Feldman
// invariant) and all parties agree on the same ECDSAPub.
func TestRoundFnKeygenThreeParties(t *testing.T) {
	const n = 3
	const threshold = 1 // 2-of-3

	preParams := loadTestPreParams(t, n)

	// Generate sorted party IDs.
	pIDs := tss.GenerateTestPartyIDs(n)
	peerCtx := tss.NewPeerContext(pIDs)

	// -- Round 1 --
	states := make([]*KeygenState, n)
	r1Outputs := make([]*RoundOutput, n)
	r1Msgs := make([][]*tss.Message, n) // r1Msgs[i] = party i's broadcasts
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.S256(), peerCtx, pIDs[i], n, threshold)
		st, out, err := Round1(context.Background(), params, preParams[i])
		if err != nil {
			t.Fatalf("Round1[%d]: %v", i, err)
		}
		states[i] = st
		r1Outputs[i] = out
		r1Msgs[i] = make([]*tss.Message, 1) // Round1 produces 1 broadcast
		r1Msgs[i][0] = out.Messages[0]
	}
	if r1Outputs[0].Poly == nil {
		t.Fatal("Round1 should return Poly for SNARK witness")
	}

	// Collect round 1 broadcasts: allR1[j] = party j's broadcast
	allR1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		allR1[i] = r1Outputs[i].Messages[0]
	}

	// -- Round 2 --
	r2Outputs := make([]*RoundOutput, n)
	for i := 0; i < n; i++ {
		out, err := Round2(context.Background(), states[i], allR1)
		if err != nil {
			t.Fatalf("Round2[%d]: %v", i, err)
		}
		r2Outputs[i] = out
	}

	// Collect round 2 messages per party.
	// Round2 produces: (n-1) P2P messages + 1 broadcast.
	// P2P messages have GetTo() != nil; broadcast has GetTo() == nil.
	allR2P2P := make([][]*tss.Message, n) // allR2P2P[receiver][sender]
	allR2Bcast := make([]*tss.Message, n) // allR2Bcast[sender]
	for i := 0; i < n; i++ {
		allR2P2P[i] = make([]*tss.Message, n)
	}
	for sender := 0; sender < n; sender++ {
		for _, msg := range r2Outputs[sender].Messages {
			pm := msg
			if pm.To == nil {
				// broadcast
				allR2Bcast[sender] = pm
			} else {
				// P2P — route to recipient
				for _, to := range pm.To {
					allR2P2P[to.Index][sender] = pm
				}
			}
		}
		// Own P2P message to self is in state.temp
		allR2P2P[sender][sender] = states[sender].temp.kgRound2Message1s[sender]
	}
	// Fill own broadcast
	for i := 0; i < n; i++ {
		allR2Bcast[i] = states[i].temp.kgRound2Message2s[i]
		if allR2Bcast[i] == nil {
			allR2Bcast[i] = r2Outputs[i].Messages[len(r2Outputs[i].Messages)-1]
		}
	}

	// -- Round 3 --
	r3Outputs := make([]*RoundOutput, n)
	for i := 0; i < n; i++ {
		out, err := Round3(context.Background(), states[i], allR2P2P[i], allR2Bcast)
		if err != nil {
			t.Fatalf("Round3[%d]: %v", i, err)
		}
		r3Outputs[i] = out
	}

	// Collect round 3 broadcasts
	allR3 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		allR3[i] = r3Outputs[i].Messages[0]
	}

	// -- Round 4 --
	saves := make([]*LocalPartySaveData, n)
	for i := 0; i < n; i++ {
		out, err := Round4(context.Background(), states[i], allR3)
		if err != nil {
			t.Fatalf("Round4[%d]: %v", i, err)
		}
		if out.Save == nil {
			t.Fatalf("Round4[%d]: Save is nil", i)
		}
		saves[i] = out.Save
	}

	// -- Verify --
	// 1. All parties agree on ECDSAPub
	for i := 1; i < n; i++ {
		if saves[i].ECDSAPub.X().Cmp(saves[0].ECDSAPub.X()) != 0 ||
			saves[i].ECDSAPub.Y().Cmp(saves[0].ECDSAPub.Y()) != 0 {
			t.Fatalf("party %d has different ECDSAPub", i)
		}
	}

	// 2. Feldman invariant: Xi * G == BigXj[ownIndex]
	curve := tss.S256()
	for i := 0; i < n; i++ {
		xi := saves[i].Xi
		gx, gy := curve.ScalarBaseMult(xi.Bytes())
		bxj := saves[i].BigXj[i]
		if gx.Cmp(bxj.X()) != 0 || gy.Cmp(bxj.Y()) != 0 {
			t.Fatalf("party %d: Xi*G != BigXj[%d]", i, i)
		}
	}

	// 3. Lagrange interpolation of Xi recovers the private key.
	// sk = sum_i(Xi * lambda_i) where lambda_i = prod_{j!=i}(kj/(kj-ki))
	q := curve.Params().N
	sk := new(big.Int)
	for i := 0; i < n; i++ {
		li := new(big.Int).SetInt64(1)
		ki := saves[i].ShareID
		for j := 0; j < n; j++ {
			if j == i {
				continue
			}
			kj := saves[j].ShareID
			num := new(big.Int).Set(kj)
			den := new(big.Int).Sub(kj, ki)
			den.Mod(den, q)
			den.ModInverse(den, q)
			li.Mul(li, num)
			li.Mul(li, den)
			li.Mod(li, q)
		}
		term := new(big.Int).Mul(saves[i].Xi, li)
		sk.Add(sk, term)
	}
	sk.Mod(sk, q)
	pubX, pubY := curve.ScalarBaseMult(sk.Bytes())
	if pubX.Cmp(saves[0].ECDSAPub.X()) != 0 || pubY.Cmp(saves[0].ECDSAPub.Y()) != 0 {
		t.Fatal("Lagrange interpolation of Xi does not match ECDSAPub")
	}

	t.Logf("keygen succeeded: ECDSAPub = (%x, %x)", saves[0].ECDSAPub.X(), saves[0].ECDSAPub.Y())
}

// TestRoundFnKeygenNoProofFlags exercises keygen with all proof
// generation/verification disabled (SNARK mode).
func TestRoundFnKeygenNoProofFlags(t *testing.T) {
	const n = 3
	const threshold = 1

	preParams := loadTestPreParams(t, n)
	pIDs := tss.GenerateTestPartyIDs(n)
	peerCtx := tss.NewPeerContext(pIDs)

	states := make([]*KeygenState, n)
	r1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.S256(), peerCtx, pIDs[i], n, threshold)
		params.SetNoProofMod()
		params.SetNoProofFac()
		params.SetNoProofDLN()
		st, out, err := Round1(context.Background(), params, preParams[i])
		if err != nil {
			t.Fatalf("Round1[%d]: %v", i, err)
		}
		states[i] = st
		r1[i] = out.Messages[0]
	}

	r2P2P := make([][]*tss.Message, n)
	r2Bcast := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		r2P2P[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := Round2(context.Background(), states[i], r1)
		if err != nil {
			t.Fatalf("Round2[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			pm := msg
			if pm.To == nil {
				r2Bcast[i] = pm
			} else {
				for _, to := range pm.To {
					r2P2P[to.Index][i] = pm
				}
			}
		}
		r2P2P[i][i] = states[i].ExportR2P2PSelf()
		if r2Bcast[i] == nil {
			r2Bcast[i] = states[i].ExportR2BcastSelf()
		}
	}

	r3 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := Round3(context.Background(), states[i], r2P2P[i], r2Bcast)
		if err != nil {
			t.Fatalf("Round3[%d]: %v", i, err)
		}
		r3[i] = out.Messages[0]
	}

	for i := 0; i < n; i++ {
		out, err := Round4(context.Background(), states[i], r3)
		if err != nil {
			t.Fatalf("Round4[%d]: %v", i, err)
		}
		if out.Save == nil {
			t.Fatalf("Round4[%d]: Save is nil", i)
		}
	}
	t.Log("keygen with all proofs disabled: passed")
}
