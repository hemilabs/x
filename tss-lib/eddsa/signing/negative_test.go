// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package signing

import (
	"crypto/sha256"
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss-lib/v3/crypto/schnorr"
	"github.com/hemilabs/x/tss-lib/v3/eddsa/keygen"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// runEdDSAKeygen performs a 3-party keygen for signing tests.
func runEdDSAKeygen(t *testing.T) ([]keygen.LocalPartySaveData, tss.SortedPartyIDs) {
	t.Helper()
	const n = 3
	const threshold = 1

	pIDs := tss.GenerateTestPartyIDs(n)
	peerCtx := tss.NewPeerContext(pIDs)

	states := make([]*keygen.KeygenState, n)
	r1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.Edwards(), peerCtx, pIDs[i], n, threshold)
		st, out, err := keygen.Round1(params)
		if err != nil {
			t.Fatalf("keygen Round1[%d]: %v", i, err)
		}
		states[i] = st
		r1[i] = out.Messages[0]
	}

	r2p2p := make([][]*tss.Message, n)
	r2bcast := make([]*tss.Message, n)
	for i := range r2p2p {
		r2p2p[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := keygen.Round2(states[i], r1)
		if err != nil {
			t.Fatalf("keygen Round2[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			if msg.To == nil {
				r2bcast[i] = msg
			} else {
				for _, to := range msg.To {
					r2p2p[to.Index][i] = msg
				}
			}
		}
		r2p2p[i][i] = states[i].ExportR2P2PSelf()
		if r2bcast[i] == nil {
			r2bcast[i] = states[i].ExportR2BcastSelf()
		}
	}

	saves := make([]keygen.LocalPartySaveData, n)
	for i := 0; i < n; i++ {
		out, err := keygen.Round3(states[i], r2p2p[i], r2bcast)
		if err != nil {
			t.Fatalf("keygen Round3[%d]: %v", i, err)
		}
		saves[i] = *out.Save
	}
	return saves, pIDs
}

// --- SignRound1 error paths ---

func TestSignRound1NilXi(t *testing.T) {
	saves, pIDs := runEdDSAKeygen(t)
	peerCtx := tss.NewPeerContext(pIDs)
	params := tss.NewParameters(tss.Edwards(), peerCtx, pIDs[0], 3, 1)

	bad := saves[0]
	bad.Xi = nil
	_, _, err := SignRound1(params, bad, big.NewInt(42), 0)
	if err == nil {
		t.Fatal("expected error for nil Xi")
	}
}

func TestSignRound1NilEDDSAPub(t *testing.T) {
	saves, pIDs := runEdDSAKeygen(t)
	peerCtx := tss.NewPeerContext(pIDs)
	params := tss.NewParameters(tss.Edwards(), peerCtx, pIDs[0], 3, 1)

	bad := saves[0]
	bad.EDDSAPub = nil
	_, _, err := SignRound1(params, bad, big.NewInt(42), 0)
	if err == nil {
		t.Fatal("expected error for nil EDDSAPub")
	}
}

func TestSignRound1WrongKeyCount(t *testing.T) {
	saves, pIDs := runEdDSAKeygen(t)
	peerCtx := tss.NewPeerContext(pIDs)
	params := tss.NewParameters(tss.Edwards(), peerCtx, pIDs[0], 3, 1)

	bad := saves[0]
	bad.Ks = bad.Ks[:1] // wrong count
	_, _, err := SignRound1(params, bad, big.NewInt(42), 0)
	if err == nil {
		t.Fatal("expected error for wrong key count")
	}
}

// --- SignRound2 error paths ---

func TestSignRound2InvalidR1(t *testing.T) {
	saves, pIDs := runEdDSAKeygen(t)
	peerCtx := tss.NewPeerContext(pIDs)

	msg := sha256.Sum256([]byte("test"))
	m := new(big.Int).SetBytes(msg[:])

	states := make([]*SigningState, 3)
	r1 := make([]*tss.Message, 3)
	for i := 0; i < 3; i++ {
		params := tss.NewParameters(tss.Edwards(), peerCtx, pIDs[i], 3, 1)
		st, out, err := SignRound1(params, saves[i], m, 0)
		if err != nil {
			t.Fatalf("SignRound1[%d]: %v", i, err)
		}
		states[i] = st
		r1[i] = out.Messages[0]
	}

	// Corrupt r1[1]
	badR1 := make([]*tss.Message, 3)
	copy(badR1, r1)
	badR1[1] = &tss.Message{
		From:    r1[1].From,
		Content: &SignRound1Message{Commitment: nil},
	}

	_, err := SignRound2(states[0], badR1)
	if err == nil {
		t.Fatal("expected error for invalid round 1 message")
	}
}

// --- SignRound3 error paths ---

func runToSignRound3(t *testing.T) ([]*SigningState, []*tss.Message, tss.SortedPartyIDs) {
	t.Helper()
	saves, pIDs := runEdDSAKeygen(t)
	peerCtx := tss.NewPeerContext(pIDs)
	msg := sha256.Sum256([]byte("test"))
	m := new(big.Int).SetBytes(msg[:])

	states := make([]*SigningState, 3)
	r1 := make([]*tss.Message, 3)
	for i := 0; i < 3; i++ {
		params := tss.NewParameters(tss.Edwards(), peerCtx, pIDs[i], 3, 1)
		st, out, err := SignRound1(params, saves[i], m, 0)
		if err != nil {
			t.Fatalf("SignRound1[%d]: %v", i, err)
		}
		states[i] = st
		r1[i] = out.Messages[0]
	}

	r2 := make([]*tss.Message, 3)
	for i := 0; i < 3; i++ {
		out, err := SignRound2(states[i], r1)
		if err != nil {
			t.Fatalf("SignRound2[%d]: %v", i, err)
		}
		r2[i] = out.Messages[0]
	}
	return states, r2, pIDs
}

func TestSignRound3BadDeCommitment(t *testing.T) {
	states, r2, _ := runToSignRound3(t)

	badR2 := make([]*tss.Message, 3)
	copy(badR2, r2)
	badContent := *r2[1].Content.(*SignRound2Message)
	badContent.DeCommitment = nil
	badR2[1] = &tss.Message{From: r2[1].From, Content: &badContent}

	_, err := SignRound3(states[0], badR2)
	if err == nil {
		t.Fatal("expected error for bad decommitment")
	}
}

func TestSignRound3MissingProof(t *testing.T) {
	states, r2, _ := runToSignRound3(t)

	badR2 := make([]*tss.Message, 3)
	copy(badR2, r2)
	badContent := *r2[1].Content.(*SignRound2Message)
	badContent.ZKProof = nil
	badR2[1] = &tss.Message{From: r2[1].From, Content: &badContent}

	_, err := SignRound3(states[0], badR2)
	if err == nil {
		t.Fatal("expected error for missing proof")
	}
}

func TestSignRound3WrongProof(t *testing.T) {
	states, r2, _ := runToSignRound3(t)

	badR2 := make([]*tss.Message, 3)
	copy(badR2, r2)
	badContent := *r2[1].Content.(*SignRound2Message)
	badContent.ZKProof = &schnorr.ZKProof{Alpha: nil, T: big.NewInt(99)}
	badR2[1] = &tss.Message{From: r2[1].From, Content: &badContent}

	_, err := SignRound3(states[0], badR2)
	if err == nil {
		t.Fatal("expected error for wrong proof")
	}
}

// --- SignFinalize error paths ---

func TestSignFinalizeOutOfRangeS(t *testing.T) {
	states, r2, _ := runToSignRound3(t)

	r3 := make([]*tss.Message, 3)
	for i := 0; i < 3; i++ {
		out, err := SignRound3(states[i], r2)
		if err != nil {
			t.Fatalf("SignRound3[%d]: %v", i, err)
		}
		r3[i] = out.Messages[0]
	}

	// Corrupt s value: set to -1 (out of range)
	badR3 := make([]*tss.Message, 3)
	copy(badR3, r3)
	badR3[1] = &tss.Message{
		From:    r3[1].From,
		Content: &SignRound3Message{S: big.NewInt(-1)},
	}

	_, err := SignFinalize(states[0], badR3)
	if err == nil {
		t.Fatal("expected error for out-of-range s")
	}
}
