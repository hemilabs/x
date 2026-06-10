// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package resharing

import (
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss/v3/crypto"
	cmt "github.com/hemilabs/x/tss/v3/crypto/commitments"
	"github.com/hemilabs/x/tss/v3/crypto/paillier"
	"github.com/hemilabs/x/tss/v3/tss"
)

func TestValidateBasicAllResharingMessages(t *testing.T) {
	pt := crypto.ScalarBaseMult(tss.S256(), big.NewInt(42))

	tests := []struct {
		name  string
		valid interface{ ValidateBasic() bool }
		bad   interface{ ValidateBasic() bool }
	}{
		{
			"DGRound1Message",
			&DGRound1Message{ECDSAPub: pt, VCommitment: big.NewInt(1), SSID: []byte("s")},
			&DGRound1Message{},
		},
		{
			"DGRound2Message1",
			&DGRound2Message1{PaillierPK: &paillier.PublicKey{N: big.NewInt(100)}, NTilde: big.NewInt(1), H1: big.NewInt(2), H2: big.NewInt(3)},
			&DGRound2Message1{},
		},
		{
			"DGRound2Message2",
			&DGRound2Message2{},
			(*DGRound2Message2)(nil),
		},
		{
			"DGRound3Message1",
			&DGRound3Message1{Share: big.NewInt(1), ReceiverID: []byte("r")},
			&DGRound3Message1{},
		},
		{
			"DGRound3Message2",
			&DGRound3Message2{VDeCommitment: cmt.HashDeCommitment{big.NewInt(1), big.NewInt(2)}},
			&DGRound3Message2{},
		},
		{
			"DGRound4Message1",
			&DGRound4Message1{ReceiverID: []byte("r")},
			&DGRound4Message1{},
		},
		{
			"DGRound4Message2",
			&DGRound4Message2{},
			(*DGRound4Message2)(nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_valid", func(t *testing.T) {
			if !tt.valid.ValidateBasic() {
				t.Fatal("valid should pass")
			}
		})
		t.Run(tt.name+"_invalid", func(t *testing.T) {
			if tt.bad.ValidateBasic() {
				t.Fatal("invalid should fail")
			}
		})
	}
}
