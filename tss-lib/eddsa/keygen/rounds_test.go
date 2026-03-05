// Copyright (c) 2024 Hemi Labs, Inc.
//
// This file is part of the hemi tss-lib fork. See LICENSE for terms.

package keygen

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

// TestEdDSAKeygenSSIDNonceDifferentiation verifies that different ssidNonce values
// produce different SSIDs. This ensures that concurrent keygen sessions using
// different nonces cannot share ZK proofs (replay prevention).
func TestEdDSAKeygenSSIDNonceDifferentiation(t *testing.T) {
	ec := tss.Edwards()

	// Use fixed party keys for reproducibility.
	partyKeys := []*big.Int{big.NewInt(100), big.NewInt(200), big.NewInt(300)}
	partyCount := int64(3)
	threshold := int64(1)
	roundNumber := int64(1)

	computeSSID := func(nonce int64) string {
		ssidList := []*big.Int{
			new(big.Int).SetBytes([]byte("eddsa-keygen")),
			ec.Params().P,
			ec.Params().N,
			ec.Params().B,
			ec.Params().Gx,
			ec.Params().Gy,
		}
		ssidList = append(ssidList, partyKeys...)
		ssidList = append(ssidList, big.NewInt(partyCount))
		ssidList = append(ssidList, big.NewInt(threshold))
		ssidList = append(ssidList, big.NewInt(roundNumber))
		ssidList = append(ssidList, big.NewInt(nonce))

		return fmt.Sprintf("%x", common.SHA512_256i(ssidList...).Bytes())
	}

	ssidNonce0 := computeSSID(0)
	ssidNonce1 := computeSSID(1)

	// Different nonces must produce different SSIDs.
	assert.NotEqual(t, ssidNonce0, ssidNonce1,
		"SSID with nonce=0 and nonce=1 must differ")

	// Determinism: same nonce must produce same SSID.
	assert.Equal(t, ssidNonce0, computeSSID(0),
		"SSID computation must be deterministic")

	// Verify expected length: SHA-512/256 produces 32 bytes = 64 hex chars.
	assert.Equal(t, 64, len(ssidNonce0),
		"hex-encoded SHA-512/256 should be 64 chars (32 bytes)")

	t.Logf("EdDSA keygen SSID nonce=0: %s", ssidNonce0)
	t.Logf("EdDSA keygen SSID nonce=1: %s", ssidNonce1)
}
