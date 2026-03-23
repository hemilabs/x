// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

// partly ported from:
// https://github.com/KZen-networks/curv/blob/78a70f43f5eda376e5888ce33aec18962f572bbe/src/cryptographic_primitives/commitments/hash_commitment.rs

package commitments

import (
	"io"
	"math/big"

	"github.com/hemilabs/x/tss-lib/v3/common"
)

const (
	HashLength = 256
)

type (
	HashCommitment   = *big.Int
	HashDeCommitment = []*big.Int

	HashCommitDecommit struct {
		C HashCommitment
		D HashDeCommitment
	}
)

func NewHashCommitmentWithRandomness(r *big.Int, secrets ...*big.Int) *HashCommitDecommit {
	parts := make([]*big.Int, len(secrets)+1)
	parts[0] = r
	for i := 1; i < len(parts); i++ {
		parts[i] = secrets[i-1]
	}
	hash := common.SHA512_256i(parts...)

	cmt := &HashCommitDecommit{}
	cmt.C = hash
	cmt.D = parts
	return cmt
}

func NewHashCommitment(rand io.Reader, secrets ...*big.Int) *HashCommitDecommit {
	r := common.MustGetRandomInt(rand, HashLength) // r
	return NewHashCommitmentWithRandomness(r, secrets...)
}

func NewHashDeCommitmentFromBytes(marshalled [][]byte) HashDeCommitment {
	return common.MultiBytesToBigInts(marshalled)
}

func (cmt *HashCommitDecommit) Verify() bool {
	C, D := cmt.C, cmt.D
	// [FORK] Upstream only checks `C == nil || D == nil`. Added len(D) < 2 check:
	// D must contain at least the randomness element plus one secret. Without this,
	// a decommitment with only the randomness (no secret) could pass verification.
	if C == nil || D == nil || len(D) < 2 {
		return false
	}
	hash := common.SHA512_256i(D...)
	return hash.Cmp(C) == 0
}

func (cmt *HashCommitDecommit) DeCommit() (bool, HashDeCommitment) {
	if cmt.Verify() {
		// [1:] skips random element r in D
		return true, cmt.D[1:]
	} else {
		return false, nil
	}
}
