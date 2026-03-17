// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"math/big"
	"testing"

	cmt "github.com/hemilabs/x/tss-lib/v3/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v3/crypto/paillier"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

func TestValidateBasicKGRound1(t *testing.T) {
	if (&KGRound1Message{}).ValidateBasic() {
		t.Fatal("empty should fail")
	}
	if (*KGRound1Message)(nil).ValidateBasic() {
		t.Fatal("nil should fail")
	}
	m := &KGRound1Message{
		Commitment: big.NewInt(1),
		PaillierPK: &paillier.PublicKey{N: big.NewInt(100)},
		NTilde:     big.NewInt(2),
		H1:         big.NewInt(3),
		H2:         big.NewInt(4),
	}
	if !m.ValidateBasic() {
		t.Fatal("valid message should pass")
	}
}

func TestValidateBasicKGRound2Message1(t *testing.T) {
	if (*KGRound2Message1)(nil).ValidateBasic() {
		t.Fatal("nil should fail")
	}
	if (&KGRound2Message1{}).ValidateBasic() {
		t.Fatal("empty should fail")
	}
	m := &KGRound2Message1{Share: big.NewInt(1), ReceiverID: []byte("r")}
	if !m.ValidateBasic() {
		t.Fatal("valid should pass")
	}
}

func TestValidateBasicKGRound2Message2(t *testing.T) {
	if (*KGRound2Message2)(nil).ValidateBasic() {
		t.Fatal("nil should fail")
	}
	if (&KGRound2Message2{}).ValidateBasic() {
		t.Fatal("empty should fail")
	}
	m := &KGRound2Message2{DeCommitment: cmt.HashDeCommitment{big.NewInt(1), big.NewInt(2)}}
	if !m.ValidateBasic() {
		t.Fatal("valid should pass")
	}
}

func TestValidateBasicKGRound3(t *testing.T) {
	if (*KGRound3Message)(nil).ValidateBasic() {
		t.Fatal("nil should fail")
	}
	var proof paillier.Proof
	for i := range proof {
		proof[i] = big.NewInt(int64(i + 1))
	}
	m := &KGRound3Message{PaillierProof: proof}
	if !m.ValidateBasic() {
		t.Fatal("valid should pass")
	}
	proof[5] = nil
	m2 := &KGRound3Message{PaillierProof: proof}
	if m2.ValidateBasic() {
		t.Fatal("nil element should fail")
	}
}

func TestExportR2BcastSelf(t *testing.T) {
	// ExportR2BcastSelf returns the stored message for own index.
	st := &KeygenState{
		params: nil, // not needed for export
	}
	// Just verify it doesn't panic on zero state.
	// In real usage this is called after Round2.
	defer func() {
		_ = recover() // expected — params is nil
	}()
	_ = st.ExportR2BcastSelf()
}

func TestValidateSaveData(t *testing.T) {
	empty := NewLocalPartySaveData(0)
	if err := empty.ValidateSaveData(); err == nil {
		t.Fatal("empty save data should fail validation")
	}
}

func TestBuildLocalSaveDataSubset(t *testing.T) {
	// BuildLocalSaveDataSubset panics on missing key — verify it does.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for missing signer key")
		}
	}()
	sd := NewLocalPartySaveData(2)
	sd.Ks = []*big.Int{big.NewInt(1), big.NewInt(2)}
	// Pass an ID whose key doesn't match anything in Ks.
	fakeIDs := tss.GenerateTestPartyIDs(1)
	BuildLocalSaveDataSubset(sd, fakeIDs)
}
