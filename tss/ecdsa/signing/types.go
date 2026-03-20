// Copyright © 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.
package signing

import (
	"math/big"

	"github.com/hemilabs/x/tss/v3/crypto"
	cmt "github.com/hemilabs/x/tss/v3/crypto/commitments"
	"github.com/hemilabs/x/tss/v3/crypto/mta"
	"github.com/hemilabs/x/tss/v3/tss"
)

// TaskName identifies the signing protocol in error messages.
const TaskName = "signing"

type (
	localMessageStore struct {
		signRound1Message1s,
		signRound1Message2s,
		signRound2Messages,
		signRound3Messages,
		signRound4Messages,
		signRound5Messages,
		signRound6Messages,
		signRound7Messages,
		signRound8Messages,
		signRound9Messages []*tss.Message
	}

	localTempData struct {
		localMessageStore

		// temp data (thrown away after sign) / round 1
		w,
		m,
		k,
		theta,
		thetaInverse,
		sigma,
		keyDerivationDelta,
		gamma *big.Int
		fullBytesLen int
		cis          []*big.Int
		bigWs        []*crypto.ECPoint
		pointGamma   *crypto.ECPoint
		deCommit     cmt.HashDeCommitment

		// round 2
		betas, // return value of Bob_mid
		c1jis,
		c2jis,
		vs []*big.Int // return value of Bob_mid_wc
		pi1jis []*mta.ProofBob
		pi2jis []*mta.ProofBobWC

		// round 5
		li,
		si,
		rx,
		ry,
		roi *big.Int
		bigR,
		bigAi,
		bigVi *crypto.ECPoint
		DPower cmt.HashDeCommitment

		// round 7
		Ui,
		Ti *crypto.ECPoint
		DTelda cmt.HashDeCommitment

		ssidNonce *big.Int
		ssid      []byte
	}
)

func padToLengthBytesInPlace(src []byte, length int) []byte {
	oriLen := len(src)
	if oriLen < length {
		for i := 0; i < length-oriLen; i++ {
			src = append([]byte{0}, src...)
		}
	}
	return src
}
