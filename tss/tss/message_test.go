// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package tss

import (
	"math/big"
	"strings"
	"testing"
)

func TestMessageStringBroadcast(t *testing.T) {
	from := NewPartyID("a", "A", big.NewInt(1))
	m := &Message{From: from, IsBroadcast: true}
	s := m.String()
	if !strings.Contains(s, "all") {
		t.Fatalf("broadcast message String should contain 'all': %s", s)
	}
	if !strings.Contains(s, "Broadcast: true") {
		t.Fatalf("broadcast message String should contain 'Broadcast: true': %s", s)
	}
}

func TestMessageStringP2P(t *testing.T) {
	from := NewPartyID("a", "A", big.NewInt(1))
	to := NewPartyID("b", "B", big.NewInt(2))
	m := &Message{From: from, To: []*PartyID{to}}
	s := m.String()
	if strings.Contains(s, "all") {
		t.Fatalf("P2P message String should not contain 'all': %s", s)
	}
}

func TestMergeMsgsBasic(t *testing.T) {
	m1 := &Message{IsBroadcast: true}
	m2 := &Message{IsBroadcast: false}
	dst := make([]*Message, 3)
	dst[0] = m1 // pre-existing

	src := make([]*Message, 3)
	src[1] = m2 // only slot 1 set

	MergeMsgs(dst, src)

	if dst[0] != m1 {
		t.Fatal("MergeMsgs should preserve existing dst[0]")
	}
	if dst[1] != m2 {
		t.Fatal("MergeMsgs should copy src[1] into dst[1]")
	}
	if dst[2] != nil {
		t.Fatal("MergeMsgs should not set dst[2] from nil src[2]")
	}
}

func TestMergeMsgsOverwrite(t *testing.T) {
	old := &Message{IsBroadcast: true}
	updated := &Message{IsBroadcast: false}
	dst := []*Message{old}
	src := []*Message{updated}
	MergeMsgs(dst, src)
	if dst[0] != updated {
		t.Fatal("MergeMsgs should overwrite non-nil with non-nil")
	}
}

func TestPeerContextSetIDs(t *testing.T) {
	ids1 := GenerateTestPartyIDs(3)
	ids2 := GenerateTestPartyIDs(2)
	ctx := NewPeerContext(ids1)
	if len(ctx.IDs()) != 3 {
		t.Fatalf("initial IDs: want 3, got %d", len(ctx.IDs()))
	}
	ctx.SetIDs(ids2)
	if len(ctx.IDs()) != 2 {
		t.Fatalf("after SetIDs: want 2, got %d", len(ctx.IDs()))
	}
}
