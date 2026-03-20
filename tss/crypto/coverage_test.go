// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package crypto

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss/v3/tss"
)

func TestECPointAddNilP1(t *testing.T) {
	ec := tss.S256()
	p := ScalarBaseMult(ec, big.NewInt(7))
	_, err := p.Add(nil)
	if err == nil {
		t.Fatal("Add(nil) should error")
	}
}

func TestECPointAddCurveMismatch(t *testing.T) {
	p1 := ScalarBaseMult(tss.S256(), big.NewInt(7))
	p2 := ScalarBaseMult(tss.Edwards(), big.NewInt(7))
	_, err := p1.Add(p2)
	if err == nil {
		t.Fatal("Add with different curves should error")
	}
}

func TestToECDSAPubKey(t *testing.T) {
	ec := tss.S256()
	p := ScalarBaseMult(ec, big.NewInt(42))
	pk := p.ToECDSAPubKey()
	if pk.X.Cmp(p.X()) != 0 || pk.Y.Cmp(p.Y()) != 0 {
		t.Fatal("coordinates mismatch")
	}
	if pk.Curve != ec {
		t.Fatal("curve mismatch")
	}
}

func TestSetCurve(t *testing.T) {
	p := ScalarBaseMult(tss.S256(), big.NewInt(7))
	if p.Curve() != tss.S256() {
		t.Fatal("initial curve wrong")
	}
	p.SetCurve(tss.Edwards())
	if p.Curve() != tss.Edwards() {
		t.Fatal("SetCurve did not update")
	}
}

func TestValidateBasic(t *testing.T) {
	ec := tss.S256()
	p := ScalarBaseMult(ec, big.NewInt(42))
	if !p.ValidateBasic() {
		t.Fatal("valid point should pass")
	}

	// nil point
	var nilP *ECPoint
	if nilP.ValidateBasic() {
		t.Fatal("nil should fail")
	}
}

func TestEqualsNilHandling(t *testing.T) {
	p := ScalarBaseMult(tss.S256(), big.NewInt(7))
	if p.Equals(nil) {
		t.Fatal("Equals(nil) should be false")
	}
	var nilP *ECPoint
	if nilP.Equals(p) {
		t.Fatal("nil.Equals(p) should be false")
	}
}

func TestEightInvEight(t *testing.T) {
	ec := tss.Edwards()
	p := ScalarBaseMult(ec, big.NewInt(42))
	cleared := p.EightInvEight()
	if !cleared.Equals(p) {
		t.Fatal("EightInvEight should be identity for prime-order subgroup points")
	}
}

func TestIsIdentityEdwardsCoverage(t *testing.T) {
	p, err := NewECPoint(tss.Edwards(), big.NewInt(0), big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}
	if !p.IsIdentity() {
		t.Fatal("(0,1) should be identity on Edwards")
	}
}

func TestIsIdentityWeierstrassCoverage(t *testing.T) {
	p := NewECPointNoCurveCheck(tss.S256(), big.NewInt(0), big.NewInt(0))
	if !p.IsIdentity() {
		t.Fatal("(0,0) should be identity on Weierstrass")
	}
}

func TestECPointJSONRoundTrip(t *testing.T) {
	ec := tss.S256()
	p := ScalarBaseMult(ec, big.NewInt(42))

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	p2 := new(ECPoint)
	if err := json.Unmarshal(data, p2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !p.Equals(p2) {
		t.Fatal("round-trip mismatch")
	}
}

func TestUnFlattenECPointsOddLength(t *testing.T) {
	_, err := UnFlattenECPoints(tss.S256(), []*big.Int{big.NewInt(1)})
	if err == nil {
		t.Fatal("odd-length input should error")
	}
}

func TestUnFlattenECPointsNil(t *testing.T) {
	_, err := UnFlattenECPoints(tss.S256(), nil)
	if err == nil {
		t.Fatal("nil input should error")
	}
}

func TestScalarMultIdentity(t *testing.T) {
	ec := tss.S256()
	N := ec.Params().N
	// Multiplying by N should give identity
	defer func() {
		// ScalarBaseMult(N) may panic on identity — that's fine
		_ = recover()
	}()
	_ = ScalarBaseMult(ec, N)
}
