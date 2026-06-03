// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

// Package testutil provides pre-computed Paillier preparams for test
// use.  Generating safe primes from scratch takes minutes per party;
// this fixture eliminates that cost from CI.
package testutil

import (
	"encoding/json"
	"testing"

	_ "embed"

	"github.com/hemilabs/x/tss-lib/v3/ecdsa/keygen"
)

//go:embed preparams.json
var embeddedPreParams []byte

var cachedParams []keygen.LocalPreParams

func allParams(t *testing.T) []keygen.LocalPreParams {
	t.Helper()
	if cachedParams != nil {
		return cachedParams
	}
	if err := json.Unmarshal(embeddedPreParams, &cachedParams); err != nil {
		t.Fatalf("parse embedded preparams: %v", err)
	}
	return cachedParams
}

// LoadPreParams returns n pre-computed LocalPreParams from the
// embedded fixture starting at index 0.
func LoadPreParams(t *testing.T, n int) []keygen.LocalPreParams {
	return LoadPreParamsFrom(t, 0, n)
}

// LoadPreParamsFrom returns n pre-computed LocalPreParams starting
// at the given offset.  Use distinct offsets when old and new
// committees need non-overlapping preparams (e.g. resharing tests).
func LoadPreParamsFrom(t *testing.T, offset, n int) []keygen.LocalPreParams {
	t.Helper()
	params := allParams(t)
	if offset+n > len(params) {
		t.Fatalf("need preparams[%d:%d], fixture has %d", offset, offset+n, len(params))
	}
	return params[offset : offset+n]
}
