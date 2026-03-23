// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package resharing

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	cmt "github.com/hemilabs/x/tss-lib/v3/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v3/crypto/dlnproof"
	"github.com/hemilabs/x/tss-lib/v3/crypto/facproof"
	"github.com/hemilabs/x/tss-lib/v3/crypto/modproof"
	"github.com/hemilabs/x/tss-lib/v3/crypto/paillier"
	"github.com/hemilabs/x/tss-lib/v3/ecdsa/keygen"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

const (
	reshareN         = 3
	reshareThreshold = 1 // 2-of-3
)

// ReshareFixture holds accumulated state from running reshare rounds
// up to a certain point. Each "setupThrough*" function extends the
// fixture by one round, filling in the next set of message slices.
//
// Committee structure (3 old -> 3 new, disjoint):
//   - Old committee: oldPIDs[0..2], oldStates[0..2], oldKeys[0..2]
//   - New committee: newPIDs[0..2], newStates[0..2], preParamsNew[0..2]
//
// Message naming convention:
//   - OldR1Msgs[oldIdx]       = DGRound1Message broadcast by old party oldIdx
//   - NewR2Msg1s[newIdx]      = DGRound2Message1 broadcast by new party newIdx (Pedersen params)
//   - NewR2Msg2s[newIdx]      = DGRound2Message2 broadcast by new party newIdx (ACK to old)
//   - OldR3P2P[newIdx][oldIdx]= DGRound3Message1 P2P from old party oldIdx to new party newIdx
//   - OldR3Bcast[oldIdx]      = DGRound3Message2 broadcast by old party oldIdx (decommitment)
//   - NewR4P2P[newIdx][senderNewIdx] = DGRound4Message1 P2P from new party senderNewIdx to newIdx
//   - NewR4Bcast[newIdx]      = DGRound4Message2 broadcast by new party newIdx (ACK)
type ReshareFixture struct {
	// Keygen outputs
	OldKeys []keygen.LocalPartySaveData

	// Party IDs and contexts
	OldPIDs tss.SortedPartyIDs
	NewPIDs tss.SortedPartyIDs
	OldCtx  *tss.PeerContext
	NewCtx  *tss.PeerContext

	// Pre-params for new committee
	PreParamsNew []keygen.LocalPreParams

	// Reshare states
	OldStates []*ReshareState
	NewStates []*ReshareState

	// Round 1: old committee broadcasts
	OldR1Msgs []*tss.Message // [oldIdx]

	// Round 2: new committee broadcasts
	NewR2Msg1s []*tss.Message // [newIdx] DGRound2Message1 (Pedersen params + proofs)
	NewR2Msg2s []*tss.Message // [newIdx] DGRound2Message2 (ACK to old)

	// Round 3: old committee P2P + broadcast
	OldR3P2P   [][]*tss.Message // [newIdx][oldIdx] DGRound3Message1
	OldR3Bcast []*tss.Message   // [oldIdx] DGRound3Message2

	// Round 4: new committee P2P + broadcast
	NewR4P2P   [][]*tss.Message // [newIdx][senderNewIdx] DGRound4Message1
	NewR4Bcast []*tss.Message   // [newIdx] DGRound4Message2
}

// doKeygen runs a full keygen for n parties with the given threshold and
// returns the keygen save data, party IDs, and peer context.
func doKeygen(t *testing.T, n, threshold int) ([]keygen.LocalPartySaveData, tss.SortedPartyIDs, *tss.PeerContext) {
	t.Helper()

	preParams := make([]keygen.LocalPreParams, n)
	for i := 0; i < n; i++ {
		pp, err := keygen.GeneratePreParams(5 * time.Minute)
		if err != nil {
			t.Fatalf("doKeygen: GeneratePreParams[%d]: %v", i, err)
		}
		preParams[i] = *pp
	}

	pIDs := tss.GenerateTestPartyIDs(n)
	peerCtx := tss.NewPeerContext(pIDs)

	// Round 1
	states := make([]*keygen.KeygenState, n)
	r1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.S256(), peerCtx, pIDs[i], n, threshold)
		st, out, err := keygen.Round1(context.Background(), params, preParams[i])
		if err != nil {
			t.Fatalf("doKeygen: Round1[%d]: %v", i, err)
		}
		states[i] = st
		r1[i] = out.Messages[0]
	}

	// Round 2
	r2P2P := make([][]*tss.Message, n)
	r2Bcast := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		r2P2P[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := keygen.Round2(context.Background(), states[i], r1)
		if err != nil {
			t.Fatalf("doKeygen: Round2[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			pm := msg
			if pm.To == nil {
				r2Bcast[i] = pm
			} else {
				for _, to := range pm.To {
					r2P2P[to.Index][i] = pm
				}
			}
		}
		r2P2P[i][i] = states[i].ExportR2P2PSelf()
		if r2Bcast[i] == nil {
			r2Bcast[i] = states[i].ExportR2BcastSelf()
		}
	}

	// Round 3
	r3 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := keygen.Round3(context.Background(), states[i], r2P2P[i], r2Bcast)
		if err != nil {
			t.Fatalf("doKeygen: Round3[%d]: %v", i, err)
		}
		r3[i] = out.Messages[0]
	}

	// Round 4
	keys := make([]keygen.LocalPartySaveData, n)
	for i := 0; i < n; i++ {
		out, err := keygen.Round4(context.Background(), states[i], r3)
		if err != nil {
			t.Fatalf("doKeygen: Round4[%d]: %v", i, err)
		}
		keys[i] = *out.Save
	}

	return keys, pIDs, peerCtx
}

// setupReshareRound1 runs keygen(3) then reshare Round1 for 3 old -> 3 new
// (disjoint committees). Returns a fixture with OldR1Msgs populated.
//
// All proof flags (DLN, Mod, Fac) are disabled for speed in negative tests,
// since the proofs themselves are not the subject under test.
func setupReshareRound1(t *testing.T) *ReshareFixture {
	t.Helper()
	n := reshareN
	threshold := reshareThreshold

	oldKeys, oldPIDs, oldCtx := doKeygen(t, n, threshold)

	newPIDs := tss.GenerateTestPartyIDs(n)
	newCtx := tss.NewPeerContext(newPIDs)

	preParamsNew := make([]keygen.LocalPreParams, n)
	for i := 0; i < n; i++ {
		pp, err := keygen.GeneratePreParams(5 * time.Minute)
		if err != nil {
			t.Fatalf("setupReshareRound1: GeneratePreParams(new)[%d]: %v", i, err)
		}
		preParamsNew[i] = *pp
	}

	oldStates := make([]*ReshareState, n)
	newStates := make([]*ReshareState, n)
	oldR1Msgs := make([]*tss.Message, n)

	// Old committee: Round1 produces DGRound1Message broadcasts.
	for i := 0; i < n; i++ {
		params := tss.NewReSharingParameters(tss.S256(), oldCtx, newCtx, oldPIDs[i], n, threshold, n, threshold)
		params.SetNoProofDLN()
		params.SetNoProofMod()
		params.SetNoProofFac()
		st, out, err := ReshareRound1(params, oldKeys[i], keygen.LocalPreParams{})
		if err != nil {
			t.Fatalf("setupReshareRound1: ReshareRound1(old)[%d]: %v", i, err)
		}
		oldStates[i] = st
		if len(out.Messages) > 0 {
			oldR1Msgs[i] = out.Messages[0]
		}
	}

	// New committee: Round1 is a no-op, just creates state.
	for i := 0; i < n; i++ {
		params := tss.NewReSharingParameters(tss.S256(), oldCtx, newCtx, newPIDs[i], n, threshold, n, threshold)
		params.SetNoProofDLN()
		params.SetNoProofMod()
		params.SetNoProofFac()
		st, _, err := ReshareRound1(params, keygen.NewLocalPartySaveData(n), preParamsNew[i])
		if err != nil {
			t.Fatalf("setupReshareRound1: ReshareRound1(new)[%d]: %v", i, err)
		}
		newStates[i] = st
	}

	return &ReshareFixture{
		OldKeys:      oldKeys,
		OldPIDs:      oldPIDs,
		NewPIDs:      newPIDs,
		OldCtx:       oldCtx,
		NewCtx:       newCtx,
		PreParamsNew: preParamsNew,
		OldStates:    oldStates,
		NewStates:    newStates,
		OldR1Msgs:    oldR1Msgs,
	}
}

// setupThroughRound2 runs keygen + reshare Round1 + Round2.
// Returns a fixture with NewR2Msg1s and NewR2Msg2s populated.
func setupThroughRound2(t *testing.T) *ReshareFixture {
	t.Helper()
	n := reshareN
	fix := setupReshareRound1(t)

	// New committee: Round2 produces DGRound2Message1 + DGRound2Message2.
	fix.NewR2Msg1s = make([]*tss.Message, n)
	fix.NewR2Msg2s = make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := ReshareRound2(fix.NewStates[i], fix.OldR1Msgs)
		if err != nil {
			t.Fatalf("setupThroughRound2: ReshareRound2(new)[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			pm := msg
			switch pm.Content.(type) {
			case *DGRound2Message1:
				fix.NewR2Msg1s[i] = pm
			case *DGRound2Message2:
				fix.NewR2Msg2s[i] = pm
			}
		}
	}

	// Old committee: Round2 is a no-op.
	for i := 0; i < n; i++ {
		_, err := ReshareRound2(fix.OldStates[i], fix.OldR1Msgs)
		if err != nil {
			t.Fatalf("setupThroughRound2: ReshareRound2(old)[%d]: %v", i, err)
		}
	}

	// Fill self-messages for new committee.
	for i := 0; i < n; i++ {
		if fix.NewR2Msg1s[i] == nil {
			fix.NewR2Msg1s[i] = fix.NewStates[i].temp.dgRound2Message1s[i]
		}
		if fix.NewR2Msg2s[i] == nil {
			fix.NewR2Msg2s[i] = fix.NewStates[i].temp.dgRound2Message2s[i]
		}
	}

	return fix
}

// setupThroughRound3 runs keygen + reshare Rounds 1-3.
// Returns a fixture with OldR3P2P and OldR3Bcast populated.
func setupThroughRound3(t *testing.T) *ReshareFixture {
	t.Helper()
	n := reshareN
	fix := setupThroughRound2(t)

	// Old committee: Round3 produces DGRound3Message1 (P2P) + DGRound3Message2 (broadcast).
	fix.OldR3P2P = make([][]*tss.Message, n)
	fix.OldR3Bcast = make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		fix.OldR3P2P[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := ReshareRound3(fix.OldStates[i], fix.NewR2Msg2s)
		if err != nil {
			t.Fatalf("setupThroughRound3: ReshareRound3(old)[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			pm := msg
			switch pm.Content.(type) {
			case *DGRound3Message1:
				for _, to := range pm.To {
					fix.OldR3P2P[to.Index][i] = pm
				}
			case *DGRound3Message2:
				fix.OldR3Bcast[i] = pm
			}
		}
	}

	// New committee: Round3 is a no-op.
	for i := 0; i < n; i++ {
		_, err := ReshareRound3(fix.NewStates[i], fix.NewR2Msg2s)
		if err != nil {
			t.Fatalf("setupThroughRound3: ReshareRound3(new)[%d]: %v", i, err)
		}
	}

	return fix
}

// setupThroughRound4 runs keygen + reshare Rounds 1-4.
// Returns a fixture with NewR4P2P and NewR4Bcast populated.
func setupThroughRound4(t *testing.T) *ReshareFixture {
	t.Helper()
	n := reshareN
	fix := setupThroughRound3(t)

	// New committee: Round4 produces DGRound4Message1 (P2P) + DGRound4Message2 (broadcast).
	fix.NewR4P2P = make([][]*tss.Message, n)
	fix.NewR4Bcast = make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		fix.NewR4P2P[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := ReshareRound4(context.Background(), fix.NewStates[i], fix.NewR2Msg1s, fix.OldR3P2P[i], fix.OldR3Bcast)
		if err != nil {
			t.Fatalf("setupThroughRound4: ReshareRound4(new)[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			pm := msg
			switch pm.Content.(type) {
			case *DGRound4Message1:
				for _, to := range pm.To {
					fix.NewR4P2P[to.Index][i] = pm
				}
			case *DGRound4Message2:
				fix.NewR4Bcast[i] = pm
			}
		}
	}

	// Old committee: Round4 is a no-op.
	for i := 0; i < n; i++ {
		_, err := ReshareRound4(context.Background(), fix.OldStates[i], fix.NewR2Msg1s, nil, nil)
		if err != nil {
			t.Fatalf("setupThroughRound4: ReshareRound4(old)[%d]: %v", i, err)
		}
	}

	// Fill self-messages.
	for i := 0; i < n; i++ {
		if fix.NewR4Bcast[i] == nil {
			fix.NewR4Bcast[i] = fix.NewStates[i].temp.dgRound4Message2s[i]
		}
	}

	return fix
}

// ---------------------------------------------------------------------------
// Clone helpers
// ---------------------------------------------------------------------------
// Each clone function creates a shallow copy of a *tss.Message with a
// deep-copied Content struct, so mutations in negative tests do not
// corrupt the original fixture messages.

// cloneDGRound1Message clones a *tss.Message containing a DGRound1Message.
func cloneDGRound1Message(orig *tss.Message) *tss.Message {
	c := orig.Content.(*DGRound1Message)
	ssid := make([]byte, len(c.SSID))
	copy(ssid, c.SSID)
	return &tss.Message{
		From:        orig.From,
		To:          orig.To,
		IsBroadcast: orig.IsBroadcast,
		Content: &DGRound1Message{
			ECDSAPub:    c.ECDSAPub, // immutable ECPoint, no need to deep-copy
			VCommitment: new(big.Int).Set(c.VCommitment),
			SSID:        ssid,
		},
	}
}

// cloneDGRound2Message1 clones a *tss.Message containing a DGRound2Message1.
// Proof objects (ModProof, DLNProof1, DLNProof2) are shared (not deep-copied)
// since they are large and typically not mutated in negative tests. The
// scalar fields (NTilde, H1, H2) are deep-copied.
func cloneDGRound2Message1(orig *tss.Message) *tss.Message {
	c := orig.Content.(*DGRound2Message1)
	var paiPK *paillier.PublicKey
	if c.PaillierPK != nil {
		paiPK = &paillier.PublicKey{N: new(big.Int).Set(c.PaillierPK.N)}
	}
	return &tss.Message{
		From:        orig.From,
		To:          orig.To,
		IsBroadcast: orig.IsBroadcast,
		Content: &DGRound2Message1{
			PaillierPK: paiPK,
			NTilde:     new(big.Int).Set(c.NTilde),
			H1:         new(big.Int).Set(c.H1),
			H2:         new(big.Int).Set(c.H2),
			ModProof:   c.ModProof,  // shared reference
			DLNProof1:  c.DLNProof1, // shared reference
			DLNProof2:  c.DLNProof2, // shared reference
		},
	}
}

// cloneDGRound2Message2 clones a *tss.Message containing a DGRound2Message2.
func cloneDGRound2Message2(orig *tss.Message) *tss.Message {
	return &tss.Message{
		From:             orig.From,
		To:               orig.To,
		IsBroadcast:      orig.IsBroadcast,
		IsToOldCommittee: orig.IsToOldCommittee,
		Content:          &DGRound2Message2{},
	}
}

// cloneDGRound3Message1 clones a *tss.Message containing a DGRound3Message1.
func cloneDGRound3Message1(orig *tss.Message) *tss.Message {
	c := orig.Content.(*DGRound3Message1)
	rid := make([]byte, len(c.ReceiverID))
	copy(rid, c.ReceiverID)
	return &tss.Message{
		From: orig.From,
		To:   orig.To,
		Content: &DGRound3Message1{
			Share:      new(big.Int).Set(c.Share),
			ReceiverID: rid,
		},
	}
}

// cloneDGRound3Message2 clones a *tss.Message containing a DGRound3Message2.
func cloneDGRound3Message2(orig *tss.Message) *tss.Message {
	c := orig.Content.(*DGRound3Message2)
	dcmt := make(cmt.HashDeCommitment, len(c.VDeCommitment))
	for i, v := range c.VDeCommitment {
		if v != nil {
			dcmt[i] = new(big.Int).Set(v)
		}
	}
	return &tss.Message{
		From:        orig.From,
		To:          orig.To,
		IsBroadcast: orig.IsBroadcast,
		Content: &DGRound3Message2{
			VDeCommitment: dcmt,
		},
	}
}

// cloneDGRound4Message1 clones a *tss.Message containing a DGRound4Message1.
// The FacProof is shared (not deep-copied) since it is large.
func cloneDGRound4Message1(orig *tss.Message) *tss.Message {
	c := orig.Content.(*DGRound4Message1)
	rid := make([]byte, len(c.ReceiverID))
	copy(rid, c.ReceiverID)
	return &tss.Message{
		From: orig.From,
		To:   orig.To,
		Content: &DGRound4Message1{
			FacProof:   c.FacProof, // shared reference
			ReceiverID: rid,
		},
	}
}

// cloneDGRound4Message2 clones a *tss.Message containing a DGRound4Message2.
func cloneDGRound4Message2(orig *tss.Message) *tss.Message {
	return &tss.Message{
		From:                    orig.From,
		To:                      orig.To,
		IsBroadcast:             orig.IsBroadcast,
		IsToOldAndNewCommittees: orig.IsToOldAndNewCommittees,
		Content:                 &DGRound4Message2{},
	}
}

// ---------------------------------------------------------------------------
// Slice-copy helpers
// ---------------------------------------------------------------------------
// These copy entire message slices so that a negative test can mutate one
// element without affecting the fixture.

// copyR1Slice returns a deep-copied slice of DGRound1Message broadcasts.
func copyR1Slice(msgs []*tss.Message) []*tss.Message {
	out := make([]*tss.Message, len(msgs))
	for i, m := range msgs {
		if m != nil {
			out[i] = cloneDGRound1Message(m)
		}
	}
	return out
}

// copyR2Msg1Slice returns a deep-copied slice of DGRound2Message1 broadcasts.
func copyR2Msg1Slice(msgs []*tss.Message) []*tss.Message {
	out := make([]*tss.Message, len(msgs))
	for i, m := range msgs {
		if m != nil {
			out[i] = cloneDGRound2Message1(m)
		}
	}
	return out
}

// copyR2Msg2Slice returns a deep-copied slice of DGRound2Message2 broadcasts.
func copyR2Msg2Slice(msgs []*tss.Message) []*tss.Message {
	out := make([]*tss.Message, len(msgs))
	for i, m := range msgs {
		if m != nil {
			out[i] = cloneDGRound2Message2(m)
		}
	}
	return out
}

// copyR3P2PSlice returns a deep-copied slice of DGRound3Message1 P2P messages
// for a single receiver (i.e., fix.OldR3P2P[receiverNewIdx]).
func copyR3P2PSlice(msgs []*tss.Message) []*tss.Message {
	out := make([]*tss.Message, len(msgs))
	for i, m := range msgs {
		if m != nil {
			out[i] = cloneDGRound3Message1(m)
		}
	}
	return out
}

// copyR3BcastSlice returns a deep-copied slice of DGRound3Message2 broadcasts.
func copyR3BcastSlice(msgs []*tss.Message) []*tss.Message {
	out := make([]*tss.Message, len(msgs))
	for i, m := range msgs {
		if m != nil {
			out[i] = cloneDGRound3Message2(m)
		}
	}
	return out
}

// copyR4P2PSlice returns a deep-copied slice of DGRound4Message1 P2P messages
// for a single receiver (i.e., fix.NewR4P2P[receiverNewIdx]).
func copyR4P2PSlice(msgs []*tss.Message) []*tss.Message {
	out := make([]*tss.Message, len(msgs))
	for i, m := range msgs {
		if m != nil {
			out[i] = cloneDGRound4Message1(m)
		}
	}
	return out
}

// copyR4BcastSlice returns a deep-copied slice of DGRound4Message2 broadcasts.
func copyR4BcastSlice(msgs []*tss.Message) []*tss.Message {
	out := make([]*tss.Message, len(msgs))
	for i, m := range msgs {
		if m != nil {
			out[i] = cloneDGRound4Message2(m)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Deep-clone helpers for proof objects (used when a negative test needs
// to corrupt a proof field without affecting the shared original).
// ---------------------------------------------------------------------------

// cloneModProof returns a deep copy of a modproof.ProofMod.
func cloneModProof(orig *modproof.ProofMod) *modproof.ProofMod {
	if orig == nil {
		return nil
	}
	p := &modproof.ProofMod{
		W: new(big.Int).Set(orig.W),
		A: new(big.Int).Set(orig.A),
		B: new(big.Int).Set(orig.B),
	}
	for i := range orig.X {
		if orig.X[i] != nil {
			p.X[i] = new(big.Int).Set(orig.X[i])
		}
	}
	for i := range orig.Z {
		if orig.Z[i] != nil {
			p.Z[i] = new(big.Int).Set(orig.Z[i])
		}
	}
	return p
}

// cloneDLNProof returns a deep copy of a dlnproof.Proof.
func cloneDLNProof(orig *dlnproof.Proof) *dlnproof.Proof {
	if orig == nil {
		return nil
	}
	p := &dlnproof.Proof{}
	for i := range orig.Alpha {
		if orig.Alpha[i] != nil {
			p.Alpha[i] = new(big.Int).Set(orig.Alpha[i])
		}
	}
	for i := range orig.T {
		if orig.T[i] != nil {
			p.T[i] = new(big.Int).Set(orig.T[i])
		}
	}
	return p
}

// cloneFacProof returns a deep copy of a facproof.ProofFac.
func cloneFacProof(orig *facproof.ProofFac) *facproof.ProofFac {
	if orig == nil {
		return nil
	}
	return &facproof.ProofFac{
		P:     new(big.Int).Set(orig.P),
		Q:     new(big.Int).Set(orig.Q),
		A:     new(big.Int).Set(orig.A),
		B:     new(big.Int).Set(orig.B),
		T:     new(big.Int).Set(orig.T),
		Sigma: new(big.Int).Set(orig.Sigma),
		Z1:    new(big.Int).Set(orig.Z1),
		Z2:    new(big.Int).Set(orig.Z2),
		W1:    new(big.Int).Set(orig.W1),
		W2:    new(big.Int).Set(orig.W2),
		V:     new(big.Int).Set(orig.V),
	}
}

// setupThroughRound3WithModProof is identical to setupThroughRound3 except
// that ModProof is NOT disabled on the new committee. This means Round2
// generates real ModProof objects, allowing Round4 tests to exercise the
// proof verification path (lines 408-426 of round_fn.go).
func setupThroughRound3WithModProof(t *testing.T) *ReshareFixture {
	t.Helper()
	n := reshareN
	threshold := reshareThreshold

	oldKeys, oldPIDs, oldCtx := doKeygen(t, n, threshold)

	newPIDs := tss.GenerateTestPartyIDs(n)
	newCtx := tss.NewPeerContext(newPIDs)

	preParamsNew := make([]keygen.LocalPreParams, n)
	for i := 0; i < n; i++ {
		pp, err := keygen.GeneratePreParams(5 * time.Minute)
		if err != nil {
			t.Fatalf("GeneratePreParams(new)[%d]: %v", i, err)
		}
		preParamsNew[i] = *pp
	}

	oldStates := make([]*ReshareState, n)
	newStates := make([]*ReshareState, n)
	oldR1Msgs := make([]*tss.Message, n)

	for i := 0; i < n; i++ {
		params := tss.NewReSharingParameters(tss.S256(), oldCtx, newCtx, oldPIDs[i], n, threshold, n, threshold)
		params.SetNoProofDLN()
		params.SetNoProofMod()
		params.SetNoProofFac()
		st, out, err := ReshareRound1(params, oldKeys[i], keygen.LocalPreParams{})
		if err != nil {
			t.Fatalf("ReshareRound1(old)[%d]: %v", i, err)
		}
		oldStates[i] = st
		if len(out.Messages) > 0 {
			oldR1Msgs[i] = out.Messages[0]
		}
	}
	for i := 0; i < n; i++ {
		params := tss.NewReSharingParameters(tss.S256(), oldCtx, newCtx, newPIDs[i], n, threshold, n, threshold)
		params.SetNoProofDLN()
		// NOTE: SetNoProofMod() is NOT called — ModProof will be generated in Round2.
		params.SetNoProofFac()
		st, _, err := ReshareRound1(params, keygen.NewLocalPartySaveData(n), preParamsNew[i])
		if err != nil {
			t.Fatalf("ReshareRound1(new)[%d]: %v", i, err)
		}
		newStates[i] = st
	}

	fix := &ReshareFixture{
		OldKeys: oldKeys, OldPIDs: oldPIDs, NewPIDs: newPIDs,
		OldCtx: oldCtx, NewCtx: newCtx, PreParamsNew: preParamsNew,
		OldStates: oldStates, NewStates: newStates, OldR1Msgs: oldR1Msgs,
	}

	// Round2
	fix.NewR2Msg1s = make([]*tss.Message, n)
	fix.NewR2Msg2s = make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := ReshareRound2(fix.NewStates[i], fix.OldR1Msgs)
		if err != nil {
			t.Fatalf("ReshareRound2(new)[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			switch msg.Content.(type) {
			case *DGRound2Message1:
				fix.NewR2Msg1s[i] = msg
			case *DGRound2Message2:
				fix.NewR2Msg2s[i] = msg
			}
		}
	}
	for i := 0; i < n; i++ {
		_, err := ReshareRound2(fix.OldStates[i], fix.OldR1Msgs)
		if err != nil {
			t.Fatalf("ReshareRound2(old)[%d]: %v", i, err)
		}
	}
	for i := 0; i < n; i++ {
		if fix.NewR2Msg1s[i] == nil {
			fix.NewR2Msg1s[i] = fix.NewStates[i].temp.dgRound2Message1s[i]
		}
		if fix.NewR2Msg2s[i] == nil {
			fix.NewR2Msg2s[i] = fix.NewStates[i].temp.dgRound2Message2s[i]
		}
	}

	// Round3
	fix.OldR3P2P = make([][]*tss.Message, n)
	fix.OldR3Bcast = make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		fix.OldR3P2P[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := ReshareRound3(fix.OldStates[i], fix.NewR2Msg2s)
		if err != nil {
			t.Fatalf("ReshareRound3(old)[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			pm := msg
			switch pm.Content.(type) {
			case *DGRound3Message1:
				for _, to := range pm.To {
					idx := newIndex2(to, newPIDs)
					fix.OldR3P2P[idx][i] = pm
				}
			case *DGRound3Message2:
				fix.OldR3Bcast[i] = pm
			}
		}
	}
	for i := 0; i < n; i++ {
		_, err := ReshareRound3(fix.NewStates[i], fix.NewR2Msg2s)
		if err != nil {
			t.Fatalf("ReshareRound3(new)[%d]: %v", i, err)
		}
	}

	return fix
}

// newIndex2 finds a party's index in a sorted ID list by key comparison.
func newIndex2(pid *tss.PartyID, pids tss.SortedPartyIDs) int {
	for i, p := range pids {
		if p.KeyInt().Cmp(pid.KeyInt()) == 0 {
			return i
		}
	}
	return -1
}

// requireCulprit unwraps a *tss.Error and asserts the culprit has the expected index.
func requireCulprit(t *testing.T, err error, wantIdx int) {
	t.Helper()
	tssErr := &tss.Error{}
	if ok := errors.As(err, &tssErr); !ok {
		t.Fatalf("expected *tss.Error, got %T", err)
	}
	culprits := tssErr.Culprits()
	if len(culprits) != 1 || culprits[0].Index != wantIdx {
		t.Fatalf("expected culprit index %d, got: %v", wantIdx, culprits)
	}
}

// Compile-time check: ensure all setup functions and clone helpers are usable.
var (
	_ = doKeygen
	_ = setupReshareRound1
	_ = setupThroughRound2
	_ = setupThroughRound3
	_ = setupThroughRound3WithModProof
	_ = setupThroughRound4
	_ = cloneDGRound1Message
	_ = cloneDGRound2Message1
	_ = cloneDGRound2Message2
	_ = cloneDGRound3Message1
	_ = cloneDGRound3Message2
	_ = cloneDGRound4Message1
	_ = cloneDGRound4Message2
	_ = copyR1Slice
	_ = copyR2Msg1Slice
	_ = copyR2Msg2Slice
	_ = copyR3P2PSlice
	_ = copyR3BcastSlice
	_ = copyR4P2PSlice
	_ = copyR4BcastSlice
	_ = cloneModProof
	_ = cloneDLNProof
	_ = cloneFacProof
)
