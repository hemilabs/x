// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package schnorr

import (
	"errors"
	"io"
	"math/big"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/crypto"
)

type (
	ZKProof struct {
		Alpha *crypto.ECPoint
		T     *big.Int
	}

	ZKVProof struct {
		Alpha *crypto.ECPoint
		T, U  *big.Int
	}
)

// NewZKProof constructs a new Schnorr ZK proof of knowledge of the discrete logarithm (GG18Spec Fig. 16).
// Session provides SSID domain separation (replay prevention across ceremonies).
func NewZKProof(Session []byte, x *big.Int, X *crypto.ECPoint, rand io.Reader) (*ZKProof, error) {
	if x == nil || X == nil || !X.ValidateBasic() {
		return nil, errors.New("ZKProof constructor received nil or invalid value(s)")
	}
	ec := X.Curve()
	ecParams := ec.Params()
	q := ecParams.N
	g := crypto.NewECPointNoCurveCheck(ec, ecParams.Gx, ecParams.Gy) // already on the curve.

	a := common.GetRandomPositiveInt(rand, q)
	alpha := crypto.ScalarBaseMult(ec, a)

	var c *big.Int
	{
		cHash := common.SHA512_256i_TAGGED(Session, X.X(), X.Y(), g.X(), g.Y(), alpha.X(), alpha.Y())
		c = common.RejectionSample(q, cHash)
	}
	t := new(big.Int).Mul(c, x)
	t = common.ModInt(q).Add(a, t)

	return &ZKProof{Alpha: alpha, T: t}, nil
}

// NewZKProof verifies a new Schnorr ZK proof of knowledge of the discrete logarithm (GG18Spec Fig. 16)
func (pf *ZKProof) Verify(Session []byte, X *crypto.ECPoint) bool {
	if pf == nil || !pf.ValidateBasic() || X == nil || !X.ValidateBasic() {
		return false
	}
	ec := X.Curve()
	ecParams := ec.Params()
	q := ecParams.N
	// [FORK] Reject proof scalars outside (0, q) to prevent malleability (T + k*q verifies identically)
	// and to guard against zero scalars that would produce identity points in ScalarBaseMult/ScalarMult.
	// Upstream does not perform this range check.
	if pf.T.Sign() <= 0 || pf.T.Cmp(q) >= 0 {
		return false
	}
	g := crypto.NewECPointNoCurveCheck(ec, ecParams.Gx, ecParams.Gy)

	var c *big.Int
	{
		// SHA512_256i_TAGGED with Session for SSID domain separation.
		cHash := common.SHA512_256i_TAGGED(Session, X.X(), X.Y(), g.X(), g.Y(), pf.Alpha.X(), pf.Alpha.Y())
		c = common.RejectionSample(q, cHash)
	}
	// [FORK] Guard c=0 before ScalarMult to prevent identity-point panic.
	// RejectionSample returns values in [0, q), so c=0 is possible (probability ~2^-256).
	if c.Sign() == 0 {
		return false
	}
	tG := crypto.ScalarBaseMult(ec, pf.T)
	Xc := X.ScalarMult(c)
	// Error handling on Add() (same as upstream): if the result is the identity point,
	// NewECPoint returns (nil, error) and the subsequent .X() call would panic.
	aXc, err := pf.Alpha.Add(Xc)
	if err != nil {
		return false
	}
	return aXc.X().Cmp(tG.X()) == 0 && aXc.Y().Cmp(tG.Y()) == 0
}

func (pf *ZKProof) ValidateBasic() bool {
	return pf.T != nil && pf.Alpha != nil && pf.Alpha.ValidateBasic()
}

// NewZKProof constructs a new Schnorr ZK proof of knowledge s_i, l_i such that V_i = R^s_i, g^l_i (GG18Spec Fig. 17)
func NewZKVProof(Session []byte, V, R *crypto.ECPoint, s, l *big.Int, rand io.Reader) (*ZKVProof, error) {
	if V == nil || R == nil || s == nil || l == nil || !V.ValidateBasic() || !R.ValidateBasic() {
		return nil, errors.New("ZKVProof constructor received nil value(s)")
	}
	ec := V.Curve()
	ecParams := ec.Params()
	q := ecParams.N
	g := crypto.NewECPointNoCurveCheck(ec, ecParams.Gx, ecParams.Gy)

	a, b := common.GetRandomPositiveInt(rand, q), common.GetRandomPositiveInt(rand, q)
	aR := R.ScalarMult(a)
	bG := crypto.ScalarBaseMult(ec, b)
	// [FORK] Upstream discards the error: `alpha, _ := aR.Add(bG)`. If the sum is
	// the identity point, alpha is nil and the subsequent alpha.X() panics.
	alpha, err := aR.Add(bG)
	if err != nil {
		return nil, errors.New("ZKVProof: aR + bG yielded an invalid point")
	}

	var c *big.Int
	{
		cHash := common.SHA512_256i_TAGGED(Session, V.X(), V.Y(), R.X(), R.Y(), g.X(), g.Y(), alpha.X(), alpha.Y())
		c = common.RejectionSample(q, cHash)
	}
	modQ := common.ModInt(q)
	t := modQ.Add(a, new(big.Int).Mul(c, s))
	u := modQ.Add(b, new(big.Int).Mul(c, l))

	return &ZKVProof{Alpha: alpha, T: t, U: u}, nil
}

func (pf *ZKVProof) Verify(Session []byte, V, R *crypto.ECPoint) bool {
	if pf == nil || !pf.ValidateBasic() || V == nil || !V.ValidateBasic() || R == nil || !R.ValidateBasic() {
		return false
	}
	ec := V.Curve()
	ecParams := ec.Params()
	q := ecParams.N
	// [FORK] Reject proof scalars outside (0, q) to prevent malleability and guard against
	// zero scalars that would produce identity points in ScalarMult/ScalarBaseMult.
	// Upstream does not perform these range checks.
	if pf.T.Sign() <= 0 || pf.T.Cmp(q) >= 0 {
		return false
	}
	if pf.U.Sign() <= 0 || pf.U.Cmp(q) >= 0 {
		return false
	}
	g := crypto.NewECPointNoCurveCheck(ec, ecParams.Gx, ecParams.Gy)

	var c *big.Int
	{
		cHash := common.SHA512_256i_TAGGED(Session, V.X(), V.Y(), R.X(), R.Y(), g.X(), g.Y(), pf.Alpha.X(), pf.Alpha.Y())
		c = common.RejectionSample(q, cHash)
	}
	// [FORK] Guard c=0 before ScalarMult to prevent identity-point panic.
	if c.Sign() == 0 {
		return false
	}
	tR := R.ScalarMult(pf.T)
	uG := crypto.ScalarBaseMult(ec, pf.U)
	// [FORK] Upstream discards the error: `tRuG, _ := tR.Add(uG)`. If the sum is
	// the identity point, tRuG is nil and the subsequent .X() call panics.
	tRuG, err := tR.Add(uG)
	if err != nil {
		return false
	}

	Vc := V.ScalarMult(c)
	aVc, err := pf.Alpha.Add(Vc)
	if err != nil {
		return false
	}
	return tRuG.X().Cmp(aVc.X()) == 0 && tRuG.Y().Cmp(aVc.Y()) == 0
}

func (pf *ZKVProof) ValidateBasic() bool {
	return pf.Alpha != nil && pf.T != nil && pf.U != nil && pf.Alpha.ValidateBasic()
}
