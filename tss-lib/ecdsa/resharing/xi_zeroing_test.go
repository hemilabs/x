// Copyright (c) 2024 Hemi Labs, Inc.
//
// This file is part of the hemi tss-lib fork. See LICENSE for terms.

package resharing

import (
	"fmt"
	"math/big"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/ecdsa/keygen"
	"github.com/hemilabs/x/tss-lib/v2/test"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

// TestResharingZerosOldCommitteeXi runs the full ECDSA resharing protocol and
// verifies that old committee parties' input.Xi is zeroed after completion.
// This exercises the [FORK] fix in round_5_new_step_3.go that unconditionally
// zeros old Xi regardless of dual-committee membership.
func TestResharingZerosOldCommitteeXi(t *testing.T) {
	threshold, newThreshold := test.TestThreshold, test.TestThreshold

	// PHASE: load keygen fixtures
	firstPartyIdx, extraParties := 1, 1
	oldKeys, oldPIDs, err := keygen.LoadKeygenTestFixtures(test.TestThreshold+1+extraParties+firstPartyIdx, firstPartyIdx)
	assert.NoError(t, err, "should load keygen fixtures")

	// PHASE: resharing
	oldP2PCtx := tss.NewPeerContext(oldPIDs)
	fixtures, _, err := keygen.LoadKeygenTestFixtures(test.TestParticipants)
	if err != nil {
		common.Logger.Info("No test fixtures were found, so the safe primes will be generated from scratch. This may take a while...")
	}
	newPIDs := tss.GenerateTestPartyIDs(test.TestParticipants)
	newP2PCtx := tss.NewPeerContext(newPIDs)
	newPCount := len(newPIDs)

	oldCommittee := make([]*LocalParty, 0, len(oldPIDs))
	newCommittee := make([]*LocalParty, 0, newPCount)
	bothCommitteesPax := len(oldPIDs) + newPCount

	errCh := make(chan *tss.Error, bothCommitteesPax)
	outCh := make(chan tss.Message, bothCommitteesPax)
	endCh := make(chan *keygen.LocalPartySaveData, bothCommitteesPax)

	updater := test.SharedPartyUpdater

	// Record old Xi values before resharing starts, to verify they are non-zero.
	oldXiValues := make([]*big.Int, len(oldPIDs))

	// init the old parties first
	for j, pID := range oldPIDs {
		params := tss.NewReSharingParameters(tss.S256(), oldP2PCtx, newP2PCtx, pID, test.TestParticipants, threshold, newPCount, newThreshold)
		P := NewLocalParty(params, oldKeys[j], outCh, endCh).(*LocalParty)
		oldCommittee = append(oldCommittee, P)
		// Save a copy of the original Xi for later comparison.
		oldXiValues[j] = new(big.Int).Set(P.input.Xi)
	}
	// init the new parties
	for j, pID := range newPIDs {
		params := tss.NewReSharingParameters(tss.S256(), oldP2PCtx, newP2PCtx, pID, test.TestParticipants, threshold, newPCount, newThreshold)
		params.SetNoProofMod()
		params.SetNoProofFac()
		save := keygen.NewLocalPartySaveData(newPCount)
		if j < len(fixtures) && len(newPIDs) <= len(fixtures) {
			save.LocalPreParams = fixtures[j].LocalPreParams
		}
		P := NewLocalParty(params, save, outCh, endCh).(*LocalParty)
		newCommittee = append(newCommittee, P)
	}

	// Verify old Xi values are non-zero before starting.
	for j, xi := range oldXiValues {
		assert.NotEqual(t, 0, xi.Sign(), "old party %d: Xi should be non-zero before resharing", j)
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

		case <-endCh:
			atomic.AddInt32(&reSharingEnded, 1)
			if atomic.LoadInt32(&reSharingEnded) == int32(len(oldCommittee)+len(newCommittee)) {
				t.Logf("Resharing done. Verifying Xi zeroing on %d old committee parties", len(oldCommittee))

				// ASSERTION: every old committee party's input.Xi must now be zero.
				for j, P := range oldCommittee {
					assert.Equalf(t, 0, P.input.Xi.Sign(),
						"old party %d: input.Xi should be zeroed after resharing (was %s)",
						j, oldXiValues[j].String())
				}
				t.Log("Xi zeroing verification passed for all old committee parties.")
				return
			}
		}
	}
}
