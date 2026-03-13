// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package tss

// MergeMsgs copies non-nil entries from src into dst, preserving
// any entries (such as self-messages from a previous round) that
// are already set in dst.  Using copy() instead would overwrite
// populated slots with nil when the source slice is sparse.
func MergeMsgs(dst, src []ParsedMessage) {
	for j, m := range src {
		if m != nil {
			dst[j] = m
		}
	}
}
