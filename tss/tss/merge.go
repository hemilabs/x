// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package tss

import "fmt"

// MergeMsgs copies non-nil entries from src into dst, preserving
// any entries (such as self-messages from a previous round) that
// are already set in dst.  Using copy() instead would overwrite
// populated slots with nil when the source slice is sparse.
// Entries past the end of dst are ignored.
func MergeMsgs(dst, src []*Message) {
	for j, m := range src {
		if m != nil && j < len(dst) {
			dst[j] = m
		}
	}
}

// ValidateMsgs checks that msgs holds one message of content type T
// from each party in ids, and that each message passes ValidateBasic.
// The sender of slot j must be the party ids[j].
// The slot at index self can be empty, because the round reads that
// message from its own state; pass -1 to require every slot.
//
// On failure it returns the error and the party of the bad slot.  A
// slice of the wrong length names no party.
func ValidateMsgs[T interface{ ValidateBasic() bool }](msgs []*Message, ids SortedPartyIDs, self int) ([]*PartyID, error) {
	if len(msgs) != len(ids) {
		return nil, fmt.Errorf("got %d messages, want %d", len(msgs), len(ids))
	}
	for j, msg := range msgs {
		if msg == nil {
			if j == self {
				continue
			}
			return []*PartyID{ids[j]}, fmt.Errorf("missing message from party %d", j)
		}
		if msg.From == nil || msg.From.KeyInt().Cmp(ids[j].KeyInt()) != 0 {
			return []*PartyID{ids[j]}, fmt.Errorf("message in slot %d is not from party %d", j, j)
		}
		c, ok := msg.Content.(T)
		if !ok || !c.ValidateBasic() {
			return []*PartyID{ids[j]}, fmt.Errorf("invalid message from party %d", j)
		}
	}
	return nil, nil
}
