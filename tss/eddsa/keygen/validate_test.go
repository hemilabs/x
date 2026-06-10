// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss/v3/crypto"
	cmt "github.com/hemilabs/x/tss/v3/crypto/commitments"
	"github.com/hemilabs/x/tss/v3/crypto/schnorr"
	"github.com/hemilabs/x/tss/v3/tss"
)

// --- ValidateBasic ---

func TestKGRound1MessageValidateBasic(t *testing.T) {
	if (&KGRound1Message{}).ValidateBasic() {
		t.Fatal("zero-value should fail")
	}
	if (&KGRound1Message{Commitment: big.NewInt(0)}).ValidateBasic() {
		t.Fatal("zero commitment should fail")
	}
	if (*KGRound1Message)(nil).ValidateBasic() {
		t.Fatal("nil should fail")
	}
	if !(&KGRound1Message{Commitment: big.NewInt(42)}).ValidateBasic() {
		t.Fatal("valid should pass")
	}
}

func TestKGRound2Message1ValidateBasic(t *testing.T) {
	if (*KGRound2Message1)(nil).ValidateBasic() {
		t.Fatal("nil should fail")
	}
	if (&KGRound2Message1{}).ValidateBasic() {
		t.Fatal("zero-value should fail")
	}
	if (&KGRound2Message1{Share: big.NewInt(1)}).ValidateBasic() {
		t.Fatal("missing ReceiverID should fail")
	}
	if !(&KGRound2Message1{Share: big.NewInt(1), ReceiverID: []byte("x")}).ValidateBasic() {
		t.Fatal("valid should pass")
	}
}

func TestKGRound2Message2ValidateBasic(t *testing.T) {
	ec := tss.Edwards()
	alpha := crypto.ScalarBaseMult(ec, big.NewInt(7))
	proof := &schnorr.ZKProof{Alpha: alpha, T: big.NewInt(99)}

	if (*KGRound2Message2)(nil).ValidateBasic() {
		t.Fatal("nil should fail")
	}
	if (&KGRound2Message2{}).ValidateBasic() {
		t.Fatal("zero-value should fail")
	}
	if (&KGRound2Message2{DeCommitment: cmt.HashDeCommitment{big.NewInt(1)}}).ValidateBasic() {
		t.Fatal("short decommitment should fail")
	}
	if !(&KGRound2Message2{
		DeCommitment: cmt.HashDeCommitment{big.NewInt(1), big.NewInt(2)},
		ZKProof:      proof,
	}).ValidateBasic() {
		t.Fatal("valid should pass")
	}
}

// --- SaveData ---

func TestValidateSaveDataNilFields(t *testing.T) {
	sd := LocalPartySaveData{}
	if err := sd.ValidateSaveData(); err == nil {
		t.Fatal("empty save data should fail")
	}
}

func TestValidateSaveDataTooFewParties(t *testing.T) {
	ec := tss.Edwards()
	pt := crypto.ScalarBaseMult(ec, big.NewInt(42))
	sd := LocalPartySaveData{
		LocalSecrets: LocalSecrets{Xi: big.NewInt(1), ShareID: big.NewInt(1)},
		EDDSAPub:     pt,
		Ks:           []*big.Int{big.NewInt(1)}, // < 2
		BigXj:        []*crypto.ECPoint{pt},
	}
	if err := sd.ValidateSaveData(); err == nil {
		t.Fatal("party count < 2 should fail")
	}
}

func TestBuildLocalSaveDataSubset(t *testing.T) {
	// Run a keygen to get real save data, then test subset
	const n = 3
	const threshold = 1
	pIDs := tss.GenerateTestPartyIDs(n)
	peerCtx := tss.NewPeerContext(pIDs)

	states := make([]*KeygenState, n)
	r1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.Edwards(), peerCtx, pIDs[i], n, threshold)
		st, out, err := Round1(params)
		if err != nil {
			t.Fatalf("Round1[%d]: %v", i, err)
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
		out, err := Round2(states[i], r1)
		if err != nil {
			t.Fatalf("Round2[%d]: %v", i, err)
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
	var fullSave *LocalPartySaveData
	for i := 0; i < n; i++ {
		out, err := Round3(states[i], r2p2p[i], r2bcast)
		if err != nil {
			t.Fatalf("Round3[%d]: %v", i, err)
		}
		if i == 0 {
			fullSave = out.Save
		}
	}

	// Take subset of 2 parties
	subset := tss.SortPartyIDs(tss.UnSortedPartyIDs{pIDs[0], pIDs[1]})
	subData := BuildLocalSaveDataSubset(*fullSave, subset)
	if len(subData.Ks) != 2 {
		t.Fatalf("subset Ks: want 2, got %d", len(subData.Ks))
	}
	if subData.EDDSAPub == nil {
		t.Fatal("subset should preserve EDDSAPub")
	}
}

// --- ExportR2BcastSelf ---

func TestExportR2BcastSelf(t *testing.T) {
	pIDs := tss.GenerateTestPartyIDs(3)
	peerCtx := tss.NewPeerContext(pIDs)
	params := tss.NewParameters(tss.Edwards(), peerCtx, pIDs[0], 3, 1)
	st, _, err := Round1(params)
	if err != nil {
		t.Fatal(err)
	}
	r1 := make([]*tss.Message, 3)
	for i := 0; i < 3; i++ {
		p := tss.NewParameters(tss.Edwards(), peerCtx, pIDs[i], 3, 1)
		_, out, err := Round1(p)
		if err != nil {
			t.Fatal(err)
		}
		r1[i] = out.Messages[0]
	}
	if _, err := Round2(st, r1); err != nil {
		t.Fatal(err)
	}
	bcast := st.ExportR2BcastSelf()
	if bcast == nil {
		t.Fatal("ExportR2BcastSelf returned nil")
	}
}
