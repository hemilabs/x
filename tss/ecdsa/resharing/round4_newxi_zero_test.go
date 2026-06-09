// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package resharing

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/hemilabs/x/tss/v3/common"
	"github.com/hemilabs/x/tss/v3/crypto"
	"github.com/hemilabs/x/tss/v3/crypto/commitments"
	"github.com/hemilabs/x/tss/v3/tss"
)

// TestReshareRound4RejectsNewXiZero exercises the "newXi is zero" check at
// round_fn.go lines 506-508. This condition occurs when the sum of all old
// parties' VSS shares (evaluated at the new party's key) is zero mod q,
// meaning the new party would receive a degenerate (zero) secret share.
//
// Triggering this through the public API is non-trivial because each
// individual share must pass Feldman VSS verification before contributing to
// the sum. The test constructs a scenario where:
//
//  1. Old parties 0 and 1 produce honest shares for new party 0.
//  2. Old party 2's share is replaced with a value that makes the total sum
//     exactly zero mod q: share2' = q - (share0 + share1) mod q.
//  3. Old party 2's VSS commitment polynomial is reconstructed from scratch
//     to be consistent with the forged share, so Feldman VSS passes.
//  4. The R1 commitment hash and R3 decommitment are updated to match.
//
// This is a COLLUDING-ADVERSARY scenario: old party 2 is fully malicious and
// crafts its polynomial to produce a zero-sum share for the target new party.
func TestReshareRound4RejectsNewXiZero(t *testing.T) {
	fix := setupThroughRound3(t)

	ec := tss.S256()
	q := ec.Params().N
	modQ := common.ModInt(q)

	const (
		newVictim  = 0 // new party index that will receive zero xi
		corruptOld = 2 // old party index we corrupt
	)

	// Step 1: Extract the honest shares from all old parties to new party 0.
	shares := make([]*big.Int, reshareN)
	for j := 0; j < reshareN; j++ {
		r3p2p := fix.OldR3P2P[newVictim][j]
		shares[j] = r3p2p.Content.(*DGRound3Message1).Share
	}

	// Step 2: Compute the current sum and the forged share for old party 2.
	partialSum := modQ.Add(shares[0], shares[1])
	// desiredShare2 = q - partialSum, so that share0 + share1 + desiredShare2 = 0 mod q.
	desiredShare2 := new(big.Int).Sub(q, partialSum)
	desiredShare2.Mod(desiredShare2, q)
	if desiredShare2.Sign() == 0 {
		// This would mean share0 + share1 = 0 mod q, which is astronomically
		// unlikely (~2^{-256}). If it ever happens, the individual VSS Verify
		// would reject a zero share anyway.
		t.Fatal("partialSum == 0 mod q; astronomically unlikely, test assumptions broken")
	}

	// Step 3: Construct a new degree-1 polynomial for old party 2 that evaluates
	// to desiredShare2 at the new party's key.
	//
	// Polynomial: f(x) = a0 + a1*x, threshold = 1.
	// We choose a1 randomly, then a0 = desiredShare2 - a1*P mod q.
	P := fix.NewPIDs[newVictim].KeyInt() // evaluation point
	a1 := common.GetRandomPositiveInt(fix.OldStates[corruptOld].params.Rand(), q)
	a1P := modQ.Mul(a1, P)
	a0 := modQ.Sub(desiredShare2, a1P) // a0 = desiredShare2 - a1*P mod q

	// Verify: a0 + a1*P = desiredShare2 mod q (sanity check).
	check := modQ.Add(a0, a1P)
	if check.Cmp(desiredShare2) != 0 {
		t.Fatalf("polynomial construction failed: a0+a1*P = %s, want %s", check, desiredShare2)
	}

	// a0 must be non-zero for ScalarBaseMult to produce a valid curve point.
	// a0 == 0 would mean desiredShare2 = a1*P, which is possible but extremely unlikely
	// for random a1. If it happens, just pick a different a1.
	if a0.Sign() == 0 {
		t.Fatal("a0 == 0; astronomically unlikely")
	}

	// Step 4: Compute the new Feldman VSS commitments V[0] = a0*G, V[1] = a1*G.
	V0 := crypto.ScalarBaseMult(ec, a0)
	V1 := crypto.ScalarBaseMult(ec, a1)

	// Step 5: Build the updated R3 broadcast (decommitment) for old party 2.
	// The decommitment D = [randomness, V0.X, V0.Y, V1.X, V1.Y].
	// Extract the original randomness from the existing decommitment.
	origR3Bcast := fix.OldR3Bcast[corruptOld].Content.(*DGRound3Message2)
	origD := origR3Bcast.VDeCommitment
	// The full D stored in the commitment includes [r, secrets...].
	// DeCommit() strips the first element (randomness) and returns secrets.
	// But VDeCommitment in the message IS the full D (including randomness).
	// Looking at the code: in Round1, temp.VD = vCmt.D (which is [r, flatVs...]).
	// In Round3, NewDGRound3Message2 passes temp.VD as VDeCommitment.
	// In Round4, the verification uses vDj directly from the message, and
	// cmtDeCmt.DeCommit() strips the randomness element.
	// So origD[0] is the randomness, and origD[1:] are the flattened points.
	randomness := origD[0]

	// Build the new decommitment.
	newFlatVs := []*big.Int{V0.X(), V0.Y(), V1.X(), V1.Y()}
	newCmt := commitments.NewHashCommitmentWithRandomness(randomness, newFlatVs...)

	// Step 6: Update the R1 message's VCommitment for old party 2 in the
	// new party 0's stored messages.
	r1stored := fix.NewStates[newVictim].temp.dgRound1Messages[corruptOld]
	r1content := r1stored.Content.(*DGRound1Message)
	r1content.VCommitment = newCmt.C

	// Step 7: Update the R3 broadcast (decommitment) for old party 2.
	r3Bcast := copyR3BcastSlice(fix.OldR3Bcast)
	r3Bcast[corruptOld] = &tss.Message{
		From:        fix.OldR3Bcast[corruptOld].From,
		To:          fix.OldR3Bcast[corruptOld].To,
		IsBroadcast: true,
		Content:     &DGRound3Message2{VDeCommitment: newCmt.D},
	}

	// Step 8: Update the R3 P2P share for old party 2 → new party 0.
	r3P2P := copyR3P2PSlice(fix.OldR3P2P[newVictim])
	origP2P := r3P2P[corruptOld].Content.(*DGRound3Message1)
	r3P2P[corruptOld] = &tss.Message{
		From: fix.OldR3P2P[newVictim][corruptOld].From,
		To:   fix.OldR3P2P[newVictim][corruptOld].To,
		Content: &DGRound3Message1{
			Share:      desiredShare2,
			ReceiverID: origP2P.ReceiverID,
		},
	}

	// Step 9: Call ReshareRound4 and expect "newXi is zero" error.
	_, err := ReshareRound4(
		context.Background(),
		fix.NewStates[newVictim],
		fix.NewR2Msg1s,
		r3P2P,
		r3Bcast,
	)
	if err == nil {
		t.Fatal("expected error from newXi == 0, got nil")
	}
	if !strings.Contains(err.Error(), "newXi is zero") {
		t.Fatalf("expected 'newXi is zero' error, got: %v", err)
	}
	t.Logf("correctly rejected zero newXi: %v", err)
}
