// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package resharing

import (
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss/v3/crypto"
	cmt "github.com/hemilabs/x/tss/v3/crypto/commitments"
	"github.com/hemilabs/x/tss/v3/tss"
)

func TestDGRound1MessageValidateBasic(t *testing.T) {
	if (*DGRound1Message)(nil).ValidateBasic() {
		t.Fatal("nil should fail")
	}
	if (&DGRound1Message{}).ValidateBasic() {
		t.Fatal("zero-value should fail")
	}
	ec := tss.Edwards()
	pt := crypto.ScalarBaseMult(ec, big.NewInt(42))
	if !(&DGRound1Message{EDDSAPub: pt, VCommitment: big.NewInt(1)}).ValidateBasic() {
		t.Fatal("valid should pass")
	}
}

func TestDGRound2MessageValidateBasic(t *testing.T) {
	if (*DGRound2Message)(nil).ValidateBasic() {
		t.Fatal("nil should fail")
	}
	if !(&DGRound2Message{}).ValidateBasic() {
		t.Fatal("valid should pass")
	}
}

func TestDGRound3Message1ValidateBasic(t *testing.T) {
	if (*DGRound3Message1)(nil).ValidateBasic() {
		t.Fatal("nil should fail")
	}
	if (&DGRound3Message1{}).ValidateBasic() {
		t.Fatal("zero-value should fail")
	}
	if (&DGRound3Message1{Share: big.NewInt(1)}).ValidateBasic() {
		t.Fatal("missing ReceiverID should fail")
	}
	if !(&DGRound3Message1{Share: big.NewInt(1), ReceiverID: []byte("x")}).ValidateBasic() {
		t.Fatal("valid should pass")
	}
}

func TestDGRound3Message2ValidateBasic(t *testing.T) {
	if (*DGRound3Message2)(nil).ValidateBasic() {
		t.Fatal("nil should fail")
	}
	if (&DGRound3Message2{}).ValidateBasic() {
		t.Fatal("zero-value should fail")
	}
	if !(&DGRound3Message2{VDeCommitment: cmt.HashDeCommitment{big.NewInt(1), big.NewInt(2)}}).ValidateBasic() {
		t.Fatal("valid should pass")
	}
}

func TestDGRound4MessageValidateBasic(t *testing.T) {
	if (*DGRound4Message)(nil).ValidateBasic() {
		t.Fatal("nil should fail")
	}
	if !(&DGRound4Message{}).ValidateBasic() {
		t.Fatal("valid should pass")
	}
}
