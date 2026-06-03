// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"encoding/json"
	"testing"

	_ "embed"
)

//go:embed testdata/preparams.json
var embeddedPreParams []byte

// loadTestPreParams returns n pre-computed LocalPreParams from the
// embedded fixture.  Internal to keygen tests to avoid import cycles.
func loadTestPreParams(t *testing.T, n int) []LocalPreParams {
	t.Helper()
	var params []LocalPreParams
	if err := json.Unmarshal(embeddedPreParams, &params); err != nil {
		t.Fatalf("parse embedded preparams: %v", err)
	}
	if n > len(params) {
		t.Fatalf("need %d preparams, fixture has %d", n, len(params))
	}
	return params[:n]
}
