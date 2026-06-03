// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package crypto

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss-lib/v3/tss"
)

func TestEightInvEightIdentityPoint(t *testing.T) {
	ec := tss.S256()
	id := NewECPointNoCurveCheck(ec, big.NewInt(0), big.NewInt(0))
	if !id.IsIdentity() {
		t.Fatal("(0,0) should be identity")
	}
	result := id.EightInvEight()
	if !result.IsIdentity() {
		t.Fatal("EightInvEight of identity should be identity")
	}
}

func TestNewECPointNilXY(t *testing.T) {
	ec := tss.S256()
	_, err := NewECPoint(ec, nil, big.NewInt(1))
	if err == nil {
		t.Fatal("expected error for nil x")
	}
	_, err = NewECPoint(ec, big.NewInt(1), nil)
	if err == nil {
		t.Fatal("expected error for nil y")
	}
}

func TestUnFlattenBadPoint(t *testing.T) {
	_, err := UnFlattenECPoints(tss.S256(), []*big.Int{big.NewInt(999), big.NewInt(999)})
	if err == nil {
		t.Fatal("expected error for off-curve point")
	}
}

func TestUnFlattenNoCurveCheck(t *testing.T) {
	pts, err := UnFlattenECPoints(tss.S256(), []*big.Int{big.NewInt(999), big.NewInt(999)}, true)
	if err != nil {
		t.Fatalf("noCurveCheck should not error: %v", err)
	}
	if len(pts) != 1 {
		t.Fatalf("expected 1 point, got %d", len(pts))
	}
}

func TestGobDecodeShortPayload(t *testing.T) {
	p := new(ECPoint)
	if err := p.GobDecode([]byte{1, 2}); err == nil {
		t.Fatal("expected error for short data")
	}
}

func TestGobDecodeOversizedY(t *testing.T) {
	ec := tss.S256()
	pt := ScalarBaseMult(ec, big.NewInt(1))
	xBytes, _ := pt.X().GobEncode()
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(xBytes))) //nolint:gosec // test data
	buf.Write(xBytes)
	_ = binary.Write(buf, binary.LittleEndian, uint32(2048))
	p := new(ECPoint)
	if err := p.GobDecode(buf.Bytes()); err == nil {
		t.Fatal("expected error for oversize y coordinate")
	}
}

func TestGobDecodeOffCurvePoint(t *testing.T) {
	xBytes, _ := big.NewInt(999).GobEncode()
	yBytes, _ := big.NewInt(999).GobEncode()
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(xBytes))) //nolint:gosec // test data
	buf.Write(xBytes)
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(yBytes))) //nolint:gosec // test data
	buf.Write(yBytes)
	p := new(ECPoint)
	if err := p.GobDecode(buf.Bytes()); err == nil {
		t.Fatal("expected error for off-curve point")
	}
}

func TestUnmarshalJSONBadJSON(t *testing.T) {
	p := new(ECPoint)
	if err := p.UnmarshalJSON([]byte("not json")); err == nil {
		t.Fatal("expected error for bad JSON")
	}
}

func TestUnmarshalJSONBadCurveName(t *testing.T) {
	payload := `{"Curve":"nonexistent","Coords":[1,2]}`
	p := new(ECPoint)
	if err := p.UnmarshalJSON([]byte(payload)); err == nil {
		t.Fatal("expected error for unknown curve name")
	}
}

func TestUnmarshalJSONOffCurvePoint(t *testing.T) {
	payload := `{"Curve":"secp256k1","Coords":[999,999]}`
	p := new(ECPoint)
	if err := p.UnmarshalJSON([]byte(payload)); err == nil {
		t.Fatal("expected error for off-curve point")
	}
}

func TestMarshalJSONNilCurve(t *testing.T) {
	p := NewECPointNoCurveCheck(nil, big.NewInt(1), big.NewInt(2))
	_, err := json.Marshal(p)
	if err == nil {
		t.Fatal("expected error for unregistered curve")
	}
}

func TestUnmarshalJSONEmptyCurveFallback(t *testing.T) {
	ec := tss.S256()
	p := ScalarBaseMult(ec, big.NewInt(42))
	data, _ := p.MarshalJSON()
	var aux struct {
		Curve  string
		Coords [2]*big.Int
	}
	_ = json.Unmarshal(data, &aux)
	aux.Curve = ""
	modified, _ := json.Marshal(aux)

	p2 := new(ECPoint)
	if err := p2.UnmarshalJSON(modified); err != nil {
		t.Fatalf("empty curve should use default: %v", err)
	}
}
