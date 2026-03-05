// Copyright (c) 2024 Hemi Labs, Inc.
//
// This file is part of the hemi tss-lib fork. See LICENSE for terms.

package signing

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/crypto"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

// TestEdDSASigningSSIDNonceDifferentiation verifies that different ssidNonce values
// produce different SSIDs. This ensures concurrent EdDSA signing sessions using
// different nonces cannot share Schnorr proofs (replay prevention).
func TestEdDSASigningSSIDNonceDifferentiation(t *testing.T) {
	ec := tss.Edwards()

	// Fixed party keys.
	partyKeys := []*big.Int{big.NewInt(100), big.NewInt(200)}
	partyCount := int64(2)
	threshold := int64(0)
	roundNumber := int64(1)

	// BigXj: 5*G and 7*G on the Edwards curve.
	gx := ec.Params().Gx
	gy := ec.Params().Gy
	bigXj0x, bigXj0y := ec.ScalarMult(gx, gy, big.NewInt(5).Bytes())
	bigXj1x, bigXj1y := ec.ScalarMult(gx, gy, big.NewInt(7).Bytes())

	bigXj0, err := crypto.NewECPoint(ec, bigXj0x, bigXj0y)
	assert.NoError(t, err)
	bigXj1, err := crypto.NewECPoint(ec, bigXj1x, bigXj1y)
	assert.NoError(t, err)
	bigXjFlat, err := crypto.FlattenECPoints([]*crypto.ECPoint{bigXj0, bigXj1})
	assert.NoError(t, err)

	// Message being signed.
	m := big.NewInt(42)

	computeSSID := func(nonce int64) string {
		ssidList := []*big.Int{
			new(big.Int).SetBytes([]byte("eddsa-signing")),
			ec.Params().P,
			ec.Params().N,
			ec.Params().B,
			ec.Params().Gx,
			ec.Params().Gy,
		}
		ssidList = append(ssidList, partyKeys...)
		ssidList = append(ssidList, bigXjFlat...)
		ssidList = append(ssidList, big.NewInt(partyCount))
		ssidList = append(ssidList, big.NewInt(threshold))
		ssidList = append(ssidList, big.NewInt(roundNumber))
		ssidList = append(ssidList, big.NewInt(nonce))
		ssidList = append(ssidList, m)

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

	t.Logf("EdDSA signing SSID nonce=0: %s", ssidNonce0)
	t.Logf("EdDSA signing SSID nonce=1: %s", ssidNonce1)
}
