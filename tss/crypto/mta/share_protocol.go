// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package mta

import (
	"crypto/elliptic"
	"errors"
	"io"
	"math/big"

	"github.com/hemilabs/x/tss-lib/v3/common"
	"github.com/hemilabs/x/tss-lib/v3/crypto"
	"github.com/hemilabs/x/tss-lib/v3/crypto/paillier"
)

// [FORK] Session parameter added for SSID domain separation (prevents cross-ceremony replay).
// Upstream has no Session parameter; hashes are not ceremony-bound.
func AliceInit(
	Session []byte,
	ec elliptic.Curve,
	pkA *paillier.PublicKey,
	a, NTildeB, h1B, h2B *big.Int,
	rand io.Reader,
) (cA *big.Int, pf *RangeProofAlice, err error) {
	// [FORK] Upstream does not validate parameters. Nil pkA or NTilde causes
	// nil-pointer panics deep in proof construction.
	if ec == nil || pkA == nil || a == nil || NTildeB == nil || h1B == nil || h2B == nil || rand == nil {
		return nil, nil, errors.New("AliceInit received nil argument")
	}
	cA, rA, err := pkA.EncryptAndReturnRandomness(rand, a)
	if err != nil {
		return nil, nil, err
	}
	pf, err = ProveRangeAlice(Session, ec, pkA, cA, NTildeB, h1B, h2B, a, rA, rand)
	return cA, pf, err
}

// [FORK] Split into two session parameters (AliceSession, BobSession) for per-party SSID
// domain separation: Alice's range proof is verified under her session tag, Bob's proof is
// constructed under his. Upstream's AliceInit/ProveRangeAlice has no session parameter at all
// (range proof hash is entirely untagged); only Bob's side has a Session parameter.
func BobMid(
	AliceSession []byte, // Session context Alice used for her range proof (SSID || Alice_index)
	BobSession []byte, // Session context Bob uses for his proof (SSID || Bob_index)
	ec elliptic.Curve,
	pkA *paillier.PublicKey,
	pf *RangeProofAlice,
	b, cA, NTildeA, h1A, h2A, NTildeB, h1B, h2B *big.Int,
	rand io.Reader,
) (beta, cB, betaPrm *big.Int, piB *ProofBob, err error) {
	// [FORK] Nil parameter guard — upstream does not validate, leading to nil-pointer panics.
	if ec == nil || pkA == nil || pf == nil || b == nil || cA == nil || rand == nil {
		err = errors.New("BobMid received nil argument")
		return
	}
	if !pf.Verify(AliceSession, ec, pkA, NTildeB, h1B, h2B, cA) {
		err = errors.New("RangeProofAlice.Verify() returned false")
		return
	}
	q := ec.Params().N
	q5 := new(big.Int).Mul(q, q)  // q^2
	q5 = new(big.Int).Mul(q5, q5) // q^4
	q5 = new(big.Int).Mul(q5, q)  // q^5
	betaPrm = common.GetRandomPositiveInt(rand, q5)
	cBetaPrm, cRand, err := pkA.EncryptAndReturnRandomness(rand, betaPrm)
	if err != nil {
		return
	}
	cB, err = pkA.HomoMult(b, cA)
	if err != nil {
		return
	}
	cB, err = pkA.HomoAdd(cB, cBetaPrm)
	if err != nil {
		return
	}
	beta = common.ModInt(q).Sub(zero, betaPrm)
	piB, err = ProveBob(BobSession, ec, pkA, NTildeA, h1A, h2A, cA, cB, b, betaPrm, cRand, rand)
	return
}

// [FORK] Same per-party session split as BobMid above, plus nil parameter guards.
func BobMidWC(
	AliceSession []byte, // Session context Alice used for her range proof (SSID || Alice_index)
	BobSession []byte, // Session context Bob uses for his proof (SSID || Bob_index)
	ec elliptic.Curve,
	pkA *paillier.PublicKey,
	pf *RangeProofAlice,
	b, cA, NTildeA, h1A, h2A, NTildeB, h1B, h2B *big.Int,
	B *crypto.ECPoint,
	rand io.Reader,
) (beta, cB, betaPrm *big.Int, piB *ProofBobWC, err error) {
	// [FORK] Nil parameter guard — upstream does not validate.
	if ec == nil || pkA == nil || pf == nil || b == nil || cA == nil || B == nil || rand == nil {
		err = errors.New("BobMidWC received nil argument")
		return
	}
	if !pf.Verify(AliceSession, ec, pkA, NTildeB, h1B, h2B, cA) {
		err = errors.New("RangeProofAlice.Verify() returned false")
		return
	}
	q := ec.Params().N
	q5 := new(big.Int).Mul(q, q)  // q^2
	q5 = new(big.Int).Mul(q5, q5) // q^4
	q5 = new(big.Int).Mul(q5, q)  // q^5
	betaPrm = common.GetRandomPositiveInt(rand, q5)
	cBetaPrm, cRand, err := pkA.EncryptAndReturnRandomness(rand, betaPrm)
	if err != nil {
		return
	}
	cB, err = pkA.HomoMult(b, cA)
	if err != nil {
		return
	}
	cB, err = pkA.HomoAdd(cB, cBetaPrm)
	if err != nil {
		return
	}
	beta = common.ModInt(q).Sub(zero, betaPrm)
	piB, err = ProveBobWC(BobSession, ec, pkA, NTildeA, h1A, h2A, cA, cB, b, betaPrm, cRand, B, rand)
	return
}

func AliceEnd(
	Session []byte,
	ec elliptic.Curve,
	pkA *paillier.PublicKey,
	pf *ProofBob,
	h1A, h2A, cA, cB, NTildeA *big.Int,
	sk *paillier.PrivateKey,
) (*big.Int, error) {
	if !pf.Verify(Session, ec, pkA, NTildeA, h1A, h2A, cA, cB) {
		return nil, errors.New("ProofBob.Verify() returned false")
	}
	alphaPrm, err := sk.Decrypt(cB)
	if err != nil {
		return nil, err
	}
	q := ec.Params().N
	return new(big.Int).Mod(alphaPrm, q), nil
}

func AliceEndWC(
	Session []byte,
	ec elliptic.Curve,
	pkA *paillier.PublicKey,
	pf *ProofBobWC,
	B *crypto.ECPoint,
	cA, cB, NTildeA, h1A, h2A *big.Int,
	sk *paillier.PrivateKey,
) (*big.Int, error) {
	if !pf.Verify(Session, ec, pkA, NTildeA, h1A, h2A, cA, cB, B) {
		return nil, errors.New("ProofBobWC.Verify() returned false")
	}
	alphaPrm, err := sk.Decrypt(cB)
	if err != nil {
		return nil, err
	}
	q := ec.Params().N
	return new(big.Int).Mod(alphaPrm, q), nil
}
