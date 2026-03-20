// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package crypto

import (
	"fmt"
	"io"
	"math/big"

	"github.com/hemilabs/x/tss/v3/common"
)

func GenerateNTildei(rand io.Reader, safePrimes [2]*big.Int) (NTildei, h1i, h2i *big.Int, err error) {
	if safePrimes[0] == nil || safePrimes[1] == nil {
		return nil, nil, nil, fmt.Errorf("GenerateNTildei: needs two primes, got %v", safePrimes)
	}
	if !safePrimes[0].ProbablyPrime(30) || !safePrimes[1].ProbablyPrime(30) {
		return nil, nil, nil, fmt.Errorf("GenerateNTildei: expected two primes")
	}
	// [FORK] Upstream does not check for equal primes. If p == q, NTilde = p^2
	// which is trivially factorable, completely breaking Pedersen commitment
	// hiding/binding and all range proofs that rely on the hardness of factoring NTilde.
	if safePrimes[0].Cmp(safePrimes[1]) == 0 {
		return nil, nil, nil, fmt.Errorf("GenerateNTildei: the two primes must be distinct")
	}
	NTildei = new(big.Int).Mul(safePrimes[0], safePrimes[1])
	h1 := common.GetRandomGeneratorOfTheQuadraticResidue(rand, NTildei)
	h2 := common.GetRandomGeneratorOfTheQuadraticResidue(rand, NTildei)
	return NTildei, h1, h2, nil
}
