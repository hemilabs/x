// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package modproof

import (
	"math/big"
	"testing"
)

func TestValidateBasicNilW(t *testing.T) {
	pf := &ProofMod{W: nil}
	if pf.ValidateBasic() {
		t.Fatal("nil W should fail")
	}
}

func TestValidateBasicNilXElement(t *testing.T) {
	pf := &ProofMod{
		W: big.NewInt(1),
		A: big.NewInt(1),
		B: big.NewInt(1),
	}
	pf.X[0] = nil
	if pf.ValidateBasic() {
		t.Fatal("nil X[0] should fail")
	}
}

func TestValidateBasicNilA(t *testing.T) {
	pf := &ProofMod{W: big.NewInt(1), A: nil}
	for i := range pf.X {
		pf.X[i] = big.NewInt(1)
	}
	if pf.ValidateBasic() {
		t.Fatal("nil A should fail")
	}
}

func TestValidateBasicNilB(t *testing.T) {
	pf := &ProofMod{W: big.NewInt(1), A: big.NewInt(1), B: nil}
	for i := range pf.X {
		pf.X[i] = big.NewInt(1)
	}
	if pf.ValidateBasic() {
		t.Fatal("nil B should fail")
	}
}

func TestValidateBasicNilZElement(t *testing.T) {
	pf := &ProofMod{W: big.NewInt(1), A: big.NewInt(1), B: big.NewInt(1)}
	for i := range pf.X {
		pf.X[i] = big.NewInt(1)
	}
	pf.Z[0] = nil
	if pf.ValidateBasic() {
		t.Fatal("nil Z[0] should fail")
	}
}

func TestValidateBasicAllValid(t *testing.T) {
	pf := &ProofMod{W: big.NewInt(1), A: big.NewInt(1), B: big.NewInt(1)}
	for i := range pf.X {
		pf.X[i] = big.NewInt(1)
	}
	for i := range pf.Z {
		pf.Z[i] = big.NewInt(1)
	}
	if !pf.ValidateBasic() {
		t.Fatal("all-valid should pass")
	}
}

func TestNewProofFromBytesTruncated(t *testing.T) {
	_, err := NewProofFromBytes([][]byte{{1}})
	if err == nil {
		t.Fatal("expected error for truncated bytes")
	}
}
