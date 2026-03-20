// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package signing

import (
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss/v3/crypto"
	cmt "github.com/hemilabs/x/tss/v3/crypto/commitments"
	"github.com/hemilabs/x/tss/v3/crypto/schnorr"
	"github.com/hemilabs/x/tss/v3/tss"
)

func TestSignRound2MessageValidateBasic(t *testing.T) {
	if (*SignRound2Message)(nil).ValidateBasic() {
		t.Fatal("nil should fail")
	}
	if (&SignRound2Message{}).ValidateBasic() {
		t.Fatal("zero-value should fail")
	}
	ec := tss.Edwards()
	alpha := crypto.ScalarBaseMult(ec, big.NewInt(7))
	proof := &schnorr.ZKProof{Alpha: alpha, T: big.NewInt(99)}
	if !(&SignRound2Message{
		DeCommitment: cmt.HashDeCommitment{big.NewInt(1), big.NewInt(2)},
		ZKProof:      proof,
	}).ValidateBasic() {
		t.Fatal("valid should pass")
	}
}

func TestSignRound3MessageValidateBasic(t *testing.T) {
	if (*SignRound3Message)(nil).ValidateBasic() {
		t.Fatal("nil should fail")
	}
	if (&SignRound3Message{}).ValidateBasic() {
		t.Fatal("zero-value should fail")
	}
	if !(&SignRound3Message{S: big.NewInt(42)}).ValidateBasic() {
		t.Fatal("valid should pass")
	}
}

func TestBigIntToEncodedBytesNil(t *testing.T) {
	result := bigIntToEncodedBytes(nil)
	if result == nil {
		t.Fatal("nil input should return zero bytes, not nil")
	}
	for _, b := range result {
		if b != 0 {
			t.Fatal("nil input should produce all zeros")
		}
	}
}

func TestCopyBytesNil(t *testing.T) {
	if copyBytes(nil) != nil {
		t.Fatal("nil input should return nil")
	}
}

func TestCopyBytesTooLong(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("should panic on > 32 bytes")
		}
	}()
	copyBytes(make([]byte, 33))
}

func TestPrepareForSigningPanics(t *testing.T) {
	ec := tss.Edwards()
	N := ec.Params().N

	t.Run("len mismatch", func(t *testing.T) {
		defer func() { _ = recover() }()
		PrepareForSigning(ec, 0, 3, big.NewInt(1), []*big.Int{big.NewInt(1), big.NewInt(2)})
		t.Fatal("should panic")
	})

	t.Run("i out of range", func(t *testing.T) {
		defer func() { _ = recover() }()
		PrepareForSigning(ec, 5, 3, big.NewInt(1), []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)})
		t.Fatal("should panic")
	})

	t.Run("equal keys", func(t *testing.T) {
		defer func() { _ = recover() }()
		PrepareForSigning(ec, 0, 2, big.NewInt(1), []*big.Int{big.NewInt(1), big.NewInt(1)})
		t.Fatal("should panic")
	})

	_ = N
}
