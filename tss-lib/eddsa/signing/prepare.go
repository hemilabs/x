// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package signing

import (
	"crypto/elliptic"
	"fmt"
	"math/big"

	"github.com/hemilabs/x/tss-lib/v2/common"
)

// PrepareForSigning(), Fig. 7
func PrepareForSigning(ec elliptic.Curve, i, pax int, xi *big.Int, ks []*big.Int) (wi *big.Int) {
	modQ := common.ModInt(ec.Params().N)
	if len(ks) != pax {
		panic(fmt.Errorf("PrepareForSigning: len(ks) != pax (%d != %d)", len(ks), pax))
	}
	if len(ks) <= i {
		panic(fmt.Errorf("PrepareForSigning: len(ks) <= i (%d <= %d)", len(ks), i))
	}

	// 1-4.
	// [FORK] Explicit copy: upstream used `wi = xi` which aliases the pointer, so the
	// subsequent modular multiplications would mutate the caller's key material in place.
	wi = new(big.Int).Set(xi)
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
		// [FORK] Nil-inverse guard: upstream computed `modQ.ModInverse(Sub(ksj, ksi))` inline
		// without checking the result. If two party keys collide mod q (which should never
		// happen with proper key generation), ModInverse returns nil, causing a nil-pointer
		// dereference in the subsequent Mul. We panic with a descriptive message instead.
		diff := new(big.Int).Sub(ksj, ksi)
		inv := modQ.ModInverse(diff)
		if inv == nil {
			panic(fmt.Errorf("PrepareForSigning: ModInverse(ks[%d]-ks[%d]) is nil; keys may collide mod q", j, i))
		}
		coef := modQ.Mul(ks[j], inv)
		wi = modQ.Mul(wi, coef)
	}

	// [FORK] Defense-in-depth: wi == 0 means this party's secret share contribution
	// is annihilated, which would silently corrupt the threshold signature.
	//
	// This condition is unreachable under normal operation for the following reasons:
	//   1. xi != 0 is validated at keygen (round_3.go) and resharing
	//      (round_4_new_step_2.go:93-96), and VSS Share.Verify() rejects zero shares.
	//   2. ks[j] != 0 mod q is validated by vss.CheckIndexes() during keygen/reshare,
	//      which checks v mod q != 0 for all party indices.
	//   3. Since q is prime, Z/qZ is a field where the product of non-zero elements
	//      is always non-zero. Therefore wi = xi * ∏(ks[j] / (ks[j] - ks[i])) cannot
	//      be zero when all factors are non-zero.
	//
	// Retained as a guard against data corruption of xi or ks values loaded from
	// storage, ensuring a loud failure rather than silent signature corruption.
	if wi.Sign() == 0 {
		panic(fmt.Errorf("PrepareForSigning: wi is zero after Lagrange interpolation for party %d; xi or party keys may be degenerate", i))
	}

	return
}
