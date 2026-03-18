// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

// Negative tests for SignRound7 and SignRound9.
//
// Error paths in SignRound7 (round_fn.go lines 546-626):
//   Path 1 (tested): de-commitment failed (line 564-566)
//   Path 2 (infeasible): bigVj not on curve -- commitment binding prevents
//   Path 3 (infeasible): bigVj is identity -- statistically negligible
//   Path 4 (infeasible): bigAj not on curve -- commitment binding prevents
//   Path 5 (infeasible): bigAj is identity -- statistically negligible
//   Path 6 (tested): schnorr Aj verify failed (line 583-586)
//   Path 7 (tested): vverify Vj failed (line 587-590)
//   Path 8 (infeasible): Ui computation fails -- ScalarMult of valid point
//   Path 9 (infeasible): Ti computation fails -- ScalarMult of valid point
//
// Error paths in SignRound9 (round_fn.go lines 638-682):
//   Path 1 (tested): Uj/Tj decommit failed (line 654-656)
//   Path 2 (infeasible): Uj not on curve -- commitment binding
//   Path 3 (infeasible): Uj is identity -- statistically negligible
//   Path 4 (infeasible): Tj not on curve -- commitment binding
//   Path 5 (infeasible): Tj is identity -- statistically negligible
//   Path 6 (not trivially testable): U != T requires internal state manipulation

package signing

import (
	"math/big"
	"strings"
	"testing"

	"github.com/hemilabs/x/tss-lib/v3/crypto/schnorr"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// ---------------------------------------------------------------------------
// SignRound7 negative tests
// ---------------------------------------------------------------------------

// TestSignRound7RejectsBadDecommitment corrupts the decommitment in a Round6
// message so that the commit/decommit check fails (error path line 564-566).
func TestSignRound7RejectsBadDecommitment(t *testing.T) {
	f := setupThroughRound6(t)

	corruptR6 := CloneBcastSlice(f.R6Bcast, CloneR6BcastMsg)

	// Flip a bit in the randomness element of party 1's decommitment.
	origContent := corruptR6[1].Content.(*SignRound6Message)
	corruptD := make([]*big.Int, len(origContent.DeCommitment))
	for i, v := range origContent.DeCommitment {
		corruptD[i] = new(big.Int).Set(v)
	}
	corruptD[0].Xor(corruptD[0], big.NewInt(1)) // flip low bit
	origContent.DeCommitment = corruptD

	_, err := SignRound7(f.States[0], f.R5Bcast, corruptR6)
	if err == nil {
		t.Fatal("expected SignRound7 to fail with corrupted decommitment, but got nil error")
	}
	if !strings.Contains(err.Error(), "de-commitment failed") {
		t.Fatalf("expected 'de-commitment failed' error, got: %v", err)
	}
	t.Logf("SignRound7 correctly rejected bad decommitment: %v", err)
}

// TestSignRound7RejectsBadSchnorrProof corrupts the Schnorr proof T scalar
// in a Round6 message so that the Aj verification fails (line 583-586).
func TestSignRound7RejectsBadSchnorrProof(t *testing.T) {
	f := setupThroughRound6(t)

	corruptR6 := CloneBcastSlice(f.R6Bcast, CloneR6BcastMsg)

	// Add 1 to the Schnorr proof T scalar for party 1.
	origContent := corruptR6[1].Content.(*SignRound6Message)
	N := tss.S256().Params().N
	corruptT := new(big.Int).Add(origContent.ZKProof.T, big.NewInt(1))
	corruptT.Mod(corruptT, N)
	if corruptT.Sign() == 0 {
		corruptT.SetInt64(1)
	}
	origContent.ZKProof = &schnorr.ZKProof{
		Alpha: origContent.ZKProof.Alpha,
		T:     corruptT,
	}

	_, err := SignRound7(f.States[0], f.R5Bcast, corruptR6)
	if err == nil {
		t.Fatal("expected SignRound7 to fail with bad Schnorr proof, but got nil error")
	}
	if !strings.Contains(err.Error(), "schnorr Aj verify failed") {
		t.Fatalf("expected 'schnorr Aj verify failed' error, got: %v", err)
	}
	t.Logf("SignRound7 correctly rejected bad Schnorr proof: %v", err)
}

// TestSignRound7RejectsBadVVerifyProof corrupts the ZKVProof T scalar
// in a Round6 message so that the V-verify fails (line 587-590).
func TestSignRound7RejectsBadVVerifyProof(t *testing.T) {
	f := setupThroughRound6(t)

	corruptR6 := CloneBcastSlice(f.R6Bcast, CloneR6BcastMsg)

	// Add 1 to the ZKVProof T scalar for party 1.
	origContent := corruptR6[1].Content.(*SignRound6Message)
	N := tss.S256().Params().N
	corruptT := new(big.Int).Add(origContent.ZKVProof.T, big.NewInt(1))
	corruptT.Mod(corruptT, N)
	if corruptT.Sign() == 0 {
		corruptT.SetInt64(1)
	}
	origContent.ZKVProof = &schnorr.ZKVProof{
		Alpha: origContent.ZKVProof.Alpha,
		T:     corruptT,
		U:     origContent.ZKVProof.U,
	}

	_, err := SignRound7(f.States[0], f.R5Bcast, corruptR6)
	if err == nil {
		t.Fatal("expected SignRound7 to fail with bad ZKVProof, but got nil error")
	}
	if !strings.Contains(err.Error(), "vverify Vj failed") {
		t.Fatalf("expected 'vverify Vj failed' error, got: %v", err)
	}
	t.Logf("SignRound7 correctly rejected bad ZKVProof: %v", err)
}

// ---------------------------------------------------------------------------
// SignRound9 negative tests
// ---------------------------------------------------------------------------

// TestSignRound9RejectsBadDecommitment corrupts the decommitment in a Round8
// message so that the Uj/Tj decommit check fails (line 654-656).
func TestSignRound9RejectsBadDecommitment(t *testing.T) {
	f := setupThroughRound8(t)

	corruptR8 := CloneBcastSlice(f.R8Bcast, CloneR8BcastMsg)

	// Flip a bit in the randomness element of party 1's decommitment.
	origContent := corruptR8[1].Content.(*SignRound8Message)
	corruptD := make([]*big.Int, len(origContent.DeCommitment))
	for i, v := range origContent.DeCommitment {
		corruptD[i] = new(big.Int).Set(v)
	}
	corruptD[0].Xor(corruptD[0], big.NewInt(1))
	origContent.DeCommitment = corruptD

	_, err := SignRound9(f.States[0], f.R7Bcast, corruptR8)
	if err == nil {
		t.Fatal("expected SignRound9 to fail with corrupted decommitment, but got nil error")
	}
	if !strings.Contains(err.Error(), "Uj/Tj decommit failed") {
		t.Fatalf("expected 'Uj/Tj decommit failed' error, got: %v", err)
	}
	t.Logf("SignRound9 correctly rejected bad decommitment: %v", err)
}
