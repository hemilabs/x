// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package signing

import (
	"math/big"
	"strings"
	"testing"

	"github.com/hemilabs/x/tss/v3/tss"
)

// TestSignFinalizeRejectsNegativeSi verifies that SignFinalize rejects a
// Round 9 message whose si value is negative.  This exercises the
// sj.Sign() < 0 branch at round_fn.go:701.
func TestSignFinalizeRejectsNegativeSi(t *testing.T) {
	f := setupThroughRound9(t)
	r9 := CloneBcastSlice(f.R9Bcast, CloneR9BcastMsg)

	// Corrupt party 1's si to be negative.
	r9[1].Content.(*SignRound9Message).S = new(big.Int).Neg(big.NewInt(42))

	_, err := SignFinalize(f.States[0], r9)
	if err == nil {
		t.Fatal("expected error for negative si, got nil")
	}
	if !strings.Contains(err.Error(), "outside [0, N)") {
		t.Fatalf("unexpected error message: %v", err)
	}
	t.Logf("correctly rejected negative si: %v", err)
}

// TestSignFinalizeRejectsOversizedSi verifies that SignFinalize rejects a
// Round 9 message whose si value equals or exceeds the curve order N.
// This exercises the sj.Cmp(N) >= 0 branch at round_fn.go:701.
func TestSignFinalizeRejectsOversizedSi(t *testing.T) {
	f := setupThroughRound9(t)
	N := tss.S256().Params().N

	// Sub-test: si == N (exactly equal to curve order).
	t.Run("si_equals_N", func(t *testing.T) {
		r9 := CloneBcastSlice(f.R9Bcast, CloneR9BcastMsg)
		r9[1].Content.(*SignRound9Message).S = new(big.Int).Set(N)

		_, err := SignFinalize(f.States[0], r9)
		if err == nil {
			t.Fatal("expected error for si == N, got nil")
		}
		if !strings.Contains(err.Error(), "outside [0, N)") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	// Sub-test: si == N + 1 (exceeds curve order).
	t.Run("si_exceeds_N", func(t *testing.T) {
		r9 := CloneBcastSlice(f.R9Bcast, CloneR9BcastMsg)
		r9[1].Content.(*SignRound9Message).S = new(big.Int).Add(N, big.NewInt(1))

		_, err := SignFinalize(f.States[0], r9)
		if err == nil {
			t.Fatal("expected error for si > N, got nil")
		}
		if !strings.Contains(err.Error(), "outside [0, N)") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})
}

// TestSignFinalizeRejectsZeroSumS verifies that SignFinalize rejects when
// all si shares sum to zero mod N.  This exercises the sumS.Sign() == 0
// guard at round_fn.go:706-707.
func TestSignFinalizeRejectsZeroSumS(t *testing.T) {
	f := setupThroughRound9(t)
	N := tss.S256().Params().N
	r9 := CloneBcastSlice(f.R9Bcast, CloneR9BcastMsg)

	// Compute the sum of si for parties 0 and 1, then set party 2's si
	// to the additive inverse so the total is zero mod N.
	s0 := f.States[0].temp.si
	s1 := r9[1].Content.(*SignRound9Message).S
	sumOthers := new(big.Int).Add(s0, s1)
	sumOthers.Mod(sumOthers, N)
	// party 2's si = N - sumOthers (i.e., -sumOthers mod N)
	r9[2].Content.(*SignRound9Message).S = new(big.Int).Sub(N, sumOthers)

	_, err := SignFinalize(f.States[0], r9)
	if err == nil {
		t.Fatal("expected error for zero accumulated S, got nil")
	}
	if !strings.Contains(err.Error(), "accumulated S is zero") {
		t.Fatalf("unexpected error message: %v", err)
	}
	t.Logf("correctly rejected zero sum S: %v", err)
}

// TestSignFinalizeRejectsCorruptedSi verifies that SignFinalize rejects a
// Round 9 message whose si is a valid-range value but produces an
// incorrect aggregate S, causing ECDSA signature verification to fail.
// This exercises the ecdsa.Verify failure branch at round_fn.go:743-744.
func TestSignFinalizeRejectsCorruptedSi(t *testing.T) {
	f := setupThroughRound9(t)
	N := tss.S256().Params().N
	r9 := CloneBcastSlice(f.R9Bcast, CloneR9BcastMsg)

	// Corrupt party 2's si to a different valid value (si + 1 mod N).
	// This keeps si in [1, N-1] but changes the aggregate S, so the
	// final ECDSA verification must fail.
	r9msg := r9[2].Content.(*SignRound9Message)
	corrupted := new(big.Int).Add(r9msg.S, big.NewInt(1))
	corrupted.Mod(corrupted, N)
	// Ensure the corrupted value is not zero (would hit a different path).
	if corrupted.Sign() == 0 {
		corrupted.SetInt64(1)
	}
	r9msg.S = corrupted

	_, err := SignFinalize(f.States[0], r9)
	if err == nil {
		t.Fatal("expected error for corrupted si, got nil")
	}
	if !strings.Contains(err.Error(), "signature verification failed") {
		t.Fatalf("unexpected error message: %v", err)
	}
	t.Logf("correctly rejected corrupted si: %v", err)
}
