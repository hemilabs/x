// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package signing

import (
	"math/big"
	"testing"

	cmt "github.com/hemilabs/x/tss-lib/v3/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v3/crypto/mta"
	"github.com/hemilabs/x/tss-lib/v3/crypto/schnorr"
)

// validR1M1 returns a SignRound1Message1 that passes ValidateBasic.
func validR1M1() *SignRound1Message1 {
	return &SignRound1Message1{
		C:               big.NewInt(42),
		RangeProofAlice: &mta.RangeProofAlice{},
		ReceiverID:      []byte("receiver"),
	}
}

func TestSignRound1Message1_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  *SignRound1Message1
		want bool
	}{
		{"valid", validR1M1(), true},
		{"nil receiver", nil, false},
		{"C nil", func() *SignRound1Message1 {
			m := validR1M1()
			m.C = nil
			return m
		}(), false},
		{"C zero", func() *SignRound1Message1 {
			m := validR1M1()
			m.C = big.NewInt(0)
			return m
		}(), false},
		{"C negative", func() *SignRound1Message1 {
			m := validR1M1()
			m.C = big.NewInt(-1)
			return m
		}(), false},
		{"RangeProofAlice nil", func() *SignRound1Message1 {
			m := validR1M1()
			m.RangeProofAlice = nil
			return m
		}(), false},
		{"ReceiverID nil", func() *SignRound1Message1 {
			m := validR1M1()
			m.ReceiverID = nil
			return m
		}(), false},
		{"ReceiverID empty", func() *SignRound1Message1 {
			m := validR1M1()
			m.ReceiverID = []byte{}
			return m
		}(), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.ValidateBasic(); got != tt.want {
				t.Errorf("ValidateBasic() = %v, want %v", got, tt.want)
			}
		})
	}
}

// validR1M2 returns a SignRound1Message2 that passes ValidateBasic.
func validR1M2() *SignRound1Message2 {
	return &SignRound1Message2{Commitment: big.NewInt(99)}
}

func TestSignRound1Message2_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  *SignRound1Message2
		want bool
	}{
		{"valid", validR1M2(), true},
		{"nil receiver", nil, false},
		{"Commitment nil", &SignRound1Message2{Commitment: nil}, false},
		{"Commitment zero", &SignRound1Message2{Commitment: big.NewInt(0)}, false},
		{"Commitment negative", &SignRound1Message2{Commitment: big.NewInt(-5)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.ValidateBasic(); got != tt.want {
				t.Errorf("ValidateBasic() = %v, want %v", got, tt.want)
			}
		})
	}
}

// validR2 returns a SignRound2Message that passes ValidateBasic.
func validR2() *SignRound2Message {
	return &SignRound2Message{
		C1:         big.NewInt(10),
		C2:         big.NewInt(20),
		ProofBob:   &mta.ProofBob{},
		ProofBobWC: &mta.ProofBobWC{},
		ReceiverID: []byte("recv"),
	}
}

func TestSignRound2Message_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  *SignRound2Message
		want bool
	}{
		{"valid", validR2(), true},
		{"nil receiver", nil, false},
		{"C1 nil", func() *SignRound2Message {
			m := validR2()
			m.C1 = nil
			return m
		}(), false},
		{"C1 zero", func() *SignRound2Message {
			m := validR2()
			m.C1 = big.NewInt(0)
			return m
		}(), false},
		{"C1 negative", func() *SignRound2Message {
			m := validR2()
			m.C1 = big.NewInt(-3)
			return m
		}(), false},
		{"C2 nil", func() *SignRound2Message {
			m := validR2()
			m.C2 = nil
			return m
		}(), false},
		{"C2 zero", func() *SignRound2Message {
			m := validR2()
			m.C2 = big.NewInt(0)
			return m
		}(), false},
		{"C2 negative", func() *SignRound2Message {
			m := validR2()
			m.C2 = big.NewInt(-7)
			return m
		}(), false},
		{"ProofBob nil", func() *SignRound2Message {
			m := validR2()
			m.ProofBob = nil
			return m
		}(), false},
		{"ProofBobWC nil", func() *SignRound2Message {
			m := validR2()
			m.ProofBobWC = nil
			return m
		}(), false},
		{"ReceiverID nil", func() *SignRound2Message {
			m := validR2()
			m.ReceiverID = nil
			return m
		}(), false},
		{"ReceiverID empty", func() *SignRound2Message {
			m := validR2()
			m.ReceiverID = []byte{}
			return m
		}(), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.ValidateBasic(); got != tt.want {
				t.Errorf("ValidateBasic() = %v, want %v", got, tt.want)
			}
		})
	}
}

// validR3 returns a SignRound3Message that passes ValidateBasic.
func validR3() *SignRound3Message {
	return &SignRound3Message{Theta: big.NewInt(77)}
}

func TestSignRound3Message_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  *SignRound3Message
		want bool
	}{
		{"valid", validR3(), true},
		{"nil receiver", nil, false},
		{"Theta nil", &SignRound3Message{Theta: nil}, false},
		{"Theta zero", &SignRound3Message{Theta: big.NewInt(0)}, false},
		{"Theta negative", &SignRound3Message{Theta: big.NewInt(-1)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.ValidateBasic(); got != tt.want {
				t.Errorf("ValidateBasic() = %v, want %v", got, tt.want)
			}
		})
	}
}

// validR4 returns a SignRound4Message that passes ValidateBasic.
func validR4() *SignRound4Message {
	return &SignRound4Message{
		DeCommitment: cmt.HashDeCommitment{big.NewInt(1), big.NewInt(2)},
		ZKProof:      &schnorr.ZKProof{},
	}
}

func TestSignRound4Message_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  *SignRound4Message
		want bool
	}{
		{"valid with 2 elements", validR4(), true},
		{"valid with 3 elements", &SignRound4Message{
			DeCommitment: cmt.HashDeCommitment{big.NewInt(1), big.NewInt(2), big.NewInt(3)},
			ZKProof:      &schnorr.ZKProof{},
		}, true},
		{"nil receiver", nil, false},
		{"DeCommitment nil", &SignRound4Message{
			DeCommitment: nil,
			ZKProof:      &schnorr.ZKProof{},
		}, false},
		{"DeCommitment empty", &SignRound4Message{
			DeCommitment: cmt.HashDeCommitment{},
			ZKProof:      &schnorr.ZKProof{},
		}, false},
		{"DeCommitment 1 element (boundary)", &SignRound4Message{
			DeCommitment: cmt.HashDeCommitment{big.NewInt(1)},
			ZKProof:      &schnorr.ZKProof{},
		}, false},
		{"ZKProof nil", &SignRound4Message{
			DeCommitment: cmt.HashDeCommitment{big.NewInt(1), big.NewInt(2)},
			ZKProof:      nil,
		}, false},
		{"both nil", &SignRound4Message{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.ValidateBasic(); got != tt.want {
				t.Errorf("ValidateBasic() = %v, want %v", got, tt.want)
			}
		})
	}
}

// validR5 returns a SignRound5Message that passes ValidateBasic.
func validR5() *SignRound5Message {
	return &SignRound5Message{Commitment: big.NewInt(55)}
}

func TestSignRound5Message_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  *SignRound5Message
		want bool
	}{
		{"valid", validR5(), true},
		{"nil receiver", nil, false},
		{"Commitment nil", &SignRound5Message{Commitment: nil}, false},
		{"Commitment zero", &SignRound5Message{Commitment: big.NewInt(0)}, false},
		{"Commitment negative", &SignRound5Message{Commitment: big.NewInt(-1)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.ValidateBasic(); got != tt.want {
				t.Errorf("ValidateBasic() = %v, want %v", got, tt.want)
			}
		})
	}
}

// validR6 returns a SignRound6Message that passes ValidateBasic.
func validR6() *SignRound6Message {
	return &SignRound6Message{
		DeCommitment: cmt.HashDeCommitment{big.NewInt(1), big.NewInt(2)},
		ZKProof:      &schnorr.ZKProof{},
		ZKVProof:     &schnorr.ZKVProof{},
	}
}

func TestSignRound6Message_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  *SignRound6Message
		want bool
	}{
		{"valid with 2 elements", validR6(), true},
		{"valid with 3 elements", &SignRound6Message{
			DeCommitment: cmt.HashDeCommitment{big.NewInt(1), big.NewInt(2), big.NewInt(3)},
			ZKProof:      &schnorr.ZKProof{},
			ZKVProof:     &schnorr.ZKVProof{},
		}, true},
		{"nil receiver", nil, false},
		{"DeCommitment nil", &SignRound6Message{
			DeCommitment: nil,
			ZKProof:      &schnorr.ZKProof{},
			ZKVProof:     &schnorr.ZKVProof{},
		}, false},
		{"DeCommitment empty", &SignRound6Message{
			DeCommitment: cmt.HashDeCommitment{},
			ZKProof:      &schnorr.ZKProof{},
			ZKVProof:     &schnorr.ZKVProof{},
		}, false},
		{"DeCommitment 1 element (boundary)", &SignRound6Message{
			DeCommitment: cmt.HashDeCommitment{big.NewInt(1)},
			ZKProof:      &schnorr.ZKProof{},
			ZKVProof:     &schnorr.ZKVProof{},
		}, false},
		{"ZKProof nil", &SignRound6Message{
			DeCommitment: cmt.HashDeCommitment{big.NewInt(1), big.NewInt(2)},
			ZKProof:      nil,
			ZKVProof:     &schnorr.ZKVProof{},
		}, false},
		{"ZKVProof nil", &SignRound6Message{
			DeCommitment: cmt.HashDeCommitment{big.NewInt(1), big.NewInt(2)},
			ZKProof:      &schnorr.ZKProof{},
			ZKVProof:     nil,
		}, false},
		{"all fields zero-value", &SignRound6Message{}, false},
		{"only DeCommitment valid", &SignRound6Message{
			DeCommitment: cmt.HashDeCommitment{big.NewInt(1), big.NewInt(2)},
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.ValidateBasic(); got != tt.want {
				t.Errorf("ValidateBasic() = %v, want %v", got, tt.want)
			}
		})
	}
}

// validR7 returns a SignRound7Message that passes ValidateBasic.
func validR7() *SignRound7Message {
	return &SignRound7Message{Commitment: big.NewInt(88)}
}

func TestSignRound7Message_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  *SignRound7Message
		want bool
	}{
		{"valid", validR7(), true},
		{"nil receiver", nil, false},
		{"Commitment nil", &SignRound7Message{Commitment: nil}, false},
		{"Commitment zero", &SignRound7Message{Commitment: big.NewInt(0)}, false},
		{"Commitment negative", &SignRound7Message{Commitment: big.NewInt(-1)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.ValidateBasic(); got != tt.want {
				t.Errorf("ValidateBasic() = %v, want %v", got, tt.want)
			}
		})
	}
}

// validR8 returns a SignRound8Message that passes ValidateBasic.
func validR8() *SignRound8Message {
	return &SignRound8Message{
		DeCommitment: cmt.HashDeCommitment{big.NewInt(1), big.NewInt(2)},
	}
}

func TestSignRound8Message_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  *SignRound8Message
		want bool
	}{
		{"valid with 2 elements", validR8(), true},
		{"valid with 3 elements", &SignRound8Message{
			DeCommitment: cmt.HashDeCommitment{big.NewInt(1), big.NewInt(2), big.NewInt(3)},
		}, true},
		{"nil receiver", nil, false},
		{"DeCommitment nil", &SignRound8Message{DeCommitment: nil}, false},
		{"DeCommitment empty", &SignRound8Message{DeCommitment: cmt.HashDeCommitment{}}, false},
		{"DeCommitment 1 element (boundary)", &SignRound8Message{
			DeCommitment: cmt.HashDeCommitment{big.NewInt(1)},
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.ValidateBasic(); got != tt.want {
				t.Errorf("ValidateBasic() = %v, want %v", got, tt.want)
			}
		})
	}
}

// validR9 returns a SignRound9Message that passes ValidateBasic.
func validR9() *SignRound9Message {
	return &SignRound9Message{S: big.NewInt(33)}
}

func TestSignRound9Message_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  *SignRound9Message
		want bool
	}{
		{"valid", validR9(), true},
		{"nil receiver", nil, false},
		{"S nil", &SignRound9Message{S: nil}, false},
		{"S zero", &SignRound9Message{S: big.NewInt(0)}, false},
		{"S negative", &SignRound9Message{S: big.NewInt(-1)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.ValidateBasic(); got != tt.want {
				t.Errorf("ValidateBasic() = %v, want %v", got, tt.want)
			}
		})
	}
}
