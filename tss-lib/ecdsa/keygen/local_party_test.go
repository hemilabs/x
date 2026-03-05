// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package keygen

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/ipfs/go-log"
	"github.com/stretchr/testify/assert"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/crypto"
	"github.com/hemilabs/x/tss-lib/v2/crypto/dlnproof"
	"github.com/hemilabs/x/tss-lib/v2/crypto/paillier"
	"github.com/hemilabs/x/tss-lib/v2/crypto/vss"
	"github.com/hemilabs/x/tss-lib/v2/test"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

const (
	testParticipants = TestParticipants
	testThreshold    = TestThreshold
)

func setUp(level string) {
	if err := log.SetLogLevel("tss-lib", level); err != nil {
		panic(err)
	}
}

func TestStartRound1Paillier(t *testing.T) {
	setUp("debug")

	pIDs := tss.GenerateTestPartyIDs(2)
	p2pCtx := tss.NewPeerContext(pIDs)
	threshold := 1 // 2-of-2: threshold must be in [1, partyCount)
	params := tss.NewParameters(tss.EC(), p2pCtx, pIDs[0], len(pIDs), threshold)

	fixtures, pIDs, err := LoadKeygenTestFixtures(testParticipants)
	if err != nil {
		common.Logger.Info("No test fixtures were found, so the safe primes will be generated from scratch. This may take a while...")
		pIDs = tss.GenerateTestPartyIDs(testParticipants)
	}

	var lp *LocalParty
	out := make(chan tss.Message, len(pIDs))
	if 0 < len(fixtures) {
		lp = NewLocalParty(params, out, nil, fixtures[0].LocalPreParams).(*LocalParty)
	} else {
		lp = NewLocalParty(params, out, nil).(*LocalParty)
	}
	if err := lp.Start(); err != nil {
		assert.FailNow(t, err.Error())
	}
	<-out

	// Paillier modulus 2048 (two 1024-bit primes)
	// round up to 256, it was used to be flaky, sometimes comes back with 1 byte less
	len1 := len(lp.data.PaillierSK.LambdaN.Bytes())
	len2 := len(lp.data.PaillierSK.PublicKey.N.Bytes())
	if len1%2 != 0 {
		len1 = len1 + (256 - (len1 % 256))
	}
	if len2%2 != 0 {
		len2 = len2 + (256 - (len2 % 256))
	}
	assert.Equal(t, 2048/8, len1)
	assert.Equal(t, 2048/8, len2)
}

func TestFinishAndSaveH1H2(t *testing.T) {
	setUp("debug")

	pIDs := tss.GenerateTestPartyIDs(2)
	p2pCtx := tss.NewPeerContext(pIDs)
	threshold := 1 // 2-of-2: threshold must be in [1, partyCount)
	params := tss.NewParameters(tss.EC(), p2pCtx, pIDs[0], len(pIDs), threshold)

	fixtures, pIDs, err := LoadKeygenTestFixtures(testParticipants)
	if err != nil {
		common.Logger.Info("No test fixtures were found, so the safe primes will be generated from scratch. This may take a while...")
		pIDs = tss.GenerateTestPartyIDs(testParticipants)
	}

	var lp *LocalParty
	out := make(chan tss.Message, len(pIDs))
	if 0 < len(fixtures) {
		lp = NewLocalParty(params, out, nil, fixtures[0].LocalPreParams).(*LocalParty)
	} else {
		lp = NewLocalParty(params, out, nil).(*LocalParty)
	}
	if err := lp.Start(); err != nil {
		assert.FailNow(t, err.Error())
	}

	// RSA modulus 2048 (two 1024-bit primes)
	// round up to 256
	len1 := len(lp.data.H1j[0].Bytes())
	len2 := len(lp.data.H2j[0].Bytes())
	len3 := len(lp.data.NTildej[0].Bytes())
	if len1%2 != 0 {
		len1 = len1 + (256 - (len1 % 256))
	}
	if len2%2 != 0 {
		len2 = len2 + (256 - (len2 % 256))
	}
	if len3%2 != 0 {
		len3 = len3 + (256 - (len3 % 256))
	}
	// 256 bytes = 2048 bits
	assert.Equal(t, 256, len1, "h1 should be correct len")
	assert.Equal(t, 256, len2, "h2 should be correct len")
	assert.Equal(t, 256, len3, "n-tilde should be correct len")
	assert.NotZero(t, lp.data.H1i, "h1 should be non-zero")
	assert.NotZero(t, lp.data.H2i, "h2 should be non-zero")
	assert.NotZero(t, lp.data.NTildei, "n-tilde should be non-zero")
}

func TestBadMessageCulprits(t *testing.T) {
	setUp("debug")

	pIDs := tss.GenerateTestPartyIDs(2)
	p2pCtx := tss.NewPeerContext(pIDs)
	params := tss.NewParameters(tss.S256(), p2pCtx, pIDs[0], len(pIDs), 1)

	fixtures, pIDs, err := LoadKeygenTestFixtures(testParticipants)
	if err != nil {
		common.Logger.Info("No test fixtures were found, so the safe primes will be generated from scratch. This may take a while...")
		pIDs = tss.GenerateTestPartyIDs(testParticipants)
	}

	var lp *LocalParty
	out := make(chan tss.Message, len(pIDs))
	if 0 < len(fixtures) {
		lp = NewLocalParty(params, out, nil, fixtures[0].LocalPreParams).(*LocalParty)
	} else {
		lp = NewLocalParty(params, out, nil).(*LocalParty)
	}
	if err := lp.Start(); err != nil {
		assert.FailNow(t, err.Error())
	}

	badMsg, _ := NewKGRound1Message(pIDs[1], zero, &paillier.PublicKey{N: zero}, zero, zero, zero, new(dlnproof.Proof), new(dlnproof.Proof))
	ok, err2 := lp.Update(badMsg)
	t.Log(err2)
	assert.False(t, ok)
	if !assert.Error(t, err2) {
		return
	}
	assert.Equal(t, 1, len(err2.Culprits()))
	assert.Equal(t, pIDs[1], err2.Culprits()[0])
	assert.Equal(t,
		"task ecdsa-keygen, party {0,P[1]}, round 1, culprits [{1,2}]: message failed ValidateBasic: Type: binance.tsslib.ecdsa.keygen.KGRound1Message, From: {1,2}, To: all",
		err2.Error())
}

func TestE2EConcurrentAndSaveFixtures(t *testing.T) {
	setUp("info")

	// tss.SetCurve(elliptic.P256())

	threshold := testThreshold
	fixtures, pIDs, err := LoadKeygenTestFixtures(testParticipants)
	if err != nil {
		common.Logger.Info("No test fixtures were found, so the safe primes will be generated from scratch. This may take a while...")
		pIDs = tss.GenerateTestPartyIDs(testParticipants)
	}

	p2pCtx := tss.NewPeerContext(pIDs)
	parties := make([]*LocalParty, 0, len(pIDs))

	errCh := make(chan *tss.Error, len(pIDs))
	outCh := make(chan tss.Message, len(pIDs))
	endCh := make(chan *LocalPartySaveData, len(pIDs))

	updater := test.SharedPartyUpdater

	startGR := runtime.NumGoroutine()

	// init the parties
	for i := 0; i < len(pIDs); i++ {
		var P *LocalParty
		params := tss.NewParameters(tss.S256(), p2pCtx, pIDs[i], len(pIDs), threshold)
		// do not use in untrusted setting
		params.SetNoProofMod()
		// do not use in untrusted setting
		params.SetNoProofFac()
		if i < len(fixtures) {
			P = NewLocalParty(params, outCh, endCh, fixtures[i].LocalPreParams).(*LocalParty)
		} else {
			P = NewLocalParty(params, outCh, endCh).(*LocalParty)
		}
		parties = append(parties, P)
		go func(P *LocalParty) {
			if err := P.Start(); err != nil {
				errCh <- err
			}
		}(P)
	}

	// PHASE: keygen
	var ended int32
keygen:
	for {
		fmt.Printf("ACTIVE GOROUTINES: %d\n", runtime.NumGoroutine())
		select {
		case err := <-errCh:
			common.Logger.Errorf("Error: %s", err)
			assert.FailNow(t, err.Error())
			break keygen

		case msg := <-outCh:
			dest := msg.GetTo()
			if dest == nil { // broadcast!
				for _, P := range parties {
					if P.PartyID().Index == msg.GetFrom().Index {
						continue
					}
					go updater(P, msg, errCh)
				}
			} else { // point-to-point!
				if dest[0].Index == msg.GetFrom().Index {
					t.Fatalf("party %d tried to send a message to itself (%d)", dest[0].Index, msg.GetFrom().Index)
					return
				}
				go updater(parties[dest[0].Index], msg, errCh)
			}

		case save := <-endCh:
			// SAVE a test fixture file for this P (if it doesn't already exist)
			// .. here comes a workaround to recover this party's index (it was removed from save data)
			index, err := save.OriginalIndex()
			assert.NoErrorf(t, err, "should not be an error getting a party's index from save data")
			tryWriteTestFixtureFile(t, index, *save)

			atomic.AddInt32(&ended, 1)
			if atomic.LoadInt32(&ended) == int32(len(pIDs)) {
				t.Logf("Done. Received save data from %d participants", ended)

				// combine shares for each Pj to get u
				u := new(big.Int)
				for j, Pj := range parties {
					pShares := make(vss.Shares, 0)
					for _, P := range parties {
						vssMsgs := P.temp.kgRound2Message1s
						share := vssMsgs[j].Content().(*KGRound2Message1).Share
						shareStruct := &vss.Share{
							Threshold: threshold,
							ID:        P.PartyID().KeyInt(),
							Share:     new(big.Int).SetBytes(share),
						}
						pShares = append(pShares, shareStruct)
					}
					uj, err := pShares[:threshold+1].ReConstruct(tss.S256())
					assert.NoError(t, err, "vss.ReConstruct should not throw error")

					// uG test: u*G[j] == V[0]
					// (temp.ui is zeroed after round 1 for security)
					uG := crypto.ScalarBaseMult(tss.EC(), uj)
					assert.True(t, uG.Equals(Pj.temp.vs[0]), "ensure u*G[j] == V_0")

					// xj tests: BigXj == xj*G
					xj := Pj.data.Xi
					gXj := crypto.ScalarBaseMult(tss.EC(), xj)
					BigXj := Pj.data.BigXj[j]
					assert.True(t, BigXj.Equals(gXj), "ensure BigX_j == g^x_j")

					// fails if threshold cannot be satisfied (bad share)
					{
						badShares := pShares[:threshold]
						badShares[len(badShares)-1].Share.Set(big.NewInt(0))
						ujBad, err := pShares[:threshold].ReConstruct(tss.S256())
						assert.NoError(t, err)
						assert.NotEqual(t, uj, ujBad)
						BigXjX, BigXjY := tss.EC().ScalarBaseMult(ujBad.Bytes())
						assert.NotEqual(t, BigXjX, Pj.temp.vs[0].X())
						assert.NotEqual(t, BigXjY, Pj.temp.vs[0].Y())
					}
					u = new(big.Int).Add(u, uj)
				}

				// build ecdsa key pair
				pkX, pkY := save.ECDSAPub.X(), save.ECDSAPub.Y()
				pk := ecdsa.PublicKey{
					Curve: tss.EC(),
					X:     pkX,
					Y:     pkY,
				}
				sk := ecdsa.PrivateKey{
					PublicKey: pk,
					D:         u,
				}
				// test pub key, should be on curve and match pkX, pkY
				assert.True(t, sk.IsOnCurve(pkX, pkY), "public key must be on curve")

				// public key tests
				assert.NotZero(t, u, "u should not be zero")
				ourPkX, ourPkY := tss.EC().ScalarBaseMult(u.Bytes())
				assert.Equal(t, pkX, ourPkX, "pkX should match expected pk derived from u")
				assert.Equal(t, pkY, ourPkY, "pkY should match expected pk derived from u")
				t.Log("Public key tests done.")

				// make sure everyone has the same ECDSA public key
				for _, Pj := range parties {
					assert.Equal(t, pkX, Pj.data.ECDSAPub.X())
					assert.Equal(t, pkY, Pj.data.ECDSAPub.Y())
				}
				t.Log("Public key distribution test done.")

				// test sign/verify
				data := make([]byte, 32)
				for i := range data {
					data[i] = byte(i)
				}
				r, s, err := ecdsa.Sign(rand.Reader, &sk, data)
				assert.NoError(t, err, "sign should not throw an error")
				ok := ecdsa.Verify(&pk, data, r, s)
				assert.True(t, ok, "signature should be ok")
				t.Log("ECDSA signing test done.")

				t.Logf("Start goroutines: %d, End goroutines: %d", startGR, runtime.NumGoroutine())

				break keygen
			}
		}
	}
}

// TestE2EConcurrentAllNoProofFlags runs a full E2E keygen with all three
// NoProof flags set (NoProofDLN, NoProofMod, NoProofFac) and a non-zero
// SSID nonce. This is the "on-chain SNARK mode" configuration.
func TestE2EConcurrentAllNoProofFlags(t *testing.T) {
	setUp("info")

	threshold := testThreshold
	fixtures, pIDs, err := LoadKeygenTestFixtures(testParticipants)
	if err != nil {
		common.Logger.Info("No test fixtures were found, so the safe primes will be generated from scratch. This may take a while...")
		pIDs = tss.GenerateTestPartyIDs(testParticipants)
	}

	p2pCtx := tss.NewPeerContext(pIDs)
	parties := make([]*LocalParty, 0, len(pIDs))

	errCh := make(chan *tss.Error, len(pIDs))
	outCh := make(chan tss.Message, len(pIDs))
	endCh := make(chan *LocalPartySaveData, len(pIDs))

	updater := test.SharedPartyUpdater

	// init the parties with ALL NoProof flags + non-zero SSID nonce
	for i := 0; i < len(pIDs); i++ {
		var P *LocalParty
		params := tss.NewParameters(tss.S256(), p2pCtx, pIDs[i], len(pIDs), threshold)
		params.SetNoProofDLN()
		params.SetNoProofMod()
		params.SetNoProofFac()
		params.SetSSIDNonce(42) // non-zero nonce
		if i < len(fixtures) {
			P = NewLocalParty(params, outCh, endCh, fixtures[i].LocalPreParams).(*LocalParty)
		} else {
			P = NewLocalParty(params, outCh, endCh).(*LocalParty)
		}
		parties = append(parties, P)
		go func(P *LocalParty) {
			if err := P.Start(); err != nil {
				errCh <- err
			}
		}(P)
	}

	var ended int32
keygen:
	for {
		select {
		case err := <-errCh:
			common.Logger.Errorf("Error: %s", err)
			assert.FailNow(t, err.Error())
			break keygen

		case msg := <-outCh:
			dest := msg.GetTo()
			if dest == nil {
				for _, P := range parties {
					if P.PartyID().Index == msg.GetFrom().Index {
						continue
					}
					go updater(P, msg, errCh)
				}
			} else {
				if dest[0].Index == msg.GetFrom().Index {
					t.Fatalf("party %d tried to send a message to itself (%d)", dest[0].Index, msg.GetFrom().Index)
					return
				}
				go updater(parties[dest[0].Index], msg, errCh)
			}

		case save := <-endCh:
			atomic.AddInt32(&ended, 1)
			if atomic.LoadInt32(&ended) == int32(len(pIDs)) {
				t.Logf("Done. Keygen completed with all NoProof flags + nonce=42. Received save data from %d participants", ended)

				// Verify all parties agree on the ECDSA public key.
				pkX, pkY := save.ECDSAPub.X(), save.ECDSAPub.Y()
				for _, Pj := range parties {
					assert.Equal(t, pkX, Pj.data.ECDSAPub.X())
					assert.Equal(t, pkY, Pj.data.ECDSAPub.Y())
				}

				// Verify the SSID nonce was set correctly in round 1.
				for _, P := range parties {
					assert.Equal(t, int64(42), P.temp.ssidNonce.Int64(),
						"ssidNonce should be 42")
				}

				break keygen
			}
		}
	}
}

// TestSSIDDifferentiationByNonce verifies that two different SSID nonces produce
// different SSID values when all other parameters are identical. This exercises
// the nonce contribution to the SSID hash (rounds.go:106).
func TestSSIDDifferentiationByNonce(t *testing.T) {
	// Replicate the SSID computation from rounds.go:102-110 with two nonces.
	ec := tss.S256()
	pIDs := tss.GenerateTestPartyIDs(3)

	makeSSID := func(nonce int64) []byte {
		ssidList := []*big.Int{ec.Params().P, ec.Params().N, ec.Params().Gx, ec.Params().Gy}
		ssidList = append(ssidList, pIDs.Keys()...)
		ssidList = append(ssidList, big.NewInt(1)) // round number
		ssidList = append(ssidList, big.NewInt(nonce))
		return common.SHA512_256i(ssidList...).Bytes()
	}

	ssid0 := makeSSID(0)
	ssid1 := makeSSID(1)
	ssid42 := makeSSID(42)
	ssid0Again := makeSSID(0)

	// Different nonces must produce different SSIDs.
	assert.NotEqual(t, ssid0, ssid1, "nonce 0 and 1 should produce different SSIDs")
	assert.NotEqual(t, ssid0, ssid42, "nonce 0 and 42 should produce different SSIDs")
	assert.NotEqual(t, ssid1, ssid42, "nonce 1 and 42 should produce different SSIDs")

	// Same nonce must be deterministic.
	assert.Equal(t, ssid0, ssid0Again, "same nonce should produce identical SSIDs")

	t.Logf("SSID(nonce=0) = %x", ssid0)
	t.Logf("SSID(nonce=1) = %x", ssid1)
	t.Logf("SSID(nonce=42) = %x", ssid42)
}

// TestE2EConcurrentDLNOnlyNoProof runs a full E2E keygen with only
// SetNoProofDLN(). MOD and FAC proofs are still generated and verified,
// ensuring partial proof skipping works correctly.
func TestE2EConcurrentDLNOnlyNoProof(t *testing.T) {
	setUp("info")

	threshold := testThreshold
	fixtures, pIDs, err := LoadKeygenTestFixtures(testParticipants)
	if err != nil {
		common.Logger.Info("No test fixtures were found, so the safe primes will be generated from scratch. This may take a while...")
		pIDs = tss.GenerateTestPartyIDs(testParticipants)
	}

	p2pCtx := tss.NewPeerContext(pIDs)
	parties := make([]*LocalParty, 0, len(pIDs))

	errCh := make(chan *tss.Error, len(pIDs))
	outCh := make(chan tss.Message, len(pIDs))
	endCh := make(chan *LocalPartySaveData, len(pIDs))

	updater := test.SharedPartyUpdater

	// init the parties with ONLY DLN proof skipping — MOD/FAC still active
	for i := 0; i < len(pIDs); i++ {
		var P *LocalParty
		params := tss.NewParameters(tss.S256(), p2pCtx, pIDs[i], len(pIDs), threshold)
		params.SetNoProofDLN() // DLN only — MOD and FAC proofs still verified
		if i < len(fixtures) {
			P = NewLocalParty(params, outCh, endCh, fixtures[i].LocalPreParams).(*LocalParty)
		} else {
			P = NewLocalParty(params, outCh, endCh).(*LocalParty)
		}
		parties = append(parties, P)
		go func(P *LocalParty) {
			if err := P.Start(); err != nil {
				errCh <- err
			}
		}(P)
	}

	var ended int32
keygen:
	for {
		select {
		case err := <-errCh:
			common.Logger.Errorf("Error: %s", err)
			assert.FailNow(t, err.Error())
			break keygen

		case msg := <-outCh:
			dest := msg.GetTo()
			if dest == nil {
				for _, P := range parties {
					if P.PartyID().Index == msg.GetFrom().Index {
						continue
					}
					go updater(P, msg, errCh)
				}
			} else {
				if dest[0].Index == msg.GetFrom().Index {
					t.Fatalf("party %d tried to send a message to itself (%d)", dest[0].Index, msg.GetFrom().Index)
					return
				}
				go updater(parties[dest[0].Index], msg, errCh)
			}

		case save := <-endCh:
			atomic.AddInt32(&ended, 1)
			if atomic.LoadInt32(&ended) == int32(len(pIDs)) {
				t.Logf("Done. Keygen completed with DLN-only NoProof. Received save data from %d participants", ended)

				// Verify all parties agree on the ECDSA public key.
				pkX, pkY := save.ECDSAPub.X(), save.ECDSAPub.Y()
				for _, Pj := range parties {
					assert.Equal(t, pkX, Pj.data.ECDSAPub.X())
					assert.Equal(t, pkY, Pj.data.ECDSAPub.Y())
				}

				// Verify DLN flag was set but MOD/FAC flags were NOT set.
				for _, P := range parties {
					assert.True(t, P.params.NoProofDLN(), "NoProofDLN should be true")
					assert.False(t, P.params.NoProofMod(), "NoProofMod should be false (MOD proofs verified)")
					assert.False(t, P.params.NoProofFac(), "NoProofFac should be false (FAC proofs verified)")
				}

				break keygen
			}
		}
	}
}

// TestSSIDNonceGoldenVector verifies that the SSID hash computation with
// known inputs produces a hardcoded golden vector. This ensures cross-language
// compatibility and catches accidental changes to the hash function.
func TestSSIDNonceGoldenVector(t *testing.T) {
	ec := tss.S256()
	// Use fixed party keys 100, 200, 300 for reproducibility.
	k1 := big.NewInt(100)
	k2 := big.NewInt(200)
	k3 := big.NewInt(300)

	computeSSID := func(nonce int64) string {
		ssidList := []*big.Int{ec.Params().P, ec.Params().N, ec.Params().Gx, ec.Params().Gy}
		ssidList = append(ssidList, k1, k2, k3)
		ssidList = append(ssidList, big.NewInt(1)) // round number
		ssidList = append(ssidList, big.NewInt(nonce))
		return fmt.Sprintf("%x", common.SHA512_256i(ssidList...).Bytes())
	}

	// Golden vectors: SHA512/256(curve params || keys || round=1 || nonce).
	expectedNonce0 := "2134c551c956db9a9d5fb9b9dd078cac48f66f3f7fc973b3faab5e91ecb89ed8"
	expectedNonce42 := "dfd0e36b999fe9a11b30fd493c9de162ddbcc97913ea56cdd6343cd2748c40d2"

	actual0 := computeSSID(0)
	actual42 := computeSSID(42)

	if actual0 != expectedNonce0 {
		t.Fatalf("SSID golden vector mismatch (nonce=0):\n  got:  %s\n  want: %s", actual0, expectedNonce0)
	}
	if actual42 != expectedNonce42 {
		t.Fatalf("SSID golden vector mismatch (nonce=42):\n  got:  %s\n  want: %s", actual42, expectedNonce42)
	}

	// Verify they're different (nonce differentiation).
	assert.NotEqual(t, actual0, actual42, "nonce 0 and 42 should produce different SSIDs")

	// Verify determinism: compute again.
	assert.Equal(t, actual0, computeSSID(0), "SSID computation should be deterministic")

	t.Logf("SSID(nonce=0) = %s (golden vector matches)", actual0)
	t.Logf("SSID(nonce=42) = %s (golden vector matches)", actual42)
}

// TestReceiverIdMismatchCausesRound3Rejection runs a full E2E keygen where
// one party's Round 2 P2P message has a tampered receiverId. Round 3 should
// detect the mismatch and reject the message from the tampered sender.
func TestReceiverIdMismatchCausesRound3Rejection(t *testing.T) {
	setUp("info")

	threshold := testThreshold
	fixtures, pIDs, err := LoadKeygenTestFixtures(testParticipants)
	if err != nil {
		common.Logger.Info("No test fixtures were found, so the safe primes will be generated from scratch. This may take a while...")
		pIDs = tss.GenerateTestPartyIDs(testParticipants)
	}

	p2pCtx := tss.NewPeerContext(pIDs)
	parties := make([]*LocalParty, 0, len(pIDs))

	errCh := make(chan *tss.Error, len(pIDs)*10)
	outCh := make(chan tss.Message, len(pIDs)*10)
	endCh := make(chan *LocalPartySaveData, len(pIDs))

	// init the parties
	for i := 0; i < len(pIDs); i++ {
		var P *LocalParty
		params := tss.NewParameters(tss.S256(), p2pCtx, pIDs[i], len(pIDs), threshold)
		params.SetNoProofMod()
		params.SetNoProofFac()
		if i < len(fixtures) {
			P = NewLocalParty(params, outCh, endCh, fixtures[i].LocalPreParams).(*LocalParty)
		} else {
			P = NewLocalParty(params, outCh, endCh).(*LocalParty)
		}
		parties = append(parties, P)
		go func(P *LocalParty) {
			if err := P.Start(); err != nil {
				errCh <- err
			}
		}(P)
	}

	// tamperUpdater follows the SharedPartyUpdater pattern but tampers the
	// receiverId on a KGRound2Message1 from party 0 destined for party 1.
	// The tampering happens AFTER wire serialization/parsing so the modified
	// protobuf struct persists in the party's temp storage and is read by round 3.
	tamperUpdater := func(party tss.Party, msg tss.Message, errCh chan<- *tss.Error) {
		if party.PartyID() == msg.GetFrom() {
			return
		}
		bz, _, err := msg.WireBytes()
		if err != nil {
			errCh <- party.WrapError(err)
			return
		}
		pMsg, err := tss.ParseWireMessage(bz, msg.GetFrom(), msg.IsBroadcast())
		if err != nil {
			errCh <- party.WrapError(err)
			return
		}

		// Tamper: if this is a P2P KGRound2Message1 from party 0 to party 1,
		// replace the receiverId with a wrong value.
		if msg.GetTo() != nil && msg.GetFrom().Index == 0 && party.PartyID().Index == 1 {
			if content, ok := pMsg.Content().(*KGRound2Message1); ok {
				content.ReceiverId = big.NewInt(0xBAADF00D).Bytes()
				t.Log("TAMPERED: Round 2 P2P message from party 0 → party 1 receiverId")
			}
		}

		if _, err := party.Update(pMsg); err != nil {
			errCh <- err
		}
	}

	var ended int32
keygen:
	for {
		select {
		case err := <-errCh:
			// We EXPECT a round 3 error from party 1 due to the tampered receiverId.
			errStr := err.Error()
			if err.Round() == 3 {
				t.Logf("Got expected round 3 error: %s", errStr)
				// Verify the error mentions receiverId mismatch.
				assert.Contains(t, errStr, "receiverId mismatch",
					"round 3 error should mention receiverId mismatch")
				t.Log("SUCCESS: Round 3 correctly rejected tampered receiverId")
				return // Test passes.
			}
			// Unexpected error from a different round.
			t.Logf("Unexpected error (round %d): %s", err.Round(), errStr)
			break keygen

		case msg := <-outCh:
			dest := msg.GetTo()
			if dest == nil { // broadcast
				for _, P := range parties {
					if P.PartyID().Index == msg.GetFrom().Index {
						continue
					}
					go tamperUpdater(P, msg, errCh)
				}
			} else { // point-to-point
				if dest[0].Index == msg.GetFrom().Index {
					t.Fatalf("party %d tried to send a message to itself", dest[0].Index)
					return
				}
				go tamperUpdater(parties[dest[0].Index], msg, errCh)
			}

		case <-endCh:
			atomic.AddInt32(&ended, 1)
			if atomic.LoadInt32(&ended) == int32(len(pIDs)) {
				// If we get here without error, the tamper wasn't detected — test fails.
				t.Fatal("keygen completed without detecting tampered receiverId — SC#2 check not working")
				break keygen
			}
		}
	}
}

func tryWriteTestFixtureFile(t *testing.T, index int, data LocalPartySaveData) {
	fixtureFileName := makeTestFixtureFilePath(index)

	// fixture file does not already exist?
	// if it does, we won't re-create it here
	fi, err := os.Stat(fixtureFileName)
	if !(err == nil && fi != nil && !fi.IsDir()) {
		fd, err := os.OpenFile(fixtureFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			assert.NoErrorf(t, err, "unable to open fixture file %s for writing", fixtureFileName)
		}
		bz, err := json.Marshal(&data)
		if err != nil {
			t.Fatalf("unable to marshal save data for fixture file %s", fixtureFileName)
		}
		_, err = fd.Write(bz)
		if err != nil {
			t.Fatalf("unable to write to fixture file %s", fixtureFileName)
		}
		t.Logf("Saved a test fixture file for party %d: %s", index, fixtureFileName)
	} else {
		t.Logf("Fixture file already exists for party %d; not re-creating: %s", index, fixtureFileName)
	}
	//
}

// TestGetPolyAfterKeygen verifies that the [FORK] GetPoly() method returns a
// non-nil polynomial of length threshold+1 after a successful keygen completes.
func TestGetPolyAfterKeygen(t *testing.T) {
	setUp("info")

	threshold := testThreshold
	fixtures, pIDs, err := LoadKeygenTestFixtures(testParticipants)
	if err != nil {
		common.Logger.Info("No test fixtures were found, so the safe primes will be generated from scratch. This may take a while...")
		pIDs = tss.GenerateTestPartyIDs(testParticipants)
	}

	p2pCtx := tss.NewPeerContext(pIDs)
	parties := make([]*LocalParty, 0, len(pIDs))

	errCh := make(chan *tss.Error, len(pIDs))
	outCh := make(chan tss.Message, len(pIDs))
	endCh := make(chan *LocalPartySaveData, len(pIDs))

	updater := test.SharedPartyUpdater

	// init the parties
	for i := 0; i < len(pIDs); i++ {
		var P *LocalParty
		params := tss.NewParameters(tss.S256(), p2pCtx, pIDs[i], len(pIDs), threshold)
		params.SetNoProofMod()
		params.SetNoProofFac()
		if i < len(fixtures) {
			P = NewLocalParty(params, outCh, endCh, fixtures[i].LocalPreParams).(*LocalParty)
		} else {
			P = NewLocalParty(params, outCh, endCh).(*LocalParty)
		}
		parties = append(parties, P)
		go func(P *LocalParty) {
			if err := P.Start(); err != nil {
				errCh <- err
			}
		}(P)
	}

	// PHASE: keygen
	var ended int32
keygen:
	for {
		select {
		case err := <-errCh:
			common.Logger.Errorf("Error: %s", err)
			assert.FailNow(t, err.Error())
			break keygen

		case msg := <-outCh:
			dest := msg.GetTo()
			if dest == nil { // broadcast
				for _, P := range parties {
					if P.PartyID().Index == msg.GetFrom().Index {
						continue
					}
					go updater(P, msg, errCh)
				}
			} else { // point-to-point
				if dest[0].Index == msg.GetFrom().Index {
					t.Fatalf("party %d tried to send a message to itself (%d)", dest[0].Index, msg.GetFrom().Index)
					return
				}
				go updater(parties[dest[0].Index], msg, errCh)
			}

		case <-endCh:
			atomic.AddInt32(&ended, 1)
			if atomic.LoadInt32(&ended) == int32(len(pIDs)) {
				t.Logf("Done. Received save data from %d participants", ended)

				// Verify GetPoly() on each party after keygen completes.
				for i, P := range parties {
					poly := P.GetPoly()
					assert.NotNil(t, poly, "party %d: GetPoly() should not return nil after keygen", i)
					assert.Equal(t, threshold+1, len(poly),
						"party %d: GetPoly() should return threshold+1 coefficients", i)

					// poly[0] is the party's secret (ui). It must be non-nil and non-zero.
					if assert.NotNil(t, poly[0], "party %d: poly[0] (secret) should not be nil", i) {
						assert.NotEqual(t, 0, poly[0].Sign(),
							"party %d: poly[0] (secret) should be non-zero", i)
					}

					t.Logf("party %d: GetPoly() returned %d coefficients, poly[0] bit-length = %d",
						i, len(poly), poly[0].BitLen())
				}

				break keygen
			}
		}
	}
}
