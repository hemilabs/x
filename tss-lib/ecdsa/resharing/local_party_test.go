// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package resharing_test

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/ipfs/go-log"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/sha3"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/crypto"
	"github.com/hemilabs/x/tss-lib/v2/ecdsa/keygen"
	. "github.com/hemilabs/x/tss-lib/v2/ecdsa/resharing"
	"github.com/hemilabs/x/tss-lib/v2/ecdsa/signing"
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
}

func TestE2EConcurrent(t *testing.T) {
	setUp("info")

	// tss.SetCurve(elliptic.P256())

	threshold, newThreshold := testThreshold, testThreshold

	// PHASE: load keygen fixtures
	firstPartyIdx, extraParties := 1, 1 // extra can be 0 to N-first
	oldKeys, oldPIDs, err := keygen.LoadKeygenTestFixtures(testThreshold+1+extraParties+firstPartyIdx, firstPartyIdx)
	assert.NoError(t, err, "should load keygen fixtures")

	// PHASE: resharing
	oldP2PCtx := tss.NewPeerContext(oldPIDs)
	// init the new parties; re-use the fixture pre-params for speed
	fixtures, _, err := keygen.LoadKeygenTestFixtures(testParticipants)
	if err != nil {
		common.Logger.Info("No test fixtures were found, so the safe primes will be generated from scratch. This may take a while...")
	}
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
		params := tss.NewReSharingParameters(tss.S256(), oldP2PCtx, newP2PCtx, pID, testParticipants, threshold, newPCount, newThreshold)
		P := NewLocalParty(params, oldKeys[j], outCh, endCh).(*LocalParty) // discard old key data
		oldCommittee = append(oldCommittee, P)
	}
	// init the new parties
	for j, pID := range newPIDs {
		params := tss.NewReSharingParameters(tss.S256(), oldP2PCtx, newP2PCtx, pID, testParticipants, threshold, newPCount, newThreshold)
		// do not use in untrusted setting
		params.SetNoProofMod()
		// do not use in untrusted setting
		params.SetNoProofFac()
		save := keygen.NewLocalPartySaveData(newPCount)
		if j < len(fixtures) && len(newPIDs) <= len(fixtures) {
			save.LocalPreParams = fixtures[j].LocalPreParams
		}
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
		fmt.Printf("ACTIVE GOROUTINES: %d\n", runtime.NumGoroutine())
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
			fmt.Println("TODO old:", len(oldCommittee), "new:", len(newCommittee), "finished:", reSharingEnded)
			if atomic.LoadInt32(&reSharingEnded) == int32(len(oldCommittee)+len(newCommittee)) {
				assert.Equal(t, len(oldCommittee), endedOldCommittee)
				t.Logf("Resharing done. Reshared %d participants", reSharingEnded)

				// xj tests: BigXj == xj*G
				for j, key := range newKeys {
					// xj test: BigXj == xj*G
					xj := key.Xi
					gXj := crypto.ScalarBaseMult(tss.S256(), xj)
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
		params := tss.NewParameters(tss.S256(), signP2pCtx, signPID, len(signPIDs), newThreshold)
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
		fmt.Printf("ACTIVE GOROUTINES: %d\n", runtime.NumGoroutine())
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

				// BEGIN ECDSA verify
				pkX, pkY := signKeys[0].ECDSAPub.X(), signKeys[0].ECDSAPub.Y()
				pk := ecdsa.PublicKey{
					Curve: tss.S256(),
					X:     pkX,
					Y:     pkY,
				}
				ok := ecdsa.Verify(&pk, big.NewInt(42).Bytes(),
					new(big.Int).SetBytes(signData.R),
					new(big.Int).SetBytes(signData.S))

				assert.True(t, ok, "ecdsa verify must pass")
				t.Log("ECDSA signing test done.")
				// END ECDSA verify

				return
			}
		}
	}
}

func TestE2EConcurrentNoProofDLN(t *testing.T) {
	setUp("info")

	threshold, newThreshold := testThreshold, testThreshold

	// PHASE: load keygen fixtures
	firstPartyIdx, extraParties := 1, 1
	oldKeys, oldPIDs, err := keygen.LoadKeygenTestFixtures(testThreshold+1+extraParties+firstPartyIdx, firstPartyIdx)
	assert.NoError(t, err, "should load keygen fixtures")

	// PHASE: resharing
	oldP2PCtx := tss.NewPeerContext(oldPIDs)
	fixtures, _, err := keygen.LoadKeygenTestFixtures(testParticipants)
	if err != nil {
		common.Logger.Info("No test fixtures were found, so the safe primes will be generated from scratch. This may take a while...")
	}
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
		params := tss.NewReSharingParameters(tss.S256(), oldP2PCtx, newP2PCtx, pID, testParticipants, threshold, newPCount, newThreshold)
		P := NewLocalParty(params, oldKeys[j], outCh, endCh).(*LocalParty)
		oldCommittee = append(oldCommittee, P)
	}
	// init the new parties — skip ALL classical proofs (on-chain SNARK mode)
	for j, pID := range newPIDs {
		params := tss.NewReSharingParameters(tss.S256(), oldP2PCtx, newP2PCtx, pID, testParticipants, threshold, newPCount, newThreshold)
		params.SetNoProofDLN()
		params.SetNoProofMod()
		params.SetNoProofFac()
		save := keygen.NewLocalPartySaveData(newPCount)
		if j < len(fixtures) && len(newPIDs) <= len(fixtures) {
			save.LocalPreParams = fixtures[j].LocalPreParams
		}
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
				t.Logf("Resharing done with NoProofDLN. Reshared %d participants", reSharingEnded)

				// xj tests: BigXj == xj*G
				for j, key := range newKeys {
					xj := key.Xi
					gXj := crypto.ScalarBaseMult(tss.S256(), xj)
					BigXj := key.BigXj[j]
					assert.True(t, BigXj.Equals(gXj), "ensure BigX_j == g^x_j")
				}

				goto signing
			}
		}
	}

signing:
	// PHASE: signing with reshared keys
	signKeys, signPIDs := newKeys, newPIDs
	signP2pCtx := tss.NewPeerContext(signPIDs)
	signParties := make([]*signing.LocalParty, 0, len(signPIDs))

	signErrCh := make(chan *tss.Error, len(signPIDs))
	signOutCh := make(chan tss.Message, len(signPIDs))
	signEndCh := make(chan *common.SignatureData, len(signPIDs))

	for j, signPID := range signPIDs {
		params := tss.NewParameters(tss.S256(), signP2pCtx, signPID, len(signPIDs), newThreshold)
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

				pkX, pkY := signKeys[0].ECDSAPub.X(), signKeys[0].ECDSAPub.Y()
				pk := ecdsa.PublicKey{
					Curve: tss.S256(),
					X:     pkX,
					Y:     pkY,
				}
				ok := ecdsa.Verify(&pk, big.NewInt(42).Bytes(),
					new(big.Int).SetBytes(signData.R),
					new(big.Int).SetBytes(signData.S))

				assert.True(t, ok, "ecdsa verify must pass")
				t.Log("ECDSA signing test done (NoProofDLN mode).")

				return
			}
		}
	}
}

// TestReshareSSIDGoldenVector verifies that the reshare SSID hash computation
// with known inputs produces hardcoded golden vectors. This ensures cross-language
// compatibility with the Rust compute_reshare_ssid() implementation.
func TestReshareSSIDGoldenVector(t *testing.T) {
	ec := tss.S256()

	// Fixed inputs for reproducibility.
	// Old party keys (keccak256-derived, sorted ascending).
	oldK1 := big.NewInt(100)
	oldK2 := big.NewInt(200)

	// New party keys (keccak256-derived, sorted ascending).
	newK1 := big.NewInt(300)
	newK2 := big.NewInt(400)

	// BigXj: use scalar multiples of the generator for reproducibility.
	// BigXj[0] = 5 * G, BigXj[1] = 7 * G
	// Gap 4: Use FlattenECPoints to exercise the production code path.
	gx := ec.Params().Gx
	gy := ec.Params().Gy
	bigXj0x, bigXj0y := ec.ScalarMult(gx, gy, big.NewInt(5).Bytes())
	bigXj1x, bigXj1y := ec.ScalarMult(gx, gy, big.NewInt(7).Bytes())

	bigXj0, err := crypto.NewECPoint(ec, bigXj0x, bigXj0y)
	assert.NoError(t, err, "NewECPoint for 5*G")
	bigXj1, err := crypto.NewECPoint(ec, bigXj1x, bigXj1y)
	assert.NoError(t, err, "NewECPoint for 7*G")
	bigXjFlat, err := crypto.FlattenECPoints([]*crypto.ECPoint{bigXj0, bigXj1})
	assert.NoError(t, err, "FlattenECPoints")
	// Verify FlattenECPoints produces [X0, Y0, X1, Y1]
	assert.Equal(t, 4, len(bigXjFlat), "FlattenECPoints should produce 2*n entries")
	assert.Equal(t, bigXj0x, bigXjFlat[0], "FlattenECPoints[0] == 5G.X")
	assert.Equal(t, bigXj0y, bigXjFlat[1], "FlattenECPoints[1] == 5G.Y")
	assert.Equal(t, bigXj1x, bigXjFlat[2], "FlattenECPoints[2] == 7G.X")
	assert.Equal(t, bigXj1y, bigXjFlat[3], "FlattenECPoints[3] == 7G.Y")

	// NTilde, H1, H2: use small known values.
	ntilde := []*big.Int{big.NewInt(1000), big.NewInt(2000)}
	h1 := []*big.Int{big.NewInt(3000), big.NewInt(4000)}
	h2 := []*big.Int{big.NewInt(5000), big.NewInt(6000)}

	computeReshareSSID := func(nonce int64) string {
		ssidList := []*big.Int{
			new(big.Int).SetBytes([]byte("ecdsa-resharing")),
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
		// BigXj flattened via FlattenECPoints (Gap 4: exercises production code path)
		ssidList = append(ssidList, bigXjFlat...)
		// NTilde, H1, H2
		ssidList = append(ssidList, ntilde...)
		ssidList = append(ssidList, h1...)
		ssidList = append(ssidList, h2...)
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

	actualNonce0 := computeReshareSSID(0)
	actualNonce42 := computeReshareSSID(42)

	t.Logf("ReshareSSID(nonce=0)  = %s", actualNonce0)
	t.Logf("ReshareSSID(nonce=42) = %s", actualNonce42)

	// Verify they differ by nonce.
	assert.NotEqual(t, actualNonce0, actualNonce42, "nonce 0 and 42 should produce different SSIDs")

	// Verify determinism.
	assert.Equal(t, actualNonce0, computeReshareSSID(0), "SSID computation should be deterministic")

	// Golden vectors: cross-validated against compute_reshare_ssid() in Rust.
	expectedNonce0 := "5b4f81b852e4697ba6cb9398b7a8358de05cb293c50340aefb0a3e54d8fa0c8c"
	expectedNonce42 := "729781fe5ac31b044c30551ec822d035aa0bf952560c13539b5c220119d99db1"

	if actualNonce0 != expectedNonce0 {
		t.Fatalf("Reshare SSID golden vector mismatch (nonce=0):\n  got:  %s\n  want: %s", actualNonce0, expectedNonce0)
	}
	if actualNonce42 != expectedNonce42 {
		t.Fatalf("Reshare SSID golden vector mismatch (nonce=42):\n  got:  %s\n  want: %s", actualNonce42, expectedNonce42)
	}

	t.Logf("Reshare SSID golden vectors verified (nonce=0 and nonce=42)")
}

// TestReshareSSIDProductionSizedGoldenVector verifies the reshare SSID hash with
// production-sized inputs: 32-byte keccak256 party keys, 256-byte (2048-bit) NTilde/H1/H2,
// and BigXj via FlattenECPoints. Cross-validated against Rust golden vector test.
func TestReshareSSIDProductionSizedGoldenVector(t *testing.T) {
	ec := tss.S256()

	// Party keys: keccak256 of compressed G, 2*G, 3*G (32 bytes each, sorted ascending).
	gx := ec.Params().Gx
	gy := ec.Params().Gy

	compressedG := make([]byte, 33)
	compressedG[0] = 0x02
	copy(compressedG[33-len(gx.Bytes()):], gx.Bytes())

	twoGx, twoGy := ec.ScalarMult(gx, gy, big.NewInt(2).Bytes())
	compressed2G := make([]byte, 33)
	if twoGy.Bit(0) == 0 {
		compressed2G[0] = 0x02
	} else {
		compressed2G[0] = 0x03
	}
	copy(compressed2G[33-len(twoGx.Bytes()):], twoGx.Bytes())

	threeGx, threeGy := ec.ScalarMult(gx, gy, big.NewInt(3).Bytes())
	compressed3G := make([]byte, 33)
	if threeGy.Bit(0) == 0 {
		compressed3G[0] = 0x02
	} else {
		compressed3G[0] = 0x03
	}
	copy(compressed3G[33-len(threeGx.Bytes()):], threeGx.Bytes())

	keccak := func(data []byte) *big.Int {
		h := sha3.NewLegacyKeccak256()
		h.Write(data)
		return new(big.Int).SetBytes(h.Sum(nil))
	}

	k1, k2, k3 := keccak(compressedG), keccak(compressed2G), keccak(compressed3G)
	// Sort ascending
	keys := []*big.Int{k1, k2, k3}
	for i := 0; i < 3; i++ {
		for j := i + 1; j < 3; j++ {
			if keys[i].Cmp(keys[j]) > 0 {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}

	// BigXj = [G, 2*G, 3*G] via FlattenECPoints (Gap 5: exercises production code path)
	bigXj0, _ := crypto.NewECPoint(ec, gx, gy)
	bigXj1, _ := crypto.NewECPoint(ec, twoGx, twoGy)
	bigXj2, _ := crypto.NewECPoint(ec, threeGx, threeGy)
	bigXjFlat, err := crypto.FlattenECPoints([]*crypto.ECPoint{bigXj0, bigXj1, bigXj2})
	assert.NoError(t, err)
	assert.Equal(t, 6, len(bigXjFlat))

	// 2048-bit NTilde/H1/H2: (2^2048 - offset) for reproducibility
	base2048 := new(big.Int).Lsh(big.NewInt(1), 2048)
	ntilde := make([]*big.Int, 3)
	h1 := make([]*big.Int, 3)
	h2 := make([]*big.Int, 3)
	for i := 0; i < 3; i++ {
		ntilde[i] = new(big.Int).Sub(base2048, big.NewInt(int64(100+i)))
		h1[i] = new(big.Int).Sub(base2048, big.NewInt(int64(200+i)))
		h2[i] = new(big.Int).Sub(base2048, big.NewInt(int64(300+i)))
	}

	ssidList := []*big.Int{
		new(big.Int).SetBytes([]byte("ecdsa-resharing")),
		ec.Params().P,
		ec.Params().N,
		ec.Params().B,
		ec.Params().Gx,
		ec.Params().Gy,
	}
	ssidList = append(ssidList, keys...)
	ssidList = append(ssidList, keys...) // same new keys
	ssidList = append(ssidList, bigXjFlat...)
	ssidList = append(ssidList, ntilde...)
	ssidList = append(ssidList, h1...)
	ssidList = append(ssidList, h2...)
	ssidList = append(ssidList, big.NewInt(3)) // old_n
	ssidList = append(ssidList, big.NewInt(1)) // old_threshold
	ssidList = append(ssidList, big.NewInt(3)) // new_n
	ssidList = append(ssidList, big.NewInt(1)) // new_threshold
	ssidList = append(ssidList, big.NewInt(1)) // round number
	ssidList = append(ssidList, big.NewInt(0)) // nonce

	actual := fmt.Sprintf("%x", common.SHA512_256i(ssidList...).Bytes())
	expected := "13119079f0e22b47772e8a77a9c47aac6a208edbe1a7c8bddead2bb06dfec980"

	if actual != expected {
		t.Fatalf("Production-sized reshare SSID golden vector mismatch:\n  got:  %s\n  want: %s", actual, expected)
	}
	t.Logf("Production-sized reshare SSID golden vector verified: %s", actual)
}

// TestGetPolyAfterResharing runs a full resharing and verifies that GetPoly()
// on old committee parties returns a non-nil polynomial of length newThreshold+1
// with a non-nil, non-zero constant term.
func TestGetPolyAfterResharing(t *testing.T) {
	setUp("info")

	threshold, newThreshold := testThreshold, testThreshold

	// PHASE: load keygen fixtures
	firstPartyIdx, extraParties := 1, 1
	oldKeys, oldPIDs, err := keygen.LoadKeygenTestFixtures(testThreshold+1+extraParties+firstPartyIdx, firstPartyIdx)
	assert.NoError(t, err, "should load keygen fixtures")

	// PHASE: resharing
	oldP2PCtx := tss.NewPeerContext(oldPIDs)
	fixtures, _, err := keygen.LoadKeygenTestFixtures(testParticipants)
	if err != nil {
		common.Logger.Info("No test fixtures were found, so the safe primes will be generated from scratch. This may take a while...")
	}
	newPIDs := tss.GenerateTestPartyIDs(testParticipants)
	newP2PCtx := tss.NewPeerContext(newPIDs)
	newPCount := len(newPIDs)

	oldCommittee := make([]*LocalParty, 0, len(oldPIDs))
	newCommittee := make([]*LocalParty, 0, newPCount)
	bothCommitteesPax := len(oldPIDs) + newPCount

	errCh := make(chan *tss.Error, bothCommitteesPax)
	outCh := make(chan tss.Message, bothCommitteesPax)
	endCh := make(chan *keygen.LocalPartySaveData, bothCommitteesPax)

	updater := test.SharedPartyUpdater

	for j, pID := range oldPIDs {
		params := tss.NewReSharingParameters(tss.S256(), oldP2PCtx, newP2PCtx, pID, testParticipants, threshold, newPCount, newThreshold)
		P := NewLocalParty(params, oldKeys[j], outCh, endCh).(*LocalParty)
		oldCommittee = append(oldCommittee, P)
	}
	for j, pID := range newPIDs {
		params := tss.NewReSharingParameters(tss.S256(), oldP2PCtx, newP2PCtx, pID, testParticipants, threshold, newPCount, newThreshold)
		params.SetNoProofMod()
		params.SetNoProofFac()
		save := keygen.NewLocalPartySaveData(newPCount)
		if j < len(fixtures) && len(newPIDs) <= len(fixtures) {
			save.LocalPreParams = fixtures[j].LocalPreParams
		}
		P := NewLocalParty(params, save, outCh, endCh).(*LocalParty)
		newCommittee = append(newCommittee, P)
	}

	for _, P := range newCommittee {
		go func(P *LocalParty) {
			if err := P.Start(); err != nil {
				errCh <- err
			}
		}(P)
	}
	for _, P := range oldCommittee {
		go func(P *LocalParty) {
			if err := P.Start(); err != nil {
				errCh <- err
			}
		}(P)
	}

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

		case <-endCh:
			atomic.AddInt32(&reSharingEnded, 1)
			if atomic.LoadInt32(&reSharingEnded) == int32(len(oldCommittee)+len(newCommittee)) {
				t.Logf("Resharing done. Verifying GetPoly() on %d old committee parties", len(oldCommittee))

				for i, P := range oldCommittee {
					poly := P.GetPoly()
					assert.NotNilf(t, poly, "old party %d: GetPoly() should not be nil", i)
					assert.Equalf(t, newThreshold+1, len(poly), "old party %d: poly length should be newThreshold+1", i)

					// poly[0] is the constant term (the party's sub-share of the secret)
					assert.NotNilf(t, poly[0], "old party %d: poly[0] should not be nil", i)
					assert.NotEqualf(t, 0, poly[0].Sign(), "old party %d: poly[0] should be non-zero", i)
				}

				t.Log("GetPoly() verification passed for all old committee parties.")
				return
			}
		}
	}
}

// TestGetNewVsAfterResharing runs a full resharing and verifies that GetNewVs()
// on old committee parties returns non-nil Feldman VSS commitments of length
// newThreshold+1, with each point non-nil and on the curve.
func TestGetNewVsAfterResharing(t *testing.T) {
	setUp("info")

	threshold, newThreshold := testThreshold, testThreshold

	// PHASE: load keygen fixtures
	firstPartyIdx, extraParties := 1, 1
	oldKeys, oldPIDs, err := keygen.LoadKeygenTestFixtures(testThreshold+1+extraParties+firstPartyIdx, firstPartyIdx)
	assert.NoError(t, err, "should load keygen fixtures")

	// PHASE: resharing
	oldP2PCtx := tss.NewPeerContext(oldPIDs)
	fixtures, _, err := keygen.LoadKeygenTestFixtures(testParticipants)
	if err != nil {
		common.Logger.Info("No test fixtures were found, so the safe primes will be generated from scratch. This may take a while...")
	}
	newPIDs := tss.GenerateTestPartyIDs(testParticipants)
	newP2PCtx := tss.NewPeerContext(newPIDs)
	newPCount := len(newPIDs)

	oldCommittee := make([]*LocalParty, 0, len(oldPIDs))
	newCommittee := make([]*LocalParty, 0, newPCount)
	bothCommitteesPax := len(oldPIDs) + newPCount

	errCh := make(chan *tss.Error, bothCommitteesPax)
	outCh := make(chan tss.Message, bothCommitteesPax)
	endCh := make(chan *keygen.LocalPartySaveData, bothCommitteesPax)

	updater := test.SharedPartyUpdater

	for j, pID := range oldPIDs {
		params := tss.NewReSharingParameters(tss.S256(), oldP2PCtx, newP2PCtx, pID, testParticipants, threshold, newPCount, newThreshold)
		P := NewLocalParty(params, oldKeys[j], outCh, endCh).(*LocalParty)
		oldCommittee = append(oldCommittee, P)
	}
	for j, pID := range newPIDs {
		params := tss.NewReSharingParameters(tss.S256(), oldP2PCtx, newP2PCtx, pID, testParticipants, threshold, newPCount, newThreshold)
		params.SetNoProofMod()
		params.SetNoProofFac()
		save := keygen.NewLocalPartySaveData(newPCount)
		if j < len(fixtures) && len(newPIDs) <= len(fixtures) {
			save.LocalPreParams = fixtures[j].LocalPreParams
		}
		P := NewLocalParty(params, save, outCh, endCh).(*LocalParty)
		newCommittee = append(newCommittee, P)
	}

	for _, P := range newCommittee {
		go func(P *LocalParty) {
			if err := P.Start(); err != nil {
				errCh <- err
			}
		}(P)
	}
	for _, P := range oldCommittee {
		go func(P *LocalParty) {
			if err := P.Start(); err != nil {
				errCh <- err
			}
		}(P)
	}

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

		case <-endCh:
			atomic.AddInt32(&reSharingEnded, 1)
			if atomic.LoadInt32(&reSharingEnded) == int32(len(oldCommittee)+len(newCommittee)) {
				t.Logf("Resharing done. Verifying GetNewVs() on %d old committee parties", len(oldCommittee))

				ec := tss.S256()
				for i, P := range oldCommittee {
					vs := P.GetNewVs()
					assert.NotNilf(t, vs, "old party %d: GetNewVs() should not be nil", i)
					assert.Equalf(t, newThreshold+1, len(vs), "old party %d: Vs length should be newThreshold+1", i)

					for k, point := range vs {
						assert.NotNilf(t, point, "old party %d: Vs[%d] should not be nil", i, k)
						assert.Truef(t, point.IsOnCurve(), "old party %d: Vs[%d] should be on the curve", i, k)
						assert.Truef(t, ec.IsOnCurve(point.X(), point.Y()),
							"old party %d: Vs[%d] coordinates should satisfy the curve equation", i, k)
					}
				}

				t.Log("GetNewVs() verification passed for all old committee parties.")
				return
			}
		}
	}
}
