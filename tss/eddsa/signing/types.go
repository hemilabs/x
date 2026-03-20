// Copyright © 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package signing

import (
	"math/big"

	"github.com/hemilabs/x/tss/v3/eddsa/keygen"
	"github.com/hemilabs/x/tss/v3/tss"
)

// TaskName identifies the EdDSA signing protocol in error messages.
const TaskName = "eddsa-signing"

type localTempData struct {
	localMessageStore

	ssidNonce *big.Int
	ssid      []byte

	// round 1
	wi       *big.Int
	ri       *big.Int
	pointRi  interface{} // *crypto.ECPoint
	deCommit []*big.Int
	cjs      []*big.Int

	// round 3
	si           *[32]byte
	r            *big.Int
	m            *big.Int
	fullBytesLen int
}

type localMessageStore struct {
	signRound1Messages []*tss.Message
	signRound2Messages []*tss.Message
	signRound3Messages []*tss.Message
}

// SignatureData holds the final EdDSA signature components.
type SignatureData struct {
	Signature []byte // 64-byte R||S
	R         []byte
	S         []byte
	M         []byte
}

// SigningState holds mutable state across all signing rounds.
type SigningState struct {
	params *tss.Parameters
	key    *keygen.LocalPartySaveData
	data   *SignatureData
	temp   localTempData
}

// SignRoundOutput is returned by each round function.
type SignRoundOutput struct {
	Messages  []*tss.Message
	Signature *SignatureData
}
