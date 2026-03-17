// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package ckd_test

import (
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"

	. "github.com/hemilabs/x/tss-lib/v3/crypto/ckd"
)

func TestDeriveChildKeyFromHierarchy(t *testing.T) {
	masterPubKey := "xpub661MyMwAqRbcFtXgS5sYJABqqG9YLmC4Q1Rdap9gSE8NqtwybGhePY2gZ29ESFjqJoCu1Rupje8YtGqsefD265TMg7usUDFdp6W1EGMcet8"
	wantPub := "xpub6BqyndF6rhZqmgktFCBcapkwubGxPqoAZtQaYewJHXVKZcLdnqBVC8N6f6FSHWUghjuTLeubWyQWfJdk2G3tGgvgj3qngo4vLTnnSjAZckv"

	ec := btcec.S256()
	extKey, err := NewExtendedKeyFromString(masterPubKey, ec)
	if err != nil {
		t.Fatalf("NewExtendedKeyFromString: %v", err)
	}

	path := []uint32{0, 1, 2}
	_, childKey, err := DeriveChildKeyFromHierarchy(path, extKey, ec.Params().N, ec)
	if err != nil {
		t.Fatalf("DeriveChildKeyFromHierarchy: %v", err)
	}
	if childKey.String() != wantPub {
		t.Fatalf("mismatch:\n  got:  %s\n  want: %s", childKey.String(), wantPub)
	}
}

func TestDeriveChildKeyFromHierarchyEmpty(t *testing.T) {
	masterPubKey := "xpub661MyMwAqRbcFtXgS5sYJABqqG9YLmC4Q1Rdap9gSE8NqtwybGhePY2gZ29ESFjqJoCu1Rupje8YtGqsefD265TMg7usUDFdp6W1EGMcet8"

	ec := btcec.S256()
	extKey, err := NewExtendedKeyFromString(masterPubKey, ec)
	if err != nil {
		t.Fatalf("NewExtendedKeyFromString: %v", err)
	}

	// Empty path should return the master key unchanged.
	delta, childKey, err := DeriveChildKeyFromHierarchy([]uint32{}, extKey, ec.Params().N, ec)
	if err != nil {
		t.Fatalf("DeriveChildKeyFromHierarchy(empty): %v", err)
	}
	if delta.Sign() != 0 {
		t.Fatalf("expected zero delta for empty path, got %v", delta)
	}
	if childKey.String() != masterPubKey {
		t.Fatal("empty path should return master key")
	}
}
