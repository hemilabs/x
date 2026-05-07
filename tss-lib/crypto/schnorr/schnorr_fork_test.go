// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package schnorr_test

import (
	"crypto/rand"
	"math/big"
	"testing"


	"github.com/hemilabs/x/tss-lib/v3/common"
	"github.com/hemilabs/x/tss-lib/v3/crypto"
	"github.com/hemilabs/x/tss-lib/v3/crypto/schnorr"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

var forkSession = []byte("fork-session")

// TestZKProofRejectsTEqualToQ verifies that ZKProof.Verify rejects a proof
// where T has been tampered to equal q (the curve order). The fork's range
// check (T < q) must catch this.
func TestZKProofRejectsTEqualToQ(t *testing.T) {
	q := tss.S256().Params().N
	x := common.GetRandomPositiveInt(rand.Reader, q)
	X := crypto.ScalarBaseMult(tss.S256(), x)

	pf, err := schnorr.NewZKProof(forkSession, x, X, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	// Sanity: honest proof verifies.
	if !pf.Verify(forkSession, X) {
		t.Fatal("honest proof must verify")
	}

	// Tamper: set T = q (out of range [0, q)).
	pf.T = new(big.Int).Set(q)
	if pf.Verify(forkSession, X) {
		t.Fatal("proof with T == q must be rejected")
	}
}

// TestZKProofRejectsTGreaterThanQ verifies that ZKProof.Verify rejects a proof
// where T has been shifted by +q. Since T + q is congruent to T mod q, the
// algebraic check would pass without the range check.
func TestZKProofRejectsTGreaterThanQ(t *testing.T) {
	q := tss.S256().Params().N
	x := common.GetRandomPositiveInt(rand.Reader, q)
	X := crypto.ScalarBaseMult(tss.S256(), x)

	pf, err := schnorr.NewZKProof(forkSession, x, X, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	// Tamper: set T = T + q (congruent mod q, but out of range).
	pf.T = new(big.Int).Add(pf.T, q)
	if pf.Verify(forkSession, X) {
		t.Fatal("proof with T >= q must be rejected")
	}
}

// TestZKProofRejectsNegativeT verifies that ZKProof.Verify rejects a proof
// where T is negative (Sign() < 0 check in the fork).
func TestZKProofRejectsNegativeT(t *testing.T) {
	q := tss.S256().Params().N
	x := common.GetRandomPositiveInt(rand.Reader, q)
	X := crypto.ScalarBaseMult(tss.S256(), x)

	pf, err := schnorr.NewZKProof(forkSession, x, X, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	// Tamper: set T = -1.
	pf.T = big.NewInt(-1)
	if pf.Verify(forkSession, X) {
		t.Fatal("proof with negative T must be rejected")
	}
}

// TestZKVProofRejectsTOutOfRange verifies that ZKVProof.Verify rejects a proof
// where T has been tampered to equal q.
func TestZKVProofRejectsTOutOfRange(t *testing.T) {
	q := tss.S256().Params().N
	k := common.GetRandomPositiveInt(rand.Reader, q)
	s := common.GetRandomPositiveInt(rand.Reader, q)
	l := common.GetRandomPositiveInt(rand.Reader, q)

	R := crypto.ScalarBaseMult(tss.S256(), k)
	Rs := R.ScalarMult(s)
	lG := crypto.ScalarBaseMult(tss.S256(), l)
	V, err := Rs.Add(lG)
	if err != nil {
		t.Fatal(err)
	}

	pf, err := schnorr.NewZKVProof(forkSession, V, R, s, l, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	// Sanity: honest proof verifies.
	if !pf.Verify(forkSession, V, R) {
		t.Fatal("honest ZKVProof must verify")
	}

	// Tamper: set T = q.
	pf.T = new(big.Int).Set(q)
	if pf.Verify(forkSession, V, R) {
		t.Fatal("ZKVProof with T == q must be rejected")
	}
}

// TestZKVProofRejectsUOutOfRange verifies that ZKVProof.Verify rejects a proof
// where U has been tampered to equal q.
func TestZKVProofRejectsUOutOfRange(t *testing.T) {
	q := tss.S256().Params().N
	k := common.GetRandomPositiveInt(rand.Reader, q)
	s := common.GetRandomPositiveInt(rand.Reader, q)
	l := common.GetRandomPositiveInt(rand.Reader, q)

	R := crypto.ScalarBaseMult(tss.S256(), k)
	Rs := R.ScalarMult(s)
	lG := crypto.ScalarBaseMult(tss.S256(), l)
	V, err := Rs.Add(lG)
	if err != nil {
		t.Fatal(err)
	}

	pf, err := schnorr.NewZKVProof(forkSession, V, R, s, l, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	// Sanity: honest proof verifies.
	if !pf.Verify(forkSession, V, R) {
		t.Fatal("honest ZKVProof must verify")
	}

	// Tamper: set U = q.
	pf.U = new(big.Int).Set(q)
	if pf.Verify(forkSession, V, R) {
		t.Fatal("ZKVProof with U == q must be rejected")
	}
}

// TestZKProofRejectsWrongSession verifies that a ZKProof generated with one
// session tag is rejected when verified with a different session tag.
func TestZKProofRejectsWrongSession(t *testing.T) {
	sessionA := []byte("session-A")
	sessionB := []byte("session-B")

	q := tss.S256().Params().N
	x := common.GetRandomPositiveInt(rand.Reader, q)
	X := crypto.ScalarBaseMult(tss.S256(), x)

	pf, err := schnorr.NewZKProof(sessionA, x, X, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	// Sanity: proof verifies with the correct session.
	if !pf.Verify(sessionA, X) {
		t.Fatal("proof must verify with correct session")
	}

	// Cross-session: proof must not verify with a different session.
	if pf.Verify(sessionB, X) {
		t.Fatal("proof must be rejected with wrong session")
	}
}

// TestZKVProofRejectsWrongSession verifies that a ZKVProof generated with one
// session tag is rejected when verified with a different session tag.
func TestZKVProofRejectsWrongSession(t *testing.T) {
	sessionA := []byte("session-A")
	sessionB := []byte("session-B")

	q := tss.S256().Params().N
	k := common.GetRandomPositiveInt(rand.Reader, q)
	s := common.GetRandomPositiveInt(rand.Reader, q)
	l := common.GetRandomPositiveInt(rand.Reader, q)

	R := crypto.ScalarBaseMult(tss.S256(), k)
	Rs := R.ScalarMult(s)
	lG := crypto.ScalarBaseMult(tss.S256(), l)
	V, err := Rs.Add(lG)
	if err != nil {
		t.Fatal(err)
	}

	pf, err := schnorr.NewZKVProof(sessionA, V, R, s, l, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	// Sanity: proof verifies with the correct session.
	if !pf.Verify(sessionA, V, R) {
		t.Fatal("ZKVProof must verify with correct session")
	}

	// Cross-session: proof must not verify with a different session.
	if pf.Verify(sessionB, V, R) {
		t.Fatal("ZKVProof must be rejected with wrong session")
	}
}
