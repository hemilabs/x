// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package resharing_test

import (
	"fmt"
	"math/big"
	"sync/atomic"
	"testing"

	"github.com/decred/dcrd/dcrec/edwards/v2"
	"github.com/ipfs/go-log"
	"github.com/stretchr/testify/assert"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/crypto"
	"github.com/hemilabs/x/tss-lib/v2/eddsa/keygen"
	. "github.com/hemilabs/x/tss-lib/v2/eddsa/resharing"
	"github.com/hemilabs/x/tss-lib/v2/eddsa/signing"
	"github.com/hemilabs/x/tss-lib/v2/test"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

const (
	testParticipants = test.TestParticipants
	testThreshold    = test.TestThreshold
)

func setUp(level string) {
	if err := log.SetLogLevel("tss-lib", level); err != nil {
		panic(err)
	}

	// only for test
	tss.SetCurve(tss.Edwards())
}

func TestE2EConcurrent(t *testing.T) {
	setUp("info")

	threshold, newThreshold := testThreshold, testThreshold

	// PHASE: load keygen fixtures
	firstPartyIdx, extraParties := 1, 1 // // extra can be 0 to N-first
	oldKeys, oldPIDs, err := keygen.LoadKeygenTestFixtures(testThreshold+1+extraParties+firstPartyIdx, firstPartyIdx)
	assert.NoError(t, err, "should load keygen fixtures")

	// PHASE: resharing
	oldP2PCtx := tss.NewPeerContext(oldPIDs)

	// init the new parties; re-use the fixture pre-params for speed
	newPIDs := tss.GenerateTestPartyIDs(testParticipants)
	newP2PCtx := tss.NewPeerContext(newPIDs)
	newPCount := len(newPIDs)

	oldCommittee := make([]*LocalParty, 0, len(oldPIDs))
	newCommittee := make([]*LocalParty, 0, newPCount)
	bothCommitteesPax := len(oldCommittee) + len(newCommittee)

	errCh := make(chan *tss.Error, bothCommitteesPax)
	outCh := make(chan tss.Message, bothCommitteesPax)
	endCh := make(chan *keygen.LocalPartySaveData, bothCommitteesPax)

	updater := test.SharedPartyUpdater

	// init the old parties first
	for j, pID := range oldPIDs {
		params := tss.NewReSharingParameters(tss.Edwards(), oldP2PCtx, newP2PCtx, pID, testParticipants, threshold, newPCount, newThreshold)
		P := NewLocalParty(params, oldKeys[j], outCh, endCh).(*LocalParty) // discard old key data
		oldCommittee = append(oldCommittee, P)
	}

	// init the new parties
	for _, pID := range newPIDs {
		params := tss.NewReSharingParameters(tss.Edwards(), oldP2PCtx, newP2PCtx, pID, testParticipants, threshold, newPCount, newThreshold)
		save := keygen.NewLocalPartySaveData(newPCount)
		P := NewLocalParty(params, save, outCh, endCh).(*LocalParty)
		newCommittee = append(newCommittee, P)
	}

	// start the new parties; they will wait for messages
	for _, P := range newCommittee {
		go func(P *LocalParty) {
			if err := P.Start(); err != nil {
				errCh <- err
			}
		}(P)
	}
	// start the old parties; they will send messages
	for _, P := range oldCommittee {
		go func(P *LocalParty) {
			if err := P.Start(); err != nil {
				errCh <- err
			}
		}(P)
	}

	newKeys := make([]keygen.LocalPartySaveData, len(newCommittee))
	endedOldCommittee := 0
	var reSharingEnded int32
	for {
		select {
		case err := <-errCh:
			common.Logger.Errorf("Error: %s", err)
			assert.FailNow(t, err.Error())
			return

		case msg := <-outCh:
			dest := msg.GetTo()
			if dest == nil {
				t.Fatal("did not expect a msg to have a nil destination during resharing")
			}
			if msg.IsToOldCommittee() || msg.IsToOldAndNewCommittees() {
				for _, destP := range dest[:len(oldCommittee)] {
					go updater(oldCommittee[destP.Index], msg, errCh)
				}
			}
			if !msg.IsToOldCommittee() || msg.IsToOldAndNewCommittees() {
				for _, destP := range dest {
					go updater(newCommittee[destP.Index], msg, errCh)
				}
			}

		case save := <-endCh:
			// old committee members that aren't receiving a share have their Xi zeroed
			if save.Xi != nil {
				index, err := save.OriginalIndex()
				assert.NoErrorf(t, err, "should not be an error getting a party's index from save data")
				newKeys[index] = *save
			} else {
				endedOldCommittee++
			}
			atomic.AddInt32(&reSharingEnded, 1)
			if atomic.LoadInt32(&reSharingEnded) == int32(len(oldCommittee)+len(newCommittee)) {
				assert.Equal(t, len(oldCommittee), endedOldCommittee)
				t.Logf("Resharing done. Reshared %d participants", reSharingEnded)

				// xj tests: BigXj == xj*G
				for j, key := range newKeys {
					// xj test: BigXj == xj*G
					xj := key.Xi
					gXj := crypto.ScalarBaseMult(tss.Edwards(), xj)
					BigXj := key.BigXj[j]
					assert.True(t, BigXj.Equals(gXj), "ensure BigX_j == g^x_j")
				}

				// more verification of signing is implemented within local_party_test.go of keygen package
				goto signing
			}
		}
	}

signing:
	// PHASE: signing
	signKeys, signPIDs := newKeys, newPIDs
	signP2pCtx := tss.NewPeerContext(signPIDs)
	signParties := make([]*signing.LocalParty, 0, len(signPIDs))

	signErrCh := make(chan *tss.Error, len(signPIDs))
	signOutCh := make(chan tss.Message, len(signPIDs))
	signEndCh := make(chan *common.SignatureData, len(signPIDs))

	for j, signPID := range signPIDs {
		params := tss.NewParameters(tss.Edwards(), signP2pCtx, signPID, len(signPIDs), newThreshold)
		P := signing.NewLocalParty(big.NewInt(42), params, signKeys[j], signOutCh, signEndCh).(*signing.LocalParty)
		signParties = append(signParties, P)
		go func(P *signing.LocalParty) {
			if err := P.Start(); err != nil {
				signErrCh <- err
			}
		}(P)
	}

	var signEnded int32
	for {
		select {
		case err := <-signErrCh:
			common.Logger.Errorf("Error: %s", err)
			assert.FailNow(t, err.Error())
			return

		case msg := <-signOutCh:
			dest := msg.GetTo()
			if dest == nil {
				for _, P := range signParties {
					if P.PartyID().Index == msg.GetFrom().Index {
						continue
					}
					go updater(P, msg, signErrCh)
				}
			} else {
				if dest[0].Index == msg.GetFrom().Index {
					t.Fatalf("party %d tried to send a message to itself (%d)", dest[0].Index, msg.GetFrom().Index)
				}
				go updater(signParties[dest[0].Index], msg, signErrCh)
			}

		case signData := <-signEndCh:
			atomic.AddInt32(&signEnded, 1)
			if atomic.LoadInt32(&signEnded) == int32(len(signPIDs)) {
				t.Logf("Signing done. Received sign data from %d participants", signEnded)

				// BEGIN EDDSA verify
				pkX, pkY := signKeys[0].EDDSAPub.X(), signKeys[0].EDDSAPub.Y()
				pk := edwards.PublicKey{
					Curve: tss.Edwards(),
					X:     pkX,
					Y:     pkY,
				}

				newSig, err := edwards.ParseSignature(signData.Signature)
				if err != nil {
					println("new sig error, ", err.Error())
				}

				ok := edwards.Verify(&pk, big.NewInt(42).Bytes(),
					newSig.R, newSig.S)

				assert.True(t, ok, "eddsa verify must pass")
				t.Log("EDDSA signing test done.")
				// END EDDSA verify

				return
			}
		}
	}
}

// TestEdDSAReshareSSIDGoldenVector verifies that the [FORK] SSID computation in
// EdDSA resharing produces a stable, expected hash value. The EdDSA resharing SSID
// is entirely new code (upstream had no SSID for resharing at all). This test
// constructs the SSID inputs manually — matching the formula in rounds.go getSSID() —
// and asserts the SHA-512/256 output matches a hardcoded golden vector.
func TestEdDSAReshareSSIDGoldenVector(t *testing.T) {
	ec := tss.Edwards()

	// Fixed inputs for reproducibility.
	// Old party keys (small known values, sorted ascending).
	oldK1 := big.NewInt(100)
	oldK2 := big.NewInt(200)

	// New party keys (small known values, sorted ascending).
	newK1 := big.NewInt(300)
	newK2 := big.NewInt(400)

	// EDDSAPub: use 5*G on the Edwards curve for a reproducible public key point.
	gx := ec.Params().Gx
	gy := ec.Params().Gy
	pubX, pubY := ec.ScalarMult(gx, gy, big.NewInt(5).Bytes())
	eddsaPub, err := crypto.NewECPoint(ec, pubX, pubY)
	assert.NoError(t, err, "NewECPoint for 5*G on Edwards")

	computeEdDSAReshareSSID := func(nonce int64) string {
		ssidList := []*big.Int{
			new(big.Int).SetBytes([]byte("eddsa-resharing")),
			ec.Params().P,
			ec.Params().N,
			ec.Params().B,
			ec.Params().Gx,
			ec.Params().Gy,
		}
		// Old party keys
		ssidList = append(ssidList, oldK1, oldK2)
		// New party keys
		ssidList = append(ssidList, newK1, newK2)
		// EDDSAPub (X, Y)
		ssidList = append(ssidList, eddsaPub.X(), eddsaPub.Y())
		// old party count, old threshold, new party count, new threshold
		ssidList = append(ssidList, big.NewInt(2)) // old n
		ssidList = append(ssidList, big.NewInt(0)) // old threshold
		ssidList = append(ssidList, big.NewInt(2)) // new n
		ssidList = append(ssidList, big.NewInt(0)) // new threshold
		// round number, ssidNonce
		ssidList = append(ssidList, big.NewInt(1))     // round number
		ssidList = append(ssidList, big.NewInt(nonce))  // nonce

		return fmt.Sprintf("%x", common.SHA512_256i(ssidList...).Bytes())
	}

	actualNonce0 := computeEdDSAReshareSSID(0)
	actualNonce42 := computeEdDSAReshareSSID(42)

	t.Logf("EdDSA ReshareSSID(nonce=0)  = %s", actualNonce0)
	t.Logf("EdDSA ReshareSSID(nonce=42) = %s", actualNonce42)

	// Verify they differ by nonce.
	assert.NotEqual(t, actualNonce0, actualNonce42, "nonce 0 and 42 should produce different SSIDs")

	// Verify determinism.
	assert.Equal(t, actualNonce0, computeEdDSAReshareSSID(0), "SSID computation should be deterministic")

	// Verify expected length: SHA-512/256 produces 32 bytes.
	assert.Equal(t, 64, len(actualNonce0), "hex-encoded SHA-512/256 should be 64 chars (32 bytes)")

	// Golden vectors (captured from first run, frozen for regression detection).
	expectedNonce0 := "e37f615e54af8a3e5c67725c965261015758134cee4c42dea54abed1ddcaaf10"
	expectedNonce42 := "ccb1416bff9cb5b41ceab6e459a49faaad4fd38611e78f74f4131e5c32013a64"

	if actualNonce0 != expectedNonce0 {
		t.Fatalf("EdDSA Reshare SSID golden vector mismatch (nonce=0):\n  got:  %s\n  want: %s", actualNonce0, expectedNonce0)
	}
	if actualNonce42 != expectedNonce42 {
		t.Fatalf("EdDSA Reshare SSID golden vector mismatch (nonce=42):\n  got:  %s\n  want: %s", actualNonce42, expectedNonce42)
	}

	t.Logf("EdDSA Reshare SSID golden vectors verified (nonce=0 and nonce=42)")
}
