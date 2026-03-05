// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

// Feldman VSS, based on Paul Feldman, 1987., A practical scheme for non-interactive verifiable secret sharing.
// In Foundations of Computer Science, 1987., 28th Annual Symposium on. IEEE, 427–43
//

package vss

import (
	"crypto/elliptic"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/crypto"
)

type (
	Share struct {
		Threshold int
		ID,       // xi
		Share *big.Int // Sigma i
	}

	Vs []*crypto.ECPoint // v0..vt

	Shares []*Share
)

var (
	ErrNumSharesBelowThreshold = fmt.Errorf("not enough shares to satisfy the threshold")

	zero = big.NewInt(0)
	one  = big.NewInt(1)
)

// Check share ids of Shamir's Secret Sharing, return error if duplicate or 0 value found
func CheckIndexes(ec elliptic.Curve, indexes []*big.Int) ([]*big.Int, error) {
	visited := make(map[string]struct{})
	for _, v := range indexes {
		vMod := new(big.Int).Mod(v, ec.Params().N)
		if vMod.Cmp(zero) == 0 {
			return nil, errors.New("party index should not be 0")
		}
		vModStr := vMod.String()
		if _, ok := visited[vModStr]; ok {
			return nil, fmt.Errorf("duplicate indexes %s", vModStr)
		}
		visited[vModStr] = struct{}{}
	}
	return indexes, nil
}

// Returns a new array of secret shares created by Shamir's Secret Sharing Algorithm,
// requiring a minimum number of shares to recreate, of length shares, from the input secret.
//
// [FORK] Returns the polynomial coefficients as the third return value. Upstream returns
// only (Vs, Shares, error). The polynomial is needed for the per-participant SNARK
// architecture where each operator's SP1 guest must evaluate the polynomial independently.
func Create(ec elliptic.Curve, threshold int, secret *big.Int, indexes []*big.Int, rand io.Reader) (Vs, Shares, []*big.Int, error) {
	if secret == nil || indexes == nil {
		return nil, nil, nil, fmt.Errorf("vss secret or indexes == nil: secret=%t indexes=%t", secret != nil, indexes != nil)
	}
	// [FORK] Reject zero secret: ScalarBaseMult(0) produces the identity point, which
	// panics. A zero secret also means the shared key is trivially known (= 0).
	if secret.Sign() == 0 {
		return nil, nil, nil, errors.New("vss secret must be non-zero")
	}
	if threshold < 1 {
		return nil, nil, nil, errors.New("vss threshold < 1")
	}

	ids, err := CheckIndexes(ec, indexes)
	if err != nil {
		return nil, nil, nil, err
	}

	num := len(indexes)
	if num < threshold {
		return nil, nil, nil, ErrNumSharesBelowThreshold
	}

	poly := samplePolynomial(ec, threshold, secret, rand)

	v := make(Vs, len(poly))
	for i, ai := range poly {
		v[i] = crypto.ScalarBaseMult(ec, ai)
	}

	shares := make(Shares, num)
	for i := 0; i < num; i++ {
		share := evaluatePolynomial(ec, threshold, poly, ids[i])
		shares[i] = &Share{Threshold: threshold, ID: ids[i], Share: share}
	}
	return v, shares, poly, nil
}

func (share *Share) Verify(ec elliptic.Curve, threshold int, vs Vs) bool {
	if share.Threshold != threshold || vs == nil || len(vs) != threshold+1 {
		return false
	}
	// [FORK] Reject shares that are zero or out of range [1, q-1].
	// Upstream does not validate share values, allowing zero shares (which map to the
	// identity point under ScalarBaseMult) or out-of-range values (>= q) that indicate
	// malformed or tampered data.
	q := ec.Params().N
	if share.Share == nil || share.Share.Sign() <= 0 || share.Share.Cmp(q) >= 0 {
		return false
	}
	// [FORK] Reject share ID that is nil or zero mod q — evaluation at x=0 leaks the
	// secret (constant term of the polynomial). Upstream does not check.
	if share.ID == nil || new(big.Int).Mod(share.ID, q).Sign() == 0 {
		return false
	}
	var err error
	modQ := common.ModInt(ec.Params().N)
	v, t := vs[0], one // YRO : we need to have our accumulator outside of the loop
	for j := 1; j <= threshold; j++ {
		// t = k_i^j
		t = modQ.Mul(t, share.ID)
		// v = v * v_j^t
		vjt := vs[j].SetCurve(ec).ScalarMult(t)
		v, err = v.SetCurve(ec).Add(vjt)
		if err != nil {
			return false
		}
	}
	sigmaGi := crypto.ScalarBaseMult(ec, share.Share)
	return sigmaGi.Equals(v)
}

func (shares Shares) ReConstruct(ec elliptic.Curve) (secret *big.Int, err error) {
	if shares != nil && shares[0].Threshold > len(shares) {
		return nil, ErrNumSharesBelowThreshold
	}
	modN := common.ModInt(ec.Params().N)

	// [FORK] Check for duplicate share IDs (reduced mod q) to prevent silently wrong
	// Lagrange interpolation. Upstream does not check — duplicate IDs cause division
	// by zero in ModInverse. Reduction mod q is necessary because distinct integers
	// that are congruent mod q (e.g., k and k+q) produce a zero denominator in the
	// Lagrange basis computation. This mirrors the approach in CheckIndexes.
	q := ec.Params().N
	xs := make([]*big.Int, 0, len(shares))
	seen := make(map[string]struct{}, len(shares))
	for _, share := range shares {
		idMod := new(big.Int).Mod(share.ID, q)
		idStr := idMod.String()
		if _, dup := seen[idStr]; dup {
			return nil, fmt.Errorf("duplicate share ID %s (mod q) in ReConstruct", idStr)
		}
		seen[idStr] = struct{}{}
		xs = append(xs, share.ID)
	}

	secret = new(big.Int)
	for i, share := range shares {
		times := new(big.Int).SetInt64(1)
		for j := 0; j < len(xs); j++ {
			if j == i {
				continue
			}
			sub := modN.Sub(xs[j], share.ID)
			subInv := modN.ModInverse(sub)
			// [FORK] Upstream does not check for nil ModInverse. If share IDs collide
			// mod q, ModInverse returns nil causing a nil-pointer panic.
			// Defense-in-depth: the mod-q duplicate check above now catches this
			// condition, but this nil guard is retained as a safeguard.
			if subInv == nil {
				return nil, errors.New("ModInverse(xs[j] - share.ID) returned nil; share IDs may collide modulo the curve order")
			}
			div := modN.Mul(xs[j], subInv)
			times = modN.Mul(times, div)
		}

		fTimes := modN.Mul(share.Share, times)
		secret = modN.Add(secret, fTimes)
	}

	return secret, nil
}

func samplePolynomial(ec elliptic.Curve, threshold int, secret *big.Int, rand io.Reader) []*big.Int {
	q := ec.Params().N
	v := make([]*big.Int, threshold+1)
	v[0] = secret
	for i := 1; i <= threshold; i++ {
		ai := common.GetRandomPositiveInt(rand, q)
		v[i] = ai
	}
	return v
}

// Evauluates a polynomial with coefficients such that:
// evaluatePolynomial([a, b, c, d], x):
//
//	returns a + bx + cx^2 + dx^3
func evaluatePolynomial(ec elliptic.Curve, threshold int, v []*big.Int, id *big.Int) (result *big.Int) {
	q := ec.Params().N
	modQ := common.ModInt(q)
	result = new(big.Int).Set(v[0])
	X := big.NewInt(int64(1))
	for i := 1; i <= threshold; i++ {
		ai := v[i]
		X = modQ.Mul(X, id)
		aiXi := new(big.Int).Mul(ai, X)
		result = modQ.Add(result, aiXi)
	}
	return
}
