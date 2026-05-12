// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package crypto_test

import (
	"encoding/binary"
	"math/big"
	"strings"
	"testing"

	"github.com/hemilabs/x/tss-lib/v3/crypto"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

func TestIsIdentityWeierstrass(t *testing.T) {
	p := crypto.NewECPointNoCurveCheck(tss.S256(), big.NewInt(0), big.NewInt(0))
	if !p.IsIdentity() {
		t.Fatal("Weierstrass identity (0,0) should be detected")
	}
}

func TestIsIdentityEdwards(t *testing.T) {
	p := crypto.NewECPointNoCurveCheck(tss.Edwards(), big.NewInt(0), big.NewInt(1))
	if !p.IsIdentity() {
		t.Fatal("Edwards identity (0,1) should be detected")
	}
}

func TestIsIdentityNonIdentity(t *testing.T) {
	p := crypto.ScalarBaseMult(tss.S256(), big.NewInt(42))
	if p.IsIdentity() {
		t.Fatal("a valid on-curve point should not be identity")
	}
}

func TestIsIdentityNilPoint(t *testing.T) {
	var p *crypto.ECPoint
	if !p.IsIdentity() {
		t.Fatal("nil ECPoint should be treated as identity")
	}
}

func TestScalarMultByGroupOrder(t *testing.T) {
	q := tss.S256().Params().N
	g := crypto.ScalarBaseMult(tss.S256(), big.NewInt(1))
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("G * q should panic (identity point)")
			}
		}()
		g.ScalarMult(q)
	}()
}

func TestScalarBaseMultByZero(t *testing.T) {
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("ScalarBaseMult(0) should panic (identity point)")
			}
		}()
		crypto.ScalarBaseMult(tss.S256(), big.NewInt(0))
	}()
}

func TestAddNilP1(t *testing.T) {
	p := crypto.ScalarBaseMult(tss.S256(), big.NewInt(7))
	_, err := p.Add(nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "p1 is nil") {
		t.Fatalf("expected %q to contain %q", err.Error(), "p1 is nil")
	}
}

func TestGobDecodeRejectsOversizedCoord(t *testing.T) {
	// Craft a payload where X coordinate length prefix is 1025 (exceeds maxCoordLen=1024).
	// Layout: uint32(xLen) | xBytes | uint32(yLen) | yBytes
	const oversizedLen = 1025
	buf := make([]byte, 0, 4+oversizedLen+4+4)

	xLenBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(xLenBytes, uint32(oversizedLen))
	buf = append(buf, xLenBytes...)
	buf = append(buf, make([]byte, oversizedLen)...)

	yLenBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(yLenBytes, uint32(4))
	buf = append(buf, yLenBytes...)
	buf = append(buf, make([]byte, 4)...)

	p := &crypto.ECPoint{}
	err := p.GobDecode(buf)
	if err == nil {
		t.Fatal("GobDecode should reject oversized coordinate")
	}
}

func TestGobDecodeAcceptsExactBoundaryCoord(t *testing.T) {
	// Craft a payload where X coordinate length is exactly 1024 (the maxCoordLen boundary).
	// The value is not a valid EC point, but GobDecode should NOT reject it due to size.
	// It should fail for a different reason (e.g., big.Int GobDecode format or "not on curve").
	const exactLen = 1024
	buf := make([]byte, 0, 4+exactLen+4+4)

	xLenBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(xLenBytes, uint32(exactLen))
	buf = append(buf, xLenBytes...)
	buf = append(buf, make([]byte, exactLen)...)

	yLenBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(yLenBytes, uint32(4))
	buf = append(buf, yLenBytes...)
	buf = append(buf, make([]byte, 4)...)

	p := &crypto.ECPoint{}
	err := p.GobDecode(buf)
	// It may fail (invalid big.Int encoding, not on curve, etc.) but must NOT be the size check.
	if err != nil {
		if strings.Contains(err.Error(), "exceeds maximum") {
			t.Fatal("1024-byte coordinate must not be rejected by the size check")
		}
	}
}

func TestGobDecodeRoundTrip(t *testing.T) {
	original := crypto.ScalarBaseMult(tss.S256(), big.NewInt(42))

	encoded, err := original.GobEncode()
	if err != nil {
		t.Fatal(err)
	}

	decoded := &crypto.ECPoint{}
	err = decoded.GobDecode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	if !original.Equals(decoded) {
		t.Fatal("round-tripped point should equal original")
	}
}
