// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package signing

import (
	"math/big"
	"testing"

	cmt "github.com/hemilabs/x/tss/v3/crypto/commitments"
	"github.com/hemilabs/x/tss/v3/crypto/mta"
	"github.com/hemilabs/x/tss/v3/crypto/schnorr"
)

func TestValidateBasicAllSignMessages(t *testing.T) {
	pt := &schnorr.ZKProof{Alpha: nil}
	vpt := &schnorr.ZKVProof{Alpha: nil}

	tests := []struct {
		name  string
		valid interface{ ValidateBasic() bool }
		bad   interface{ ValidateBasic() bool }
	}{
		{
			"SignRound1Message1",
			&SignRound1Message1{C: big.NewInt(1), RangeProofAlice: &mta.RangeProofAlice{}, ReceiverID: []byte("r")},
			&SignRound1Message1{},
		},
		{
			"SignRound1Message2",
			&SignRound1Message2{Commitment: big.NewInt(1)},
			&SignRound1Message2{},
		},
		{
			"SignRound2Message",
			&SignRound2Message{C1: big.NewInt(1), C2: big.NewInt(2), ProofBob: &mta.ProofBob{}, ProofBobWC: &mta.ProofBobWC{}, ReceiverID: []byte("r")},
			&SignRound2Message{},
		},
		{
			"SignRound3Message",
			&SignRound3Message{Theta: big.NewInt(1)},
			&SignRound3Message{},
		},
		{
			"SignRound4Message",
			&SignRound4Message{DeCommitment: cmt.HashDeCommitment{big.NewInt(1), big.NewInt(2)}, ZKProof: pt},
			&SignRound4Message{},
		},
		{
			"SignRound5Message",
			&SignRound5Message{Commitment: big.NewInt(1)},
			&SignRound5Message{},
		},
		{
			"SignRound6Message",
			&SignRound6Message{DeCommitment: cmt.HashDeCommitment{big.NewInt(1), big.NewInt(2)}, ZKProof: pt, ZKVProof: vpt},
			&SignRound6Message{},
		},
		{
			"SignRound7Message",
			&SignRound7Message{Commitment: big.NewInt(1)},
			&SignRound7Message{},
		},
		{
			"SignRound8Message",
			&SignRound8Message{DeCommitment: cmt.HashDeCommitment{big.NewInt(1), big.NewInt(2)}},
			&SignRound8Message{},
		},
		{
			"SignRound9Message",
			&SignRound9Message{S: big.NewInt(1)},
			&SignRound9Message{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_valid", func(t *testing.T) {
			if !tt.valid.ValidateBasic() {
				t.Fatal("valid message should pass")
			}
		})
		t.Run(tt.name+"_invalid", func(t *testing.T) {
			if tt.bad.ValidateBasic() {
				t.Fatal("empty message should fail")
			}
		})
	}
}
