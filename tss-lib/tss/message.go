// Copyright © 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.
package tss

import "fmt"

// Message carries a round function's output: routing metadata +
// content.  Content is an untyped interface{} — each round function
// knows the concrete type it produces, and consumers type-assert.
//
// Serialization is the caller's responsibility.  The library does
// not prescribe a wire format.
type Message struct {
	From                    *PartyID
	To                      []*PartyID // nil = broadcast
	IsBroadcast             bool
	IsToOldCommittee        bool
	IsToOldAndNewCommittees bool
	Content                 interface{}
}

// String returns a human-readable summary for logging.
func (m *Message) String() string {
	toStr := "all"
	if m.To != nil {
		toStr = fmt.Sprintf("%v", m.To)
	}
	return fmt.Sprintf("From: %s, To: %s, Broadcast: %v", m.From, toStr, m.IsBroadcast)
}
