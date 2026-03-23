// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package crypto_test

import (
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hemilabs/x/tss-lib/v3/crypto"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

func TestIsIdentityWeierstrass(t *testing.T) {
	p := crypto.NewECPointNoCurveCheck(tss.S256(), big.NewInt(0), big.NewInt(0))
	assert.True(t, p.IsIdentity(), "Weierstrass identity (0,0) should be detected")
}

func TestIsIdentityEdwards(t *testing.T) {
	p := crypto.NewECPointNoCurveCheck(tss.Edwards(), big.NewInt(0), big.NewInt(1))
	assert.True(t, p.IsIdentity(), "Edwards identity (0,1) should be detected")
}

func TestIsIdentityNonIdentity(t *testing.T) {
	p := crypto.ScalarBaseMult(tss.S256(), big.NewInt(42))
	assert.False(t, p.IsIdentity(), "a valid on-curve point should not be identity")
}

func TestIsIdentityNilPoint(t *testing.T) {
	var p *crypto.ECPoint
	assert.True(t, p.IsIdentity(), "nil ECPoint should be treated as identity")
}

func TestScalarMultByGroupOrder(t *testing.T) {
	q := tss.S256().Params().N
	g := crypto.ScalarBaseMult(tss.S256(), big.NewInt(1))
	assert.Panics(t, func() { g.ScalarMult(q) }, "G * q should panic (identity point)")
}

func TestScalarBaseMultByZero(t *testing.T) {
	assert.Panics(t, func() { crypto.ScalarBaseMult(tss.S256(), big.NewInt(0)) }, "ScalarBaseMult(0) should panic (identity point)")
}

func TestAddNilP1(t *testing.T) {
	p := crypto.ScalarBaseMult(tss.S256(), big.NewInt(7))
	_, err := p.Add(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "p1 is nil")
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
	assert.Error(t, err, "GobDecode should reject oversized coordinate")
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
		assert.NotContains(t, err.Error(), "exceeds maximum", "1024-byte coordinate must not be rejected by the size check")
	}
}

func TestGobDecodeRoundTrip(t *testing.T) {
	original := crypto.ScalarBaseMult(tss.S256(), big.NewInt(42))

	encoded, err := original.GobEncode()
	assert.NoError(t, err)

	decoded := &crypto.ECPoint{}
	err = decoded.GobDecode(encoded)
	assert.NoError(t, err)

	assert.True(t, original.Equals(decoded), "round-tripped point should equal original")
}
