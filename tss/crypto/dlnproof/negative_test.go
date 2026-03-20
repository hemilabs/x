// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package dlnproof

import (
	"math/big"
	"testing"
)

func TestUnmarshalDLNProofTooFewParts(t *testing.T) {
	_, err := UnmarshalDLNProof([][]byte{big.NewInt(1).Bytes(), big.NewInt(2).Bytes()})
	if err == nil {
		t.Fatal("expected error for too few parts")
	}
}

func TestUnmarshalDLNProofEmpty(t *testing.T) {
	_, err := UnmarshalDLNProof(nil)
	if err == nil {
		t.Fatal("expected error for nil input")
	}
}
