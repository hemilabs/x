// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"math/big"

	cmt "github.com/hemilabs/x/tss/v3/crypto/commitments"
	"github.com/hemilabs/x/tss/v3/crypto/vss"
	"github.com/hemilabs/x/tss/v3/tss"
)

// TaskName identifies the keygen protocol in error messages.
const TaskName = "ecdsa-keygen"

const paillierBitsLen = 2048

type (
	localMessageStore struct {
		kgRound1Messages,
		kgRound2Message1s,
		kgRound2Message2s,
		kgRound3Messages []*tss.Message
	}

	localTempData struct {
		localMessageStore

		// temp data (thrown away after keygen)
		ui            *big.Int // used for tests
		KGCs          []cmt.HashCommitment
		vs            vss.Vs
		ssid          []byte
		ssidNonce     *big.Int
		shares        vss.Shares
		deCommitPolyG cmt.HashDeCommitment
		// [FORK] Store VSS polynomial coefficients for SNARK witness extraction.
		Poly []*big.Int
	}
)
