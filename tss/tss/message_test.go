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

// testContent is a message body for the ValidateMsgs tests.
type testContent struct{ ok bool }

func (c *testContent) ValidateBasic() bool { return c != nil && c.ok }

// otherContent is a message body of a different type.
type otherContent struct{}

func (c *otherContent) ValidateBasic() bool { return c != nil }

// TestValidateMsgs verifies that ValidateMsgs returns an error that
// names the bad slot, and that it does not panic.
func TestValidateMsgs(t *testing.T) {
	ids := GenerateTestPartyIDs(3)
	good := func(j int) *Message { return &Message{From: ids[j], Content: &testContent{ok: true}} }
	tests := []struct {
		name    string
		msgs    []*Message
		self    int
		culprit int // -1: no error; -2: error without a culprit
	}{
		{"valid", []*Message{good(0), good(1), good(2)}, -1, -1},
		{"own slot empty", []*Message{good(0), nil, good(2)}, 1, -1},
		{"missing", []*Message{good(0), nil, good(2)}, -1, 1},
		{"nil content", []*Message{good(0), {From: ids[1]}, good(2)}, -1, 1},
		{"wrong type", []*Message{good(0), {From: ids[1], Content: &otherContent{}}, good(2)}, -1, 1},
		{"typed nil", []*Message{good(0), {From: ids[1], Content: (*testContent)(nil)}, good(2)}, -1, 1},
		{"fails ValidateBasic", []*Message{good(0), {From: ids[1], Content: &testContent{}}, good(2)}, -1, 1},
		{"nil From", []*Message{good(0), {Content: &testContent{ok: true}}, good(2)}, -1, 1},
		{"From of another party", []*Message{good(0), {From: ids[2], Content: &testContent{ok: true}}, good(2)}, -1, 1},
		{"short", []*Message{good(0), good(1)}, -1, -2},
		{"long", []*Message{good(0), good(1), good(2), good(0)}, -1, -2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := ValidateMsgs[*testContent](tt.msgs, ids, tt.self)
			switch {
			case tt.culprit == -1 && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tt.culprit == -2 && (err == nil || len(c) != 0):
				t.Fatalf("got %v, %v; want an error without a culprit", c, err)
			case tt.culprit >= 0 && (err == nil || len(c) != 1 || c[0] != ids[tt.culprit]):
				t.Fatalf("got %v, %v; want party %d", c, err, tt.culprit)
			}
		})
	}
}

// TestMergeMsgsIgnoresLongSource verifies that a source longer than
// the store does not panic.
func TestMergeMsgsIgnoresLongSource(t *testing.T) {
	dst := make([]*Message, 1)
	MergeMsgs(dst, []*Message{{}, {}})
	if dst[0] == nil {
		t.Fatal("MergeMsgs must copy slot 0")
	}
}
