// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package mta

import (
	"crypto/elliptic"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/hemilabs/x/tss/v3/common"
	"github.com/hemilabs/x/tss/v3/crypto/paillier"
)

const (
	RangeProofAliceBytesParts = 6
)

type (
	RangeProofAlice struct {
		Z, U, W, S, S1, S2 *big.Int
	}
)

// [FORK] Session parameter added for SSID domain separation (prevents cross-ceremony replay).
// ProveRangeAlice implements Alice's range proof used in the MtA and MtAwc protocols from GG18Spec (9) Fig. 9.
func ProveRangeAlice(Session []byte, ec elliptic.Curve, pk *paillier.PublicKey, c, NTilde, h1, h2, m, r *big.Int, rand io.Reader) (*RangeProofAlice, error) {
	if pk == nil || NTilde == nil || h1 == nil || h2 == nil || c == nil || m == nil || r == nil {
		return nil, errors.New("ProveRangeAlice constructor received nil value(s)")
	}

	q := ec.Params().N
	q3 := new(big.Int).Mul(q, q)
	q3 = new(big.Int).Mul(q, q3)
	qNTilde := new(big.Int).Mul(q, NTilde)
	q3NTilde := new(big.Int).Mul(q3, NTilde)

	// 1.
	alpha := common.GetRandomPositiveInt(rand, q3)
	// 2.
	beta := common.GetRandomPositiveRelativelyPrimeInt(rand, pk.N)

	// 3.
	gamma := common.GetRandomPositiveInt(rand, q3NTilde)

	// 4.
	rho := common.GetRandomPositiveInt(rand, qNTilde)

	// 5.
	modNTilde := common.ModInt(NTilde)
	z := modNTilde.Exp(h1, m)
	z = modNTilde.Mul(z, modNTilde.Exp(h2, rho))

	// 6.
	modNSquared := common.ModInt(pk.NSquare())
	u := modNSquared.Exp(pk.Gamma(), alpha)
	u = modNSquared.Mul(u, modNSquared.Exp(beta, pk.N))

	// 7.
	w := modNTilde.Exp(h1, alpha)
	w = modNTilde.Mul(w, modNTilde.Exp(h2, gamma))

	// 8-9. e'
	var e *big.Int
	{ // must use RejectionSample
		eHash := common.SHA512_256i_TAGGED(Session, append(pk.AsInts(), c, z, u, w)...)
		e = common.RejectionSample(q, eHash)
	}

	modN := common.ModInt(pk.N)
	s := modN.Exp(r, e)
	s = modN.Mul(s, beta)

	// s1 = e * m + alpha
	s1 := new(big.Int).Mul(e, m)
	s1 = new(big.Int).Add(s1, alpha)

	// s2 = e * rho + gamma
	s2 := new(big.Int).Mul(e, rho)
	s2 = new(big.Int).Add(s2, gamma)

	return &RangeProofAlice{Z: z, U: u, W: w, S: s, S1: s1, S2: s2}, nil
}

func RangeProofAliceFromBytes(bzs [][]byte) (*RangeProofAlice, error) {
	if !common.NonEmptyMultiBytes(bzs, RangeProofAliceBytesParts) {
		return nil, fmt.Errorf("expected %d byte parts to construct RangeProofAlice", RangeProofAliceBytesParts)
	}
	return &RangeProofAlice{
		Z:  new(big.Int).SetBytes(bzs[0]),
		U:  new(big.Int).SetBytes(bzs[1]),
		W:  new(big.Int).SetBytes(bzs[2]),
		S:  new(big.Int).SetBytes(bzs[3]),
		S1: new(big.Int).SetBytes(bzs[4]),
		S2: new(big.Int).SetBytes(bzs[5]),
	}, nil
}

func (pf *RangeProofAlice) Verify(Session []byte, ec elliptic.Curve, pk *paillier.PublicKey, NTilde, h1, h2, c *big.Int) bool {
	if pf == nil || !pf.ValidateBasic() || pk == nil || NTilde == nil || h1 == nil || h2 == nil || c == nil {
		return false
	}
	// [FORK] Reject degenerate Pedersen parameters: h1=1 or h2=1 eliminates
	// binding or hiding, making the range proof unsound. Upstream does not check.
	one := big.NewInt(1)
	if h1.Cmp(one) == 0 || h2.Cmp(one) == 0 {
		return false
	}

	// [FORK] NTilde (Pedersen commitment modulus) must be sufficiently large for soundness.
	// Upstream does not check NTilde size in the proof verifier (only at keygen round 2).
	// Defense-in-depth: proof verifiers should be self-contained against untrusted parameters.
	if NTilde.BitLen() < 2048 {
		return false
	}

	// [FORK] Paillier modulus must also be sufficiently large. Upstream does not check
	// pk.N size in the proof verifier. Defense-in-depth: keygen round 2 validates exact
	// 2048 bits, but the proof verifier should not rely on that.
	if pk.N.BitLen() < 2048 {
		return false
	}

	q := ec.Params().N
	q3 := new(big.Int).Mul(q, q)
	q3 = new(big.Int).Mul(q, q3)

	// Interval, coprimality, and degeneracy checks on proof elements (present in both
	// upstream and fork). Without them, a malicious prover can submit out-of-range or
	// degenerate elements that cause modular arithmetic failures or weaken soundness.
	if !common.IsInInterval(pf.Z, NTilde) {
		return false
	}
	if !common.IsInInterval(pf.U, pk.NSquare()) {
		return false
	}
	if !common.IsInInterval(pf.W, NTilde) {
		return false
	}
	if !common.IsInInterval(pf.S, pk.N) {
		return false
	}
	if new(big.Int).GCD(nil, nil, pf.Z, NTilde).Cmp(one) != 0 {
		return false
	}
	if new(big.Int).GCD(nil, nil, pf.U, pk.NSquare()).Cmp(one) != 0 {
		return false
	}
	if new(big.Int).GCD(nil, nil, pf.W, NTilde).Cmp(one) != 0 {
		return false
	}
	if pf.S1.Cmp(q) == -1 {
		return false
	}
	if pf.S2.Cmp(q) == -1 {
		return false
	}
	if pf.S.Cmp(one) == 0 {
		return false
	}
	if pf.Z.Cmp(one) == 0 {
		return false
	}
	if pf.S1.Cmp(pf.S2) == 0 {
		return false
	}

	// 3.
	if pf.S1.Cmp(q3) == 1 {
		return false
	}
	// [FORK] Defense-in-depth: s2 upper bound. Honest s2 = e·rho + gamma where
	// e ∈ [0, q), rho ∈ [1, q·NTilde), gamma ∈ [1, q³·NTilde).
	// Maximum honest value: (q-1)(q·NTilde - 1) + (q³·NTilde - 1)
	//   = q²·NTilde - q·NTilde - q + 1 + q³·NTilde - 1
	//   = q³·NTilde + q²·NTilde - q·NTilde - q
	//   < 2·q³·NTilde (since q² < q³ for q > 1).
	// This bound has EXACTLY ZERO false-rejection probability for honest provers.
	// Without it, a malicious prover could set s2 to an arbitrarily large value,
	// increasing the exponent size in h2^s2 and enabling DoS via expensive modular
	// exponentiation (~2817-bit exponents are the honest maximum on secp256k1).
	// Upstream does not check.
	q3NTilde := new(big.Int).Mul(q3, NTilde)
	s2Bound := new(big.Int).Lsh(q3NTilde, 1) // 2 · q³ · NTilde
	if pf.S2.Cmp(s2Bound) == 1 {
		return false
	}

	// 1-2. e'
	var e *big.Int
	{ // must use RejectionSample
		eHash := common.SHA512_256i_TAGGED(Session, append(pk.AsInts(), c, pf.Z, pf.U, pf.W)...)
		e = common.RejectionSample(q, eHash)
	}

	var products *big.Int // for the following conditionals
	minusE := new(big.Int).Sub(zero, e)

	// [FORK] Defense-in-depth: verify c is coprime with N^2 before negative-exponent
	// computation. A malicious c with gcd(c, N^2) != 1 would cause Exp to
	// return nil (ModInverse fails), triggering a nil-pointer panic. Upstream does not check.
	if new(big.Int).GCD(nil, nil, c, pk.NSquare()).Cmp(one) != 0 {
		return false
	}

	{ // 4. gamma^s_1 * s^N * c^-e
		modNSquared := common.ModInt(pk.NSquare())

		cExpMinusE := modNSquared.Exp(c, minusE)
		sExpN := modNSquared.Exp(pf.S, pk.N)
		gammaExpS1 := modNSquared.Exp(pk.Gamma(), pf.S1)
		// u != (4)
		products = modNSquared.Mul(gammaExpS1, sExpN)
		products = modNSquared.Mul(products, cExpMinusE)
		if pf.U.Cmp(products) != 0 {
			return false
		}
	}

	{ // 5. h_1^s_1 * h_2^s_2 * z^-e
		modNTilde := common.ModInt(NTilde)

		h1ExpS1 := modNTilde.Exp(h1, pf.S1)
		h2ExpS2 := modNTilde.Exp(h2, pf.S2)
		zExpMinusE := modNTilde.Exp(pf.Z, minusE)
		// w != (5)
		products = modNTilde.Mul(h1ExpS1, h2ExpS2)
		products = modNTilde.Mul(products, zExpMinusE)
		if pf.W.Cmp(products) != 0 {
			return false
		}
	}
	return true
}

func (pf *RangeProofAlice) ValidateBasic() bool {
	return pf.Z != nil &&
		pf.U != nil &&
		pf.W != nil &&
		pf.S != nil &&
		pf.S1 != nil &&
		pf.S2 != nil
}

func (pf *RangeProofAlice) Bytes() [RangeProofAliceBytesParts][]byte {
	return [...][]byte{
		pf.Z.Bytes(),
		pf.U.Bytes(),
		pf.W.Bytes(),
		pf.S.Bytes(),
		pf.S1.Bytes(),
		pf.S2.Bytes(),
	}
}
