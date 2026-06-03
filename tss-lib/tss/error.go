// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package tss

import (
	"fmt"
)

// fundamental is an error that has a message and a stack, but no caller.
type Error struct {
	cause    error
	task     string
	round    int
	victim   *PartyID
	culprits []*PartyID
}

// NewError creates a TSS protocol error with round context and culprits.
func NewError(err error, task string, round int, victim *PartyID, culprits ...*PartyID) *Error {
	return &Error{cause: err, task: task, round: round, victim: victim, culprits: culprits}
}

// Unwrap returns the underlying error.
func (err *Error) Unwrap() error { return err.cause }

// Cause returns the underlying error (alias for Unwrap).
func (err *Error) Cause() error { return err.cause }

// Task returns the protocol name (e.g. "ecdsa-keygen").
func (err *Error) Task() string { return err.task }

// Round returns the round number where the error occurred.
func (err *Error) Round() int { return err.round }

// Victim returns the party that detected the error.
func (err *Error) Victim() *PartyID { return err.victim }

// Culprits returns the parties responsible for the error.
func (err *Error) Culprits() []*PartyID { return err.culprits }

// Error returns a human-readable error string.
func (err *Error) Error() string {
	if err == nil || err.cause == nil {
		return "Error is nil"
	}
	if len(err.culprits) > 0 {
		return fmt.Sprintf("task %s, party %v, round %d, culprits %s: %s",
			err.task, err.victim, err.round, err.culprits, err.cause.Error())
	}
	return fmt.Sprintf("task %s, party %v, round %d: %s",
		err.task, err.victim, err.round, err.cause.Error())
}
