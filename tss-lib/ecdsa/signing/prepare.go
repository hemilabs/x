// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package signing

import (
	"crypto/elliptic"
	"fmt"
	"math/big"

	"github.com/hemilabs/x/tss-lib/v3/common"
	"github.com/hemilabs/x/tss-lib/v3/crypto"
)

// PrepareForSigning(), GG18Spec (11) Fig. 14
func PrepareForSigning(ec elliptic.Curve, i, pax int, xi *big.Int, ks []*big.Int, bigXs []*crypto.ECPoint) (wi *big.Int, bigWs []*crypto.ECPoint) {
	modQ := common.ModInt(ec.Params().N)
	// Precondition panics (same as upstream). The [FORK] changes in this file are the
	// Set() copy on line 36 and the nil-checks on ModInverse below.
	if len(ks) != len(bigXs) {
		panic(fmt.Errorf("PrepareForSigning: len(ks) != len(bigXs) (%d != %d)", len(ks), len(bigXs)))
	}
	if len(ks) != pax {
		panic(fmt.Errorf("PrepareForSigning: len(ks) != pax (%d != %d)", len(ks), pax))
	}
	if len(ks) <= i {
		panic(fmt.Errorf("PrepareForSigning: len(ks) <= i (%d <= %d)", len(ks), i))
	}

	// 2-4.
	// [FORK] Upstream: `wi = xi` aliases the pointer, so subsequent Mul() calls
	// mutate the caller's xi in-place, corrupting the key share. Use Set() to copy.
	wi = new(big.Int).Set(xi) // explicit copy to avoid mutating key material
	for j := 0; j < pax; j++ {
		if j == i {
			continue
		}
		ksj := ks[j]
		ksi := ks[i]
		if ksj.Cmp(ksi) == 0 {
			panic(fmt.Errorf("index of two parties are equal"))
		}
		// big.Int Div is calculated as: a/b = a * modInv(b,q)
		diff := new(big.Int).Sub(ksj, ksi)
		inv := modQ.ModInverse(diff)
		// [FORK] Nil-check on ModInverse: upstream does not check. If two party keys
		// collide mod q, ModInverse returns nil, causing a nil-pointer panic in Mul().
		if inv == nil {
			panic(fmt.Errorf("PrepareForSigning: ModInverse(ks[%d]-ks[%d]) is nil; keys may collide mod q", j, i))
		}
		coef := modQ.Mul(ks[j], inv)
		wi = modQ.Mul(wi, coef)
	}

	// [FORK] Defense-in-depth: wi == 0 means this party's secret share contribution
	// is annihilated, which would silently corrupt the threshold signature (zero
	// signature share propagates through MtA, producing k*w = 0 regardless of nonce).
	//
	// This condition is unreachable under normal operation for the following reasons:
	//   1. xi != 0 is validated at keygen (round_3.go:48-50) and resharing
	//      (round_4_new_step_2.go:257-259), and VSS Share.Verify() rejects zero shares.
	//   2. ks[j] != 0 mod q is validated by vss.CheckIndexes() during keygen/reshare,
	//      which checks v mod q != 0 for all party indices.
	//   3. Since q is prime, Z/qZ is a field where the product of non-zero elements
	//      is always non-zero. Therefore wi = xi * ∏(ks[j] / (ks[j] - ks[i])) cannot
	//      be zero when all factors are non-zero.
	//
	// The only theoretical (negligible-probability) path is via BIP-32 key derivation:
	// if keyDerivationDelta == -xi mod q (probability 1/q ≈ 2^{-256}), the derived xi
	// becomes zero. This check catches that edge case as well as any data corruption
	// of xi or ks values loaded from storage.
	if wi.Sign() == 0 {
		panic(fmt.Errorf("PrepareForSigning: wi is zero after Lagrange interpolation for party %d; xi or party keys may be degenerate", i))
	}

	// 5-10.
	bigWs = make([]*crypto.ECPoint, len(ks))
	for j := 0; j < pax; j++ {
		bigWj := bigXs[j]
		for c := 0; c < pax; c++ {
			if j == c {
				continue
			}
			ksc := ks[c]
			ksj := ks[j]
			if ksj.Cmp(ksc) == 0 {
				panic(fmt.Errorf("index of two parties are equal"))
			}
			// big.Int Div is calculated as: a/b = a * modInv(b,q)
			diff := new(big.Int).Sub(ksc, ksj)
			inv := modQ.ModInverse(diff)
			// [FORK] Same nil-check as above for the BigXj Lagrange interpolation loop.
			if inv == nil {
				panic(fmt.Errorf("PrepareForSigning: ModInverse(ks[%d]-ks[%d]) is nil; keys may collide mod q", c, j))
			}
			iotaVal := modQ.Mul(ksc, inv)
			bigWj = bigWj.ScalarMult(iotaVal)
		}
		bigWs[j] = bigWj
	}
	return
}
