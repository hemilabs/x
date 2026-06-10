// Copyright (c) 2019-2020 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

// Zero-knowledge proof of knowledge of the discrete logarithm over safe prime product

// A proof of knowledge of the discrete log of an element h2 = hx1 with respect to h1.
// In our protocol, we will run two of these in parallel to prove that two elements h1,h2 generate the same group modN.

package dlnproof

import (
	"fmt"
	"io"
	"math/big"

	"github.com/hemilabs/x/tss/v3/common"
	cmts "github.com/hemilabs/x/tss/v3/crypto/commitments"
)

const Iterations = 128

type (
	Proof struct {
		Alpha,
		T [Iterations]*big.Int
	}
)

var one = big.NewInt(1)

// [FORK] Session parameter added for SSID domain separation (prevents cross-ceremony replay).
func NewDLNProof(Session []byte, h1, h2, x, p, q, N *big.Int, rand io.Reader) *Proof {
	pMulQ := new(big.Int).Mul(p, q)
	modN, modPQ := common.ModInt(N), common.ModInt(pMulQ)
	a := make([]*big.Int, Iterations)
	alpha := [Iterations]*big.Int{}
	for i := range alpha {
		a[i] = common.GetRandomPositiveInt(rand, pMulQ)
		alpha[i] = modN.Exp(h1, a[i])
	}
	msg := append([]*big.Int{h1, h2, N}, alpha[:]...)
	c := common.SHA512_256i_TAGGED(Session, msg...)
	t := [Iterations]*big.Int{}
	cIBI := new(big.Int)
	for i := range t {
		cI := c.Bit(i)
		cIBI = cIBI.SetUint64(uint64(cI))
		t[i] = modPQ.Add(a[i], modPQ.Mul(cIBI, x))
	}
	return &Proof{alpha, t}
}

func (p *Proof) Verify(Session []byte, h1, h2, N *big.Int) bool {
	if p == nil {
		return false
	}
	if N == nil || N.Sign() != 1 {
		return false
	}
	// [FORK] Reject undersized moduli — DLN proofs are only sound when N is the
	// product of two large safe primes (>= 2048-bit modulus). Upstream does not check.
	if N.BitLen() < 2048 {
		return false
	}
	modN := common.ModInt(N)
	h1_ := new(big.Int).Mod(h1, N)
	if h1_.Cmp(one) != 1 || h1_.Cmp(N) != -1 {
		return false
	}
	h2_ := new(big.Int).Mod(h2, N)
	if h2_.Cmp(one) != 1 || h2_.Cmp(N) != -1 {
		return false
	}
	if h1_.Cmp(h2_) == 0 {
		return false
	}
	// [FORK] Upstream validates proof elements with range checks in separate loops and nil
	// checks inside the verification loop. We consolidate all validation (nil checks, bit-length
	// bounds, and range checks) into pre-verification loops to prevent computational DoS via
	// oversized exponents before modular reduction.
	maxBits := N.BitLen() + 2
	for i := range p.T {
		if p.T[i] == nil || p.T[i].BitLen() > maxBits {
			return false
		}
		a := new(big.Int).Mod(p.T[i], N)
		if a.Cmp(one) != 1 || a.Cmp(N) != -1 {
			return false
		}
	}
	for i := range p.Alpha {
		if p.Alpha[i] == nil || p.Alpha[i].BitLen() > maxBits {
			return false
		}
		a := new(big.Int).Mod(p.Alpha[i], N)
		if a.Cmp(one) != 1 || a.Cmp(N) != -1 {
			return false
		}
	}
	// [FORK] Uses SHA512_256i_TAGGED with Session for SSID domain separation.
	// Upstream uses SHA512_256i without any session tag.
	msg := append([]*big.Int{h1, h2, N}, p.Alpha[:]...)
	c := common.SHA512_256i_TAGGED(Session, msg...)
	cIBI := new(big.Int)
	for i := 0; i < Iterations; i++ {
		cI := c.Bit(i)
		cIBI = cIBI.SetUint64(uint64(cI))
		h1ExpTi := modN.Exp(h1, p.T[i])
		h2ExpCi := modN.Exp(h2, cIBI)
		alphaIMulH2ExpCi := modN.Mul(p.Alpha[i], h2ExpCi)
		if h1ExpTi.Cmp(alphaIMulH2ExpCi) != 0 {
			return false
		}
	}
	return true
}

func (p *Proof) Serialize() ([][]byte, error) {
	cb := cmts.NewBuilder()
	cb = cb.AddPart(p.Alpha[:])
	cb = cb.AddPart(p.T[:])
	ints, err := cb.Secrets()
	if err != nil {
		return nil, err
	}
	bzs := make([][]byte, len(ints))
	for i, part := range ints {
		if part == nil {
			bzs[i] = []byte{}
			continue
		}
		bzs[i] = part.Bytes()
	}
	return bzs, nil
}

func UnmarshalDLNProof(bzs [][]byte) (*Proof, error) {
	bis := make([]*big.Int, len(bzs))
	for i := range bis {
		bis[i] = new(big.Int).SetBytes(bzs[i])
	}
	parsed, err := cmts.ParseSecrets(bis)
	if err != nil {
		return nil, err
	}
	if len(parsed) != 2 {
		return nil, fmt.Errorf("UnmarshalDLNProof expected %d parts but got %d", 2, len(parsed))
	}
	pf := new(Proof)
	if len1 := copy(pf.Alpha[:], parsed[0]); len1 != Iterations {
		return nil, fmt.Errorf("UnmarshalDLNProof expected %d but copied %d", Iterations, len1)
	}
	if len2 := copy(pf.T[:], parsed[1]); len2 != Iterations {
		return nil, fmt.Errorf("UnmarshalDLNProof expected %d but copied %d", Iterations, len2)
	}
	return pf, nil
}
