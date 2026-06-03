// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package common

import (
	"math/big"
)

// RejectionSample converts a hash output to a value in [0, q).
//
// Despite its name, this is modular reduction (hash mod q), not true
// rejection sampling. The statistical distance from uniform is at most
// (2^hashBits mod q) / 2^hashBits. For SHA-512/256 (256-bit output) and
// secp256k1 (q ≈ 2^256), the bias is ≈ 2^-128 — within standard
// Fiat-Shamir security bounds. For larger moduli (e.g., 2048-bit N in
// ModProof), the bias is negligible.
// [FORK] Upstream mutates eHash in-place (eHash.Mod(eHash, q)), which is an
// aliasing bug: callers that retain a reference to eHash see it silently
// modified. We allocate a new big.Int to avoid this side effect.
func RejectionSample(q *big.Int, eHash *big.Int) *big.Int { // e' = eHash
	return new(big.Int).Mod(eHash, q)
}
