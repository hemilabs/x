// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package crypto

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/decred/dcrd/dcrec/edwards/v2"

	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// ECPoint convenience helper
type ECPoint struct {
	curve  elliptic.Curve
	coords [2]*big.Int
}

var (
	eight    = big.NewInt(8)
	eightInv = new(big.Int).ModInverse(eight, edwards.Edwards().Params().N)
)

// Creates a new ECPoint and checks that the given coordinates are on the elliptic curve.
func NewECPoint(curve elliptic.Curve, X, Y *big.Int) (*ECPoint, error) {
	if !isOnCurve(curve, X, Y) {
		return nil, fmt.Errorf("NewECPoint: the given point is not on the elliptic curve")
	}
	return &ECPoint{curve, [2]*big.Int{X, Y}}, nil
}

// Creates a new ECPoint without checking that the coordinates are on the elliptic curve.
// Only use this function when you are completely sure that the point is already on the curve.
func NewECPointNoCurveCheck(curve elliptic.Curve, X, Y *big.Int) *ECPoint {
	return &ECPoint{curve, [2]*big.Int{X, Y}}
}

func (p *ECPoint) X() *big.Int {
	return new(big.Int).Set(p.coords[0])
}

func (p *ECPoint) Y() *big.Int {
	return new(big.Int).Set(p.coords[1])
}

func (p *ECPoint) Add(p1 *ECPoint) (*ECPoint, error) {
	// [FORK] Upstream does not validate p1 before calling elliptic.Curve.Add. A nil p1
	// causes a nil-pointer panic, and mismatched curves silently miscompute. Added nil
	// check and curve mismatch guard, and wrapped the result in NewECPoint for validation.
	if p1 == nil {
		return nil, fmt.Errorf("ECPoint.Add: p1 is nil")
	}
	if p.curve != p1.curve {
		return nil, fmt.Errorf("ECPoint.Add: cannot add points on different curves")
	}
	x, y := p.curve.Add(p.X(), p.Y(), p1.X(), p1.Y())
	return NewECPoint(p.curve, x, y)
}

func (p *ECPoint) ScalarMult(k *big.Int) *ECPoint {
	x, y := p.curve.ScalarMult(p.X(), p.Y(), k.Bytes())
	// [FORK] Restored upstream panic behavior. Identity results (from zero scalar or
	// group-order multiples) cause NewECPoint to reject (0,0) and panic. This is
	// intentional: ~30 of 34 call sites do not check IsIdentity(), so silently
	// returning identity would propagate bad math through the protocol. Pre-call
	// guards at each site ensure the panic is unreachable in normal operation.
	newP, err := NewECPoint(p.curve, x, y)
	if err != nil {
		panic(fmt.Errorf("scalar mult to an ecpoint %s", err.Error()))
	}
	return newP
}

func (p *ECPoint) ToECDSAPubKey() *ecdsa.PublicKey {
	return &ecdsa.PublicKey{
		Curve: p.curve,
		X:     p.X(),
		Y:     p.Y(),
	}
}

func (p *ECPoint) IsOnCurve() bool {
	return isOnCurve(p.curve, p.coords[0], p.coords[1])
}

func (p *ECPoint) Curve() elliptic.Curve {
	return p.curve
}

func (p *ECPoint) Equals(p2 *ECPoint) bool {
	if p == nil || p2 == nil {
		return false
	}
	return p.X().Cmp(p2.X()) == 0 && p.Y().Cmp(p2.Y()) == 0
}

// [FORK] IsIdentity returns true if this point is the identity element (point at infinity).
// On Weierstrass curves (secp256k1, P-256, etc.), Go represents identity as (0, 0).
// On Edwards curves (edwards25519), the identity is (0, 1).
// New method added to support callers that need to detect identity results from
// ScalarMult/ScalarBaseMult.
func (p *ECPoint) IsIdentity() bool {
	if p == nil {
		return true
	}
	if p.coords[0].Sign() != 0 {
		return false // x != 0 means definitely not identity on any curve
	}
	// x == 0: check y
	if p.coords[1].Sign() == 0 {
		return true // (0, 0) — Weierstrass identity
	}
	if p.coords[1].Cmp(big.NewInt(1)) == 0 {
		return true // (0, 1) — Edwards identity
	}
	return false
}

func (p *ECPoint) SetCurve(curve elliptic.Curve) *ECPoint {
	p.curve = curve
	return p
}

func (p *ECPoint) ValidateBasic() bool {
	return p != nil && p.coords[0] != nil && p.coords[1] != nil && p.IsOnCurve()
}

func (p *ECPoint) EightInvEight() *ECPoint {
	// [FORK] Use raw curve.ScalarMult for the *8 step to detect torsion points without
	// panicking. If p*8 = identity, the point is small-order; return identity directly.
	// Callers' subsequent crypto checks (Schnorr verify, VSS verify) will reject it.
	// CRITICAL: On Edwards25519 the identity is (0, 1), NOT (0, 0). Must use IsIdentity()
	// — a raw x==0 && y==0 check would MISS the Edwards identity and the subsequent
	// ScalarMult(eightInv) would panic on the identity input.
	x, y := p.curve.ScalarMult(p.X(), p.Y(), eight.Bytes())
	tmp := NewECPointNoCurveCheck(p.curve, x, y)
	if tmp.IsIdentity() {
		return tmp
	}
	cleared, err := NewECPoint(p.curve, x, y)
	if err != nil {
		panic(fmt.Errorf("EightInvEight: intermediate point not on curve: %s", err.Error()))
	}
	return cleared.ScalarMult(eightInv)
}

func ScalarBaseMult(curve elliptic.Curve, k *big.Int) *ECPoint {
	x, y := curve.ScalarBaseMult(k.Bytes())
	// [FORK] Restored upstream panic behavior. See ScalarMult comment for rationale.
	p, err := NewECPoint(curve, x, y)
	if err != nil {
		panic(fmt.Errorf("scalar mult to an ecpoint %s", err.Error()))
	}
	return p
}

func isOnCurve(c elliptic.Curve, x, y *big.Int) bool {
	if x == nil || y == nil {
		return false
	}
	return c.IsOnCurve(x, y)
}

// ----- //

func FlattenECPoints(in []*ECPoint) ([]*big.Int, error) {
	if in == nil {
		return nil, errors.New("FlattenECPoints encountered a nil in slice")
	}
	flat := make([]*big.Int, 0, len(in)*2)
	for _, point := range in {
		if point == nil || point.coords[0] == nil || point.coords[1] == nil {
			return nil, errors.New("FlattenECPoints found nil point/coordinate")
		}
		flat = append(flat, point.coords[0])
		flat = append(flat, point.coords[1])
	}
	return flat, nil
}

func UnFlattenECPoints(curve elliptic.Curve, in []*big.Int, noCurveCheck ...bool) ([]*ECPoint, error) {
	if in == nil || len(in)%2 != 0 {
		return nil, errors.New("UnFlattenECPoints expected an in len divisible by 2")
	}
	var err error
	unFlat := make([]*ECPoint, len(in)/2)
	for i, j := 0, 0; i < len(in); i, j = i+2, j+1 {
		if len(noCurveCheck) == 0 || !noCurveCheck[0] {
			unFlat[j], err = NewECPoint(curve, in[i], in[i+1])
			if err != nil {
				return nil, err
			}
		} else {
			unFlat[j] = NewECPointNoCurveCheck(curve, in[i], in[i+1])
		}
	}
	for _, point := range unFlat {
		if point.coords[0] == nil || point.coords[1] == nil {
			return nil, errors.New("UnFlattenECPoints found nil coordinate after unpack")
		}
	}
	return unFlat, nil
}

// ----- //
// Gob helpers for if you choose to encode messages with Gob.

func (p *ECPoint) GobEncode() ([]byte, error) {
	buf := &bytes.Buffer{}
	x, err := p.coords[0].GobEncode()
	if err != nil {
		return nil, err
	}
	y, err := p.coords[1].GobEncode()
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.LittleEndian, uint32(len(x)))
	if err != nil {
		return nil, err
	}
	buf.Write(x)
	err = binary.Write(buf, binary.LittleEndian, uint32(len(y)))
	if err != nil {
		return nil, err
	}
	buf.Write(y)

	return buf.Bytes(), nil
}

func (p *ECPoint) GobDecode(buf []byte) error {
	// [FORK] Upstream has no length bound on decoded coordinates, allowing a crafted
	// payload to allocate arbitrary memory. Cap at 1024 bytes (covers all standard curves).
	const maxCoordLen = 1024
	reader := bytes.NewReader(buf)
	var length uint32
	if err := binary.Read(reader, binary.LittleEndian, &length); err != nil {
		return err
	}
	if length > maxCoordLen {
		return fmt.Errorf("gob decode failed: x coordinate length %d exceeds maximum %d", length, maxCoordLen)
	}
	x := make([]byte, length)
	n, err := reader.Read(x)
	if n != int(length) || err != nil {
		return fmt.Errorf("gob decode failed: %w", err)
	}
	if err := binary.Read(reader, binary.LittleEndian, &length); err != nil {
		return err
	}
	if length > maxCoordLen {
		return fmt.Errorf("gob decode failed: y coordinate length %d exceeds maximum %d", length, maxCoordLen)
	}
	y := make([]byte, length)
	n, err = reader.Read(y)
	if n != int(length) || err != nil {
		return fmt.Errorf("gob decode failed: %w", err)
	}

	X := new(big.Int)
	if err := X.GobDecode(x); err != nil {
		return err
	}
	Y := new(big.Int)
	if err := Y.GobDecode(y); err != nil {
		return err
	}
	p.curve = tss.EC()
	p.coords = [2]*big.Int{X, Y}
	if !p.IsOnCurve() {
		return errors.New("ECPoint.UnmarshalJSON: the point is not on the elliptic curve")
	}
	return nil
}

// ----- //

// crypto.ECPoint is not inherently json marshal-able
func (p *ECPoint) MarshalJSON() ([]byte, error) {
	ecName, ok := tss.GetCurveName(p.curve)
	if !ok {
		return nil, fmt.Errorf("cannot find %T name in curve registry, please call tss.RegisterCurve(name, curve) to register it first", p.curve)
	}

	return json.Marshal(&struct {
		Curve  string
		Coords [2]*big.Int
	}{
		Curve:  string(ecName),
		Coords: p.coords,
	})
}

func (p *ECPoint) UnmarshalJSON(payload []byte) error {
	aux := &struct {
		Curve  string
		Coords [2]*big.Int
	}{}
	if err := json.Unmarshal(payload, &aux); err != nil {
		return err
	}
	p.coords = [2]*big.Int{aux.Coords[0], aux.Coords[1]}

	if len(aux.Curve) > 0 {
		ec, ok := tss.GetCurveByName(tss.CurveName(aux.Curve))
		if !ok {
			return fmt.Errorf("cannot find curve named with %s in curve registry, please call tss.RegisterCurve(name, curve) to register it first", aux.Curve)
		}
		p.curve = ec
	} else {
		// forward compatible, use global ec as default value
		p.curve = tss.EC()
	}

	if !p.IsOnCurve() {
		return fmt.Errorf("ECPoint.UnmarshalJSON: the point is not on the elliptic curve (%T) ", p.curve)
	}

	return nil
}
