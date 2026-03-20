// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package tss

import (
	"errors"
	"math/big"
	"strings"
	"testing"
)

func TestNewErrorWithCulprits(t *testing.T) {
	victim := NewPartyID("v", "V", big.NewInt(1))
	c1 := NewPartyID("c1", "C1", big.NewInt(2))
	c2 := NewPartyID("c2", "C2", big.NewInt(3))
	cause := errors.New("bad share")

	e := NewError(cause, "ecdsa-keygen", 3, victim, c1, c2)

	if e.Task() != "ecdsa-keygen" {
		t.Fatalf("Task: want ecdsa-keygen, got %s", e.Task())
	}
	if e.Round() != 3 {
		t.Fatalf("Round: want 3, got %d", e.Round())
	}
	if e.Victim() != victim {
		t.Fatal("Victim mismatch")
	}
	if len(e.Culprits()) != 2 {
		t.Fatalf("Culprits: want 2, got %d", len(e.Culprits()))
	}
	if !errors.Is(e.Unwrap(), cause) {
		t.Fatal("Unwrap mismatch")
	}
	if !errors.Is(e.Cause(), cause) {
		t.Fatal("Cause mismatch")
	}

	s := e.Error()
	if !strings.Contains(s, "culprits") {
		t.Fatalf("Error() should contain 'culprits': %s", s)
	}
	if !strings.Contains(s, "bad share") {
		t.Fatalf("Error() should contain cause: %s", s)
	}
}

func TestNewErrorWithoutCulprits(t *testing.T) {
	victim := NewPartyID("v", "V", big.NewInt(1))
	e := NewError(errors.New("oops"), "eddsa-signing", 2, victim)

	s := e.Error()
	if strings.Contains(s, "culprits") {
		t.Fatalf("Error() without culprits should not contain 'culprits': %s", s)
	}
	if !strings.Contains(s, "round 2") {
		t.Fatalf("Error() should contain round: %s", s)
	}
}

func TestErrorNilReceiver(t *testing.T) {
	var e *Error
	if e.Error() != "Error is nil" {
		t.Fatalf("nil Error.Error(): %s", e.Error())
	}
}

func TestErrorNilCause(t *testing.T) {
	e := &Error{}
	if e.Error() != "Error is nil" {
		t.Fatalf("nil-cause Error.Error(): %s", e.Error())
	}
}
