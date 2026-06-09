// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package signing

// Negative tests for SignRound4 and SignRound5.
//
// SignRound4 (round_fn.go:402-433) has 2 error paths:
//
//  1. Line 419-420: "theta is zero" — aggregate theta is zero mod N,
//     so ModInverse returns nil.
//     TESTABLE: corrupt Round3 theta shares to sum to zero.
//
//  2. Line 425-426: NewZKProof fails — requires gamma or pointGamma to
//     be nil/invalid.
//     INFEASIBLE: gamma is always a random positive int and pointGamma
//     = gamma*G is always a valid curve point from honest execution.
//
// SignRound5 (round_fn.go:436-523) has 10 error paths:
//
//  1. Line 452-453: "commitment verify failed" — decommitment check fails.
//     TESTABLE: corrupt the commitment in Round1Message2.
//
//  2. Line 455-457: NewECPoint fails — decommitted values not on curve.
//     TESTABLE: replace commitment + decommitment with off-curve coords.
//
//  3. Line 460-461: "bigGamma proof missing" — ZKProof is nil.
//     TESTABLE: set ZKProof to nil in Round4 message.
//
//  4. Line 463-464: "bigGamma proof verify failed" — ZK proof does not verify.
//     TESTABLE: corrupt the ZKProof.T scalar.
//
//  5. Line 467-469: R.Add error.
//     INFEASIBLE via message corruption: the point just passed NewECPoint.
//
//  6. Line 473-474: "sum of gamma points is identity".
//     INFEASIBLE: gamma_i are random non-zero scalars.
//
//  7. Line 477-478: "r is identity after theta-inverse".
//     INFEASIBLE: R is non-identity and thetaInverse is non-zero.
//
//  8. Line 484-485: "r component is zero" — R.X mod N == 0.
//     INFEASIBLE: probability ~2^-256.
//
//  9. Line 489-490: "si is zero".
//     INFEASIBLE: requires m*k + rx*sigma = 0 mod N with random k, sigma.
//
//  10. Line 505-506: "round 5 compute bigVi" — R*si + li*G Add fails.
//      INFEASIBLE: both points are valid curve points.

import (
	"math/big"
	"strings"
	"testing"

	cmt "github.com/hemilabs/x/tss/v3/crypto/commitments"
	"github.com/hemilabs/x/tss/v3/tss"
)

// ===========================================================================
// SignRound4 negative tests
// ===========================================================================

func TestSignRound4RejectsZeroTheta(t *testing.T) {
	f := setupThroughRound3(t)

	// Strategy: corrupt Round3 messages so that the sum of all thetas
	// (party 0's own theta + party 1's theta + party 2's theta) equals
	// zero mod N.
	//
	// Party 0's theta is stored in f.States[0].temp.theta.
	// Round3 broadcasts from parties 1 and 2 each carry a Theta field.
	// Set theta_1 = -theta_0 mod N, theta_2 = N (which is 0 mod N).
	// Then: theta_0 + (-theta_0) + 0 = 0 mod N.
	N := tss.S256().Params().N
	theta0 := f.States[0].temp.theta

	r3 := CloneBcastSlice(f.R3Bcast, CloneR3BcastMsg)

	// party 1's theta = N - theta_0 (i.e., -theta_0 mod N)
	negTheta0 := new(big.Int).Sub(N, theta0)
	negTheta0.Mod(negTheta0, N)
	r3[1].Content.(*SignRound3Message).Theta = negTheta0

	// party 2's theta = N (which is 0 mod N, but non-zero as a big.Int
	// so the type assertion won't panic).
	r3[2].Content.(*SignRound3Message).Theta = new(big.Int).Set(N)

	_, err := SignRound4(f.States[0], r3)
	if err == nil {
		t.Fatal("expected 'theta is zero' error, but SignRound4 succeeded")
	}
	if !strings.Contains(err.Error(), "theta is zero") {
		t.Fatalf("expected error containing 'theta is zero', got: %v", err)
	}
}

// ===========================================================================
// SignRound5 negative tests
// ===========================================================================

func TestSignRound5RejectsBadCommitment(t *testing.T) {
	f := setupThroughRound4(t)

	// Corrupt the commitment that party 1 broadcast in Round1Message2.
	// SignRound5 reads this from f.States[0].temp.signRound1Message2s[1].
	r1m2 := f.States[0].temp.signRound1Message2s[1].Content.(*SignRound1Message2)
	r1m2.Commitment = new(big.Int).Add(r1m2.Commitment, big.NewInt(1))

	r4 := CloneBcastSlice(f.R4Bcast, CloneR4BcastMsg)
	_, err := SignRound5(f.States[0], r4)
	if err == nil {
		t.Fatal("expected 'commitment verify failed' error, but SignRound5 succeeded")
	}
	if !strings.Contains(err.Error(), "commitment verify failed") {
		t.Fatalf("expected error containing 'commitment verify failed', got: %v", err)
	}
	requireCulprit(t, err, 1)
}

func TestSignRound5RejectsNilZKProof(t *testing.T) {
	f := setupThroughRound4(t)

	r4 := CloneBcastSlice(f.R4Bcast, CloneR4BcastMsg)

	// Set party 1's ZKProof to nil.
	r4[1].Content.(*SignRound4Message).ZKProof = nil

	_, err := SignRound5(f.States[0], r4)
	if err == nil {
		t.Fatal("expected 'bigGamma proof missing' error, but SignRound5 succeeded")
	}
	if !strings.Contains(err.Error(), "bigGamma proof missing") {
		t.Fatalf("expected error containing 'bigGamma proof missing', got: %v", err)
	}
	requireCulprit(t, err, 1)
}

func TestSignRound5RejectsBadZKProof(t *testing.T) {
	f := setupThroughRound4(t)

	r4 := CloneBcastSlice(f.R4Bcast, CloneR4BcastMsg)

	// Corrupt the T field of party 1's ZKProof so that verification fails.
	proof := r4[1].Content.(*SignRound4Message).ZKProof
	proof.T = new(big.Int).Add(proof.T, big.NewInt(1))

	_, err := SignRound5(f.States[0], r4)
	if err == nil {
		t.Fatal("expected 'bigGamma proof verify failed' error, but SignRound5 succeeded")
	}
	if !strings.Contains(err.Error(), "bigGamma proof verify failed") {
		t.Fatalf("expected error containing 'bigGamma proof verify failed', got: %v", err)
	}
	requireCulprit(t, err, 1)
}

func TestSignRound5RejectsBadDecommitmentPoint(t *testing.T) {
	// Tests error path at line 455-457: the decommitted values form
	// coordinates that are not on the curve, so NewECPoint fails.
	f := setupThroughRound4(t)

	r4 := CloneBcastSlice(f.R4Bcast, CloneR4BcastMsg)

	// Strategy: replace party 1's Round1Message2 commitment AND
	// Round4 decommitment with a fresh commitment to an off-curve point.
	// (1, 1) is not on secp256k1 (y^2 != x^3 + 7 mod p).
	offX := big.NewInt(1)
	offY := big.NewInt(1)
	fakeCmt := cmt.NewHashCommitment(f.States[0].params.Rand(), offX, offY)

	// Overwrite party 1's Round1Message2 commitment in party 0's state.
	f.States[0].temp.signRound1Message2s[1] = &tss.Message{
		From:        f.States[0].temp.signRound1Message2s[1].From,
		IsBroadcast: true,
		Content:     &SignRound1Message2{Commitment: fakeCmt.C},
	}

	// Overwrite party 1's Round4 decommitment. The ZKProof is kept as-is
	// because NewECPoint fails before the proof is checked.
	r4Content := r4[1].Content.(*SignRound4Message)
	r4Content.DeCommitment = fakeCmt.D

	_, err := SignRound5(f.States[0], r4)
	if err == nil {
		t.Fatal("expected error for off-curve decommitment point, but SignRound5 succeeded")
	}
	// The error comes from NewECPoint ("not on the elliptic curve")
	// wrapped by tss.NewError.
	if !strings.Contains(err.Error(), "not on the") {
		t.Fatalf("expected 'not on the elliptic curve' error, got: %v", err)
	}
	requireCulprit(t, err, 1)
	t.Logf("got expected error for off-curve point: %v", err)
}
