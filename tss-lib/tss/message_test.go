// Copyright (c) 2025 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package tss

import (
	"bytes"
	"math/big"
	"testing"

	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// TestWireBytesDeterministic verifies that calling WireBytes() multiple times
// on the same message produces identical output.
func TestWireBytesDeterministic(t *testing.T) {
	// Build a MessageImpl with some content.
	from := &PartyID{
		MessageWrapper_PartyID: &MessageWrapper_PartyID{
			Id:      "party-1",
			Moniker: "Alice",
			Key:     big.NewInt(12345).Bytes(),
		},
		Index: 0,
	}

	// Use a simple well-known proto message as content.
	inner := wrapperspb.String("deterministic test payload")
	anyMsg, err := anypb.New(inner)
	if err != nil {
		t.Fatalf("anypb.New failed: %v", err)
	}

	wire := &MessageWrapper{
		IsBroadcast: true,
		From:        from.MessageWrapper_PartyID,
		Message:     anyMsg,
	}

	msg := &MessageImpl{
		MessageRouting: MessageRouting{
			From:        from,
			IsBroadcast: true,
		},
		wire: wire,
	}

	// Marshal twice and compare.
	bz1, _, err := msg.WireBytes()
	if err != nil {
		t.Fatalf("first WireBytes failed: %v", err)
	}
	bz2, _, err := msg.WireBytes()
	if err != nil {
		t.Fatalf("second WireBytes failed: %v", err)
	}

	if !bytes.Equal(bz1, bz2) {
		t.Fatalf("WireBytes is not deterministic:\n  first:  %x\n  second: %x", bz1, bz2)
	}

	if len(bz1) == 0 {
		t.Fatal("WireBytes produced empty output")
	}
}
