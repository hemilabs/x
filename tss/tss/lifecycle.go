// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package tss

import (
	"errors"
	"fmt"
	"sync"
)

// ErrStateFailed reports a call on a state whose earlier round failed.
var ErrStateFailed = errors.New("state failed in an earlier round")

// Lifecycle makes a protocol state run each round once and in order.
// Round 1 creates the state, so the zero value accepts round 2 first.
// A round that returns an error or panics marks the state failed, and
// the state then rejects every later call; start a new session with a
// new state.  The mutex serializes the rounds.
type Lifecycle struct {
	mu     sync.Mutex
	ran    int // rounds that completed after round 1
	failed bool
}

// Begin locks the state for round.  It returns an error, and does not
// lock, if the state failed or round is not the next round.  A call
// out of order does not mark the state failed.  Call End with defer
// when Begin returns nil.
func (l *Lifecycle) Begin(round int) error {
	l.mu.Lock()
	if l.failed {
		l.mu.Unlock()
		return ErrStateFailed
	}
	if next := l.ran + 2; round != next {
		l.mu.Unlock()
		return fmt.Errorf("round %d called, want round %d", round, next)
	}
	l.failed = true // until End reports success
	return nil
}

// End unlocks the state.  If *errp is nil and the round did not panic,
// the state accepts the next round.  Otherwise the state stays failed
// and wipe, if not nil, clears its secrets.  Call it directly with
// defer, so that it can see a panic.
func (l *Lifecycle) End(errp *error, wipe func()) {
	r := recover()
	if r == nil && *errp == nil {
		l.failed = false
		l.ran++
	} else if wipe != nil {
		wipe()
	}
	l.mu.Unlock()
	if r != nil {
		panic(r)
	}
}
