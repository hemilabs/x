// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package resharing

import (
	"math/big"
	"testing"

	"github.com/hemilabs/x/tss-lib/v3/crypto"
	cmt "github.com/hemilabs/x/tss-lib/v3/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v3/crypto/paillier"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// helper returns a valid ECPoint on secp256k1 for use in tests.
func testECPoint() *crypto.ECPoint {
	return crypto.ScalarBaseMult(tss.S256(), big.NewInt(42))
}

// helper returns a valid PaillierPK for use in tests.
func testPaillierPK() *paillier.PublicKey {
	return &paillier.PublicKey{N: big.NewInt(100)}
}

func TestDGRound1Message_ValidateBasic(t *testing.T) {
	t.Parallel()
	valid := &DGRound1Message{
		ECDSAPub:    testECPoint(),
		VCommitment: big.NewInt(1),
		SSID:        []byte("ssid"),
	}

	tests := []struct {
		name string
		msg  *DGRound1Message
		want bool
	}{
		{"valid", valid, true},
		{"nil_receiver", nil, false},
		{"ECDSAPub_nil", &DGRound1Message{
			ECDSAPub:    nil,
			VCommitment: big.NewInt(1),
			SSID:        []byte("ssid"),
		}, false},
		{"VCommitment_nil", &DGRound1Message{
			ECDSAPub:    testECPoint(),
			VCommitment: nil,
			SSID:        []byte("ssid"),
		}, false},
		{"VCommitment_zero", &DGRound1Message{
			ECDSAPub:    testECPoint(),
			VCommitment: big.NewInt(0),
			SSID:        []byte("ssid"),
		}, false},
		{"VCommitment_negative", &DGRound1Message{
			ECDSAPub:    testECPoint(),
			VCommitment: big.NewInt(-1),
			SSID:        []byte("ssid"),
		}, false},
		{"SSID_nil", &DGRound1Message{
			ECDSAPub:    testECPoint(),
			VCommitment: big.NewInt(1),
			SSID:        nil,
		}, false},
		{"SSID_empty", &DGRound1Message{
			ECDSAPub:    testECPoint(),
			VCommitment: big.NewInt(1),
			SSID:        []byte{},
		}, false},
		{"all_zero_value", &DGRound1Message{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.ValidateBasic(); got != tt.want {
				t.Fatalf("ValidateBasic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDGRound2Message1_ValidateBasic(t *testing.T) {
	t.Parallel()
	valid := &DGRound2Message1{
		PaillierPK: testPaillierPK(),
		NTilde:     big.NewInt(7),
		H1:         big.NewInt(3),
		H2:         big.NewInt(5),
	}

	tests := []struct {
		name string
		msg  *DGRound2Message1
		want bool
	}{
		{"valid", valid, true},
		{"valid_with_nil_optional_proofs", &DGRound2Message1{
			PaillierPK: testPaillierPK(),
			NTilde:     big.NewInt(7),
			H1:         big.NewInt(3),
			H2:         big.NewInt(5),
			ModProof:   nil,
			DLNProof1:  nil,
			DLNProof2:  nil,
		}, true},
		{"nil_receiver", nil, false},
		{"PaillierPK_nil", &DGRound2Message1{
			PaillierPK: nil,
			NTilde:     big.NewInt(7),
			H1:         big.NewInt(3),
			H2:         big.NewInt(5),
		}, false},
		{"PaillierPK_N_nil", &DGRound2Message1{
			PaillierPK: &paillier.PublicKey{N: nil},
			NTilde:     big.NewInt(7),
			H1:         big.NewInt(3),
			H2:         big.NewInt(5),
		}, false},
		{"PaillierPK_N_zero", &DGRound2Message1{
			PaillierPK: &paillier.PublicKey{N: big.NewInt(0)},
			NTilde:     big.NewInt(7),
			H1:         big.NewInt(3),
			H2:         big.NewInt(5),
		}, false},
		{"PaillierPK_N_negative", &DGRound2Message1{
			PaillierPK: &paillier.PublicKey{N: big.NewInt(-1)},
			NTilde:     big.NewInt(7),
			H1:         big.NewInt(3),
			H2:         big.NewInt(5),
		}, false},
		{"NTilde_nil", &DGRound2Message1{
			PaillierPK: testPaillierPK(),
			NTilde:     nil,
			H1:         big.NewInt(3),
			H2:         big.NewInt(5),
		}, false},
		{"NTilde_zero", &DGRound2Message1{
			PaillierPK: testPaillierPK(),
			NTilde:     big.NewInt(0),
			H1:         big.NewInt(3),
			H2:         big.NewInt(5),
		}, false},
		{"NTilde_negative", &DGRound2Message1{
			PaillierPK: testPaillierPK(),
			NTilde:     big.NewInt(-1),
			H1:         big.NewInt(3),
			H2:         big.NewInt(5),
		}, false},
		{"H1_nil", &DGRound2Message1{
			PaillierPK: testPaillierPK(),
			NTilde:     big.NewInt(7),
			H1:         nil,
			H2:         big.NewInt(5),
		}, false},
		{"H1_zero", &DGRound2Message1{
			PaillierPK: testPaillierPK(),
			NTilde:     big.NewInt(7),
			H1:         big.NewInt(0),
			H2:         big.NewInt(5),
		}, false},
		{"H1_negative", &DGRound2Message1{
			PaillierPK: testPaillierPK(),
			NTilde:     big.NewInt(7),
			H1:         big.NewInt(-1),
			H2:         big.NewInt(5),
		}, false},
		{"H2_nil", &DGRound2Message1{
			PaillierPK: testPaillierPK(),
			NTilde:     big.NewInt(7),
			H1:         big.NewInt(3),
			H2:         nil,
		}, false},
		{"H2_zero", &DGRound2Message1{
			PaillierPK: testPaillierPK(),
			NTilde:     big.NewInt(7),
			H1:         big.NewInt(3),
			H2:         big.NewInt(0),
		}, false},
		{"H2_negative", &DGRound2Message1{
			PaillierPK: testPaillierPK(),
			NTilde:     big.NewInt(7),
			H1:         big.NewInt(3),
			H2:         big.NewInt(-1),
		}, false},
		{"all_zero_value", &DGRound2Message1{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.ValidateBasic(); got != tt.want {
				t.Fatalf("ValidateBasic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDGRound2Message2_ValidateBasic(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		msg  *DGRound2Message2
		want bool
	}{
		{"valid", &DGRound2Message2{}, true},
		{"nil_receiver", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.ValidateBasic(); got != tt.want {
				t.Fatalf("ValidateBasic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDGRound3Message1_ValidateBasic(t *testing.T) {
	t.Parallel()
	valid := &DGRound3Message1{
		Share:      big.NewInt(42),
		ReceiverID: []byte("receiver"),
	}

	tests := []struct {
		name string
		msg  *DGRound3Message1
		want bool
	}{
		{"valid", valid, true},
		{"nil_receiver", nil, false},
		{"Share_nil", &DGRound3Message1{
			Share:      nil,
			ReceiverID: []byte("receiver"),
		}, false},
		{"Share_zero", &DGRound3Message1{
			Share:      big.NewInt(0),
			ReceiverID: []byte("receiver"),
		}, false},
		{"Share_negative", &DGRound3Message1{
			Share:      big.NewInt(-1),
			ReceiverID: []byte("receiver"),
		}, false},
		{"ReceiverID_nil", &DGRound3Message1{
			Share:      big.NewInt(42),
			ReceiverID: nil,
		}, false},
		{"ReceiverID_empty", &DGRound3Message1{
			Share:      big.NewInt(42),
			ReceiverID: []byte{},
		}, false},
		{"all_zero_value", &DGRound3Message1{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.ValidateBasic(); got != tt.want {
				t.Fatalf("ValidateBasic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDGRound3Message2_ValidateBasic(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		msg  *DGRound3Message2
		want bool
	}{
		{"valid_2_elements", &DGRound3Message2{
			VDeCommitment: cmt.HashDeCommitment{big.NewInt(1), big.NewInt(2)},
		}, true},
		{"valid_3_elements", &DGRound3Message2{
			VDeCommitment: cmt.HashDeCommitment{big.NewInt(1), big.NewInt(2), big.NewInt(3)},
		}, true},
		{"nil_receiver", nil, false},
		{"VDeCommitment_nil", &DGRound3Message2{
			VDeCommitment: nil,
		}, false},
		{"VDeCommitment_empty", &DGRound3Message2{
			VDeCommitment: cmt.HashDeCommitment{},
		}, false},
		{"VDeCommitment_1_element", &DGRound3Message2{
			VDeCommitment: cmt.HashDeCommitment{big.NewInt(1)},
		}, false},
		{"zero_value", &DGRound3Message2{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.ValidateBasic(); got != tt.want {
				t.Fatalf("ValidateBasic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDGRound4Message1_ValidateBasic(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		msg  *DGRound4Message1
		want bool
	}{
		{"valid", &DGRound4Message1{ReceiverID: []byte("receiver")}, true},
		{"valid_nil_FacProof", &DGRound4Message1{
			FacProof:   nil,
			ReceiverID: []byte("receiver"),
		}, true},
		{"nil_receiver", nil, false},
		{"ReceiverID_nil", &DGRound4Message1{
			ReceiverID: nil,
		}, false},
		{"ReceiverID_empty", &DGRound4Message1{
			ReceiverID: []byte{},
		}, false},
		{"zero_value", &DGRound4Message1{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.ValidateBasic(); got != tt.want {
				t.Fatalf("ValidateBasic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDGRound4Message2_ValidateBasic(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		msg  *DGRound4Message2
		want bool
	}{
		{"valid", &DGRound4Message2{}, true},
		{"nil_receiver", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.ValidateBasic(); got != tt.want {
				t.Fatalf("ValidateBasic() = %v, want %v", got, tt.want)
			}
		})
	}
}
