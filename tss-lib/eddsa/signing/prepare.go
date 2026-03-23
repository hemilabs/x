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
)

// PrepareForSigning computes the Lagrange interpolated secret share
// wi for party i, given the party's secret xi and all party keys ks.
func PrepareForSigning(ec elliptic.Curve, i, pax int, xi *big.Int, ks []*big.Int) (wi *big.Int) {
	modQ := common.ModInt(ec.Params().N)
	if len(ks) != pax {
		panic(fmt.Errorf("PrepareForSigning: len(ks) != pax (%d != %d)", len(ks), pax))
	}
	if len(ks) <= i {
		panic(fmt.Errorf("PrepareForSigning: len(ks) <= i (%d <= %d)", len(ks), i))
	}
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
		diff := new(big.Int).Sub(ksj, ksi)
		inv := modQ.ModInverse(diff)
		if inv == nil {
			panic(fmt.Errorf("PrepareForSigning: ModInverse(ks[%d]-ks[%d]) is nil; keys may collide mod q", j, i))
		}
		coef := modQ.Mul(ks[j], inv)
		wi = modQ.Mul(wi, coef)
	}
	if wi.Sign() == 0 {
		panic(fmt.Errorf("PrepareForSigning: wi is zero after Lagrange interpolation for party %d", i))
	}
	return
}
