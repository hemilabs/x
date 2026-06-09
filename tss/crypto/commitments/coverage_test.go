// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package commitments

import (
	"math/big"
	"testing"
)

func TestNewHashDeCommitmentFromBytes(t *testing.T) {
	d := NewHashDeCommitmentFromBytes([][]byte{
		big.NewInt(1).Bytes(),
		big.NewInt(2).Bytes(),
	})
	if len(d) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(d))
	}
	if d[0].Cmp(big.NewInt(1)) != 0 || d[1].Cmp(big.NewInt(2)) != 0 {
		t.Fatal("values mismatch")
	}
}

func TestBuilder(t *testing.T) {
	b := NewBuilder()
	if b == nil {
		t.Fatal("NewBuilder returned nil")
	}
	if len(b.Parts()) != 0 {
		t.Fatal("new builder should have 0 parts")
	}
	b.AddPart([]*big.Int{big.NewInt(1), big.NewInt(2)})
	b.AddPart([]*big.Int{big.NewInt(3)})
	if len(b.Parts()) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(b.Parts()))
	}
	if len(b.Parts()[0]) != 2 {
		t.Fatalf("expected part 0 len 2, got %d", len(b.Parts()[0]))
	}
}

func TestSecretsAndParseSecretsRoundTrip(t *testing.T) {
	b := NewBuilder()
	b.AddPart([]*big.Int{big.NewInt(10), big.NewInt(20)})
	b.AddPart([]*big.Int{big.NewInt(30)})

	secrets, err := b.Secrets()
	if err != nil {
		t.Fatalf("Secrets: %v", err)
	}

	parts, err := ParseSecrets(secrets)
	if err != nil {
		t.Fatalf("ParseSecrets: %v", err)
	}
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	if parts[0][0].Cmp(big.NewInt(10)) != 0 || parts[0][1].Cmp(big.NewInt(20)) != 0 {
		t.Fatal("part 0 mismatch")
	}
}

func TestParseSecretsTooSmall(t *testing.T) {
	_, err := ParseSecrets([]*big.Int{big.NewInt(1)})
	if err == nil {
		t.Fatal("expected error for too-small input")
	}
}

func TestParseSecretsNil(t *testing.T) {
	_, err := ParseSecrets(nil)
	if err == nil {
		t.Fatal("expected error for nil input")
	}
}

func TestSecretsTooManyParts(t *testing.T) {
	b := NewBuilder()
	// PartsCap is 3, add 4 parts
	for i := 0; i < 4; i++ {
		b.AddPart([]*big.Int{big.NewInt(int64(i))})
	}
	_, err := b.Secrets()
	if err == nil {
		t.Fatal("expected error for too many parts")
	}
}

func TestParseSecretsBadPartLen(t *testing.T) {
	// Craft: [length=-1, ...]
	_, err := ParseSecrets([]*big.Int{big.NewInt(-1), big.NewInt(0)})
	if err == nil {
		t.Fatal("expected error for negative part length")
	}
}

func TestParseSecretsTooManyParts(t *testing.T) {
	// Craft secrets with 4 parts (PartsCap=3)
	secrets := make([]*big.Int, 0)
	for i := 0; i < 4; i++ {
		secrets = append(secrets, big.NewInt(1))        // length prefix: 1
		secrets = append(secrets, big.NewInt(int64(i))) // value
	}
	_, err := ParseSecrets(secrets)
	if err == nil {
		t.Fatal("expected error for too many parts")
	}
}
