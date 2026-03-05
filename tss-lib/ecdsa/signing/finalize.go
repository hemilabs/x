// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package signing

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

func (round *finalization) Start() *tss.Error {
	if round.started {
		return round.WrapError(errors.New("round already started"))
	}
	round.number = 10
	round.started = true
	round.resetOK()

	// [FORK] Defense-in-depth: upstream uses `sumS := round.temp.si` which aliases the
	// pointer. While modN.Add allocates a new big.Int (so the alias is broken after the
	// first iteration), using Set() from the start prevents aliasing hazards if the
	// implementation of modInt.Add ever changes.
	sumS := new(big.Int).Set(round.temp.si)
	modN := common.ModInt(round.Params().EC().Params().N)

	N := round.Params().EC().Params().N
	for j := range round.Parties().IDs() {
		round.ok[j] = true
		if j == round.PartyID().Index {
			continue
		}
		r9msg := round.temp.signRound9Messages[j].Content().(*SignRound9Message)
		sj := r9msg.UnmarshalS()
		// [FORK] Range check on each party's s_j share. Upstream accepts any value
		// from UnmarshalS(). A malicious party could send >= N values to manipulate
		// the aggregated signature or cause undefined modular arithmetic.
		// Defense-in-depth: sj.Sign()<0 is unreachable because UnmarshalS() uses
		// SetBytes() which always produces non-negative values. Retained alongside
		// the Cmp(N) check for completeness — the range check [0, N) is the meaningful
		// validation.
		if sj.Sign() < 0 || sj.Cmp(N) >= 0 {
			return round.WrapError(fmt.Errorf("party %d sent s_i outside [0, N)", j),
				round.Parties().IDs()[j])
		}
		sumS = modN.Add(sumS, sj)
	}

	// [FORK] Zero-S rejection. Upstream does not check. A colluding set of malicious
	// parties could craft their s_j values to force sumS = 0 mod N, producing an
	// invalid ECDSA signature (s=0 is explicitly forbidden by the spec).
	if sumS.Sign() == 0 {
		return round.WrapError(errors.New("accumulated S is zero: malicious share detected"))
	}

	recid := 0
	// byte v = if(R.X > curve.N) then 2 else 0) | (if R.Y.IsEven then 0 else 1);
	if round.temp.rx.Cmp(round.Params().EC().Params().N) > 0 {
		recid = 2
	}
	if round.temp.ry.Bit(0) != 0 {
		recid |= 1
	}

	// This is copied from:
	// https://github.com/btcsuite/btcd/blob/c26ffa870fd817666a857af1bf6498fabba1ffe3/btcec/signature.go#L442-L444
	// This is needed because of tendermint checks here:
	// https://github.com/tendermint/tendermint/blob/d9481e3648450cb99e15c6a070c1fb69aa0c255b/crypto/secp256k1/secp256k1_nocgo.go#L43-L47
	secp256k1halfN := new(big.Int).Rsh(round.Params().EC().Params().N, 1)
	if sumS.Cmp(secp256k1halfN) > 0 {
		sumS.Sub(round.Params().EC().Params().N, sumS)
		recid ^= 1
	}

	// save the signature for final output
	// [FORK] Ceiling division: upstream uses `BitSize / 8` which truncates for curves
	// whose bit size is not a multiple of 8 (e.g. P-521 = 521 bits -> 65 instead of 66).
	// Latent on secp256k1 (256/8 = 32 exact), but a real bug for non-standard curves.
	bitSizeInBytes := (round.Params().EC().Params().BitSize + 7) / 8
	round.data.R = padToLengthBytesInPlace(round.temp.rx.Bytes(), bitSizeInBytes)
	round.data.S = padToLengthBytesInPlace(sumS.Bytes(), bitSizeInBytes)
	round.data.Signature = append(round.data.R, round.data.S...)
	round.data.SignatureRecovery = []byte{byte(recid)}
	if round.temp.fullBytesLen == 0 {
		round.data.M = round.temp.m.Bytes()
	} else {
		var mBytes = make([]byte, round.temp.fullBytesLen)
		round.temp.m.FillBytes(mBytes)
		round.data.M = mBytes
	}

	pk := ecdsa.PublicKey{
		Curve: round.Params().EC(),
		X:     round.key.ECDSAPub.X(),
		Y:     round.key.ECDSAPub.Y(),
	}

	ok := ecdsa.Verify(&pk, round.data.M, round.temp.rx, sumS)
	if !ok {
		return round.WrapError(fmt.Errorf("signature verification failed"))
	}

	round.end <- round.data

	return nil
}

func (round *finalization) CanAccept(msg tss.ParsedMessage) bool {
	// not expecting any incoming messages in this round
	return false
}

func (round *finalization) Update() (bool, *tss.Error) {
	// not expecting any incoming messages in this round
	return false, nil
}

func (round *finalization) NextRound() tss.Round {
	return nil // finished!
}

func padToLengthBytesInPlace(src []byte, length int) []byte {
	oriLen := len(src)
	if oriLen < length {
		for i := 0; i < length-oriLen; i++ {
			src = append([]byte{0}, src...)
		}
	}
	return src
}
