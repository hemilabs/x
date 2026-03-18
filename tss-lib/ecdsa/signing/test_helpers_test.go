// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package signing

import (
	"context"
	"crypto/sha256"
	"math/big"
	"testing"
	"time"

	"github.com/hemilabs/x/tss-lib/v3/crypto"
	"github.com/hemilabs/x/tss-lib/v3/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v3/crypto/mta"
	"github.com/hemilabs/x/tss-lib/v3/crypto/schnorr"
	"github.com/hemilabs/x/tss-lib/v3/ecdsa/keygen"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// testN is the fixed party count for negative test fixtures.
const testN = 3

// testThreshold is the fixed threshold for negative test fixtures (2-of-3).
const testThreshold = 1

// testMsg returns a deterministic message hash for signing test fixtures.
func testMsg() *big.Int {
	h := sha256.Sum256([]byte("negative test fixture message"))
	return new(big.Int).SetBytes(h[:])
}

// doKeygen runs a full 3-party keygen ceremony (with proofs enabled)
// and returns the save data for each party.
func doKeygen(t *testing.T) []keygen.LocalPartySaveData {
	t.Helper()
	const n = testN

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

	kgStates := make([]*keygen.KeygenState, n)
	kgR1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.S256(), peerCtx, pIDs[i], n, testThreshold)
		st, out, err := keygen.Round1(context.Background(), params, preParams[i])
		if err != nil {
			t.Fatalf("doKeygen: Round1[%d]: %v", i, err)
		}
		kgStates[i] = st
		kgR1[i] = out.Messages[0]
	}

	kgR2P2P := make([][]*tss.Message, n)
	kgR2Bcast := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		kgR2P2P[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := keygen.Round2(context.Background(), kgStates[i], kgR1)
		if err != nil {
			t.Fatalf("doKeygen: Round2[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			pm := msg
			if pm.To == nil {
				kgR2Bcast[i] = pm
			} else {
				for _, to := range pm.To {
					kgR2P2P[to.Index][i] = pm
				}
			}
		}
		kgR2P2P[i][i] = kgStates[i].ExportR2P2PSelf()
		if kgR2Bcast[i] == nil {
			kgR2Bcast[i] = kgStates[i].ExportR2BcastSelf()
		}
	}

	kgR3 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := keygen.Round3(context.Background(), kgStates[i], kgR2P2P[i], kgR2Bcast)
		if err != nil {
			t.Fatalf("doKeygen: Round3[%d]: %v", i, err)
		}
		kgR3[i] = out.Messages[0]
	}

	saves := make([]keygen.LocalPartySaveData, n)
	for i := 0; i < n; i++ {
		out, err := keygen.Round4(context.Background(), kgStates[i], kgR3)
		if err != nil {
			t.Fatalf("doKeygen: Round4[%d]: %v", i, err)
		}
		saves[i] = *out.Save
	}
	return saves
}

// setupPartyIDs returns a consistent set of party IDs and peer context
// for the test fixture party count.
func setupPartyIDs() (tss.SortedPartyIDs, *tss.PeerContext) {
	pIDs := tss.GenerateTestPartyIDs(testN)
	return pIDs, tss.NewPeerContext(pIDs)
}

// -----------------------------------------------------------------
// Round fixture types
// -----------------------------------------------------------------

// SignFixture holds the accumulated state from running signing rounds.
// Each setupThroughRoundN function returns this with progressively
// more fields populated.
type SignFixture struct {
	// Party infrastructure
	PIDs    tss.SortedPartyIDs
	PeerCtx *tss.PeerContext
	Keys    []keygen.LocalPartySaveData
	Msg     *big.Int

	// Per-party signing state (mutated as rounds progress)
	States []*SigningState

	// Round 1 outputs
	R1P2P   [][]*tss.Message // R1P2P[recipient][sender]
	R1Bcast []*tss.Message   // R1Bcast[sender]

	// Round 2 outputs
	R2P2P [][]*tss.Message // R2P2P[recipient][sender]

	// Round 3 outputs
	R3Bcast []*tss.Message // R3Bcast[sender]

	// Round 4 outputs
	R4Bcast []*tss.Message // R4Bcast[sender]

	// Round 5 outputs
	R5Bcast []*tss.Message // R5Bcast[sender]

	// Round 6 outputs
	R6Bcast []*tss.Message // R6Bcast[sender]

	// Round 7 outputs
	R7Bcast []*tss.Message // R7Bcast[sender]

	// Round 8 outputs
	R8Bcast []*tss.Message // R8Bcast[sender]

	// Round 9 outputs
	R9Bcast []*tss.Message // R9Bcast[sender]
}

// -----------------------------------------------------------------
// setupSignRound1
// -----------------------------------------------------------------

// setupSignRound1 runs keygen and SignRound1 for all parties.
// Returns the fixture with States, R1P2P, and R1Bcast populated.
func setupSignRound1(t *testing.T, keys []keygen.LocalPartySaveData) *SignFixture {
	t.Helper()
	const n = testN

	pIDs, peerCtx := setupPartyIDs()
	m := testMsg()

	states := make([]*SigningState, n)
	r1P2P := make([][]*tss.Message, n)
	r1Bcast := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		r1P2P[i] = make([]*tss.Message, n)
	}

	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.S256(), peerCtx, pIDs[i], n, testThreshold)
		st, out, err := SignRound1(params, keys[i], m, nil, 0)
		if err != nil {
			t.Fatalf("setupSignRound1: SignRound1[%d]: %v", i, err)
		}
		states[i] = st
		for _, msg := range out.Messages {
			pm := msg
			if pm.To == nil {
				r1Bcast[i] = pm
			} else {
				for _, to := range pm.To {
					r1P2P[to.Index][i] = pm
				}
			}
		}
	}

	return &SignFixture{
		PIDs:    pIDs,
		PeerCtx: peerCtx,
		Keys:    keys,
		Msg:     m,
		States:  states,
		R1P2P:   r1P2P,
		R1Bcast: r1Bcast,
	}
}

// -----------------------------------------------------------------
// setupThroughRound2
// -----------------------------------------------------------------

// setupThroughRound2 runs keygen + rounds 1-2. Returns fixture with
// R2P2P populated (P2P messages).
func setupThroughRound2(t *testing.T) *SignFixture {
	t.Helper()
	const n = testN

	keys := doKeygen(t)
	f := setupSignRound1(t, keys)

	r2P2P := make([][]*tss.Message, n)
	for i := 0; i < n; i++ {
		r2P2P[i] = make([]*tss.Message, n)
	}
	for i := 0; i < n; i++ {
		out, err := SignRound2(context.Background(), f.States[i], f.R1P2P[i], f.R1Bcast)
		if err != nil {
			t.Fatalf("setupThroughRound2: SignRound2[%d]: %v", i, err)
		}
		for _, msg := range out.Messages {
			pm := msg
			for _, to := range pm.To {
				r2P2P[to.Index][i] = pm
			}
		}
	}

	f.R2P2P = r2P2P
	return f
}

// -----------------------------------------------------------------
// setupThroughRound3
// -----------------------------------------------------------------

// setupThroughRound3 runs keygen + rounds 1-3. Returns fixture with
// R3Bcast populated (broadcast theta shares).
func setupThroughRound3(t *testing.T) *SignFixture {
	t.Helper()
	const n = testN

	f := setupThroughRound2(t)

	r3Bcast := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := SignRound3(context.Background(), f.States[i], f.R2P2P[i])
		if err != nil {
			t.Fatalf("setupThroughRound3: SignRound3[%d]: %v", i, err)
		}
		r3Bcast[i] = out.Messages[0]
	}

	f.R3Bcast = r3Bcast
	return f
}

// -----------------------------------------------------------------
// setupThroughRound4
// -----------------------------------------------------------------

// setupThroughRound4 runs keygen + rounds 1-4. Returns fixture with
// R4Bcast populated (broadcast decommitment + ZK proof).
func setupThroughRound4(t *testing.T) *SignFixture {
	t.Helper()
	const n = testN

	f := setupThroughRound3(t)

	r4Bcast := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := SignRound4(f.States[i], f.R3Bcast)
		if err != nil {
			t.Fatalf("setupThroughRound4: SignRound4[%d]: %v", i, err)
		}
		r4Bcast[i] = out.Messages[0]
	}

	f.R4Bcast = r4Bcast
	return f
}

// -----------------------------------------------------------------
// setupThroughRound5
// -----------------------------------------------------------------

// setupThroughRound5 runs keygen + rounds 1-5. Returns fixture with
// R5Bcast populated (broadcast commitment to blinding).
func setupThroughRound5(t *testing.T) *SignFixture {
	t.Helper()
	const n = testN

	f := setupThroughRound4(t)

	r5Bcast := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := SignRound5(f.States[i], f.R4Bcast)
		if err != nil {
			t.Fatalf("setupThroughRound5: SignRound5[%d]: %v", i, err)
		}
		r5Bcast[i] = out.Messages[0]
	}

	f.R5Bcast = r5Bcast
	return f
}

// -----------------------------------------------------------------
// setupThroughRound6
// -----------------------------------------------------------------

// setupThroughRound6 runs keygen + rounds 1-6. Returns fixture with
// R6Bcast populated (broadcast decommitment + Schnorr proofs).
func setupThroughRound6(t *testing.T) *SignFixture {
	t.Helper()
	const n = testN

	f := setupThroughRound5(t)

	r6Bcast := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := SignRound6(f.States[i])
		if err != nil {
			t.Fatalf("setupThroughRound6: SignRound6[%d]: %v", i, err)
		}
		r6Bcast[i] = out.Messages[0]
	}

	f.R6Bcast = r6Bcast
	return f
}

// -----------------------------------------------------------------
// setupThroughRound7
// -----------------------------------------------------------------

// setupThroughRound7 runs keygen + rounds 1-7. Returns fixture with
// R7Bcast populated (broadcast commitment to Ui/Ti).
func setupThroughRound7(t *testing.T) *SignFixture {
	t.Helper()
	const n = testN

	f := setupThroughRound6(t)

	r7Bcast := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := SignRound7(f.States[i], f.R5Bcast, f.R6Bcast)
		if err != nil {
			t.Fatalf("setupThroughRound7: SignRound7[%d]: %v", i, err)
		}
		r7Bcast[i] = out.Messages[0]
	}

	f.R7Bcast = r7Bcast
	return f
}

// -----------------------------------------------------------------
// setupThroughRound8
// -----------------------------------------------------------------

// setupThroughRound8 runs keygen + rounds 1-8. Returns fixture with
// R8Bcast populated (broadcast decommitment of Ui/Ti).
func setupThroughRound8(t *testing.T) *SignFixture {
	t.Helper()
	const n = testN

	f := setupThroughRound7(t)

	r8Bcast := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := SignRound8(f.States[i])
		if err != nil {
			t.Fatalf("setupThroughRound8: SignRound8[%d]: %v", i, err)
		}
		r8Bcast[i] = out.Messages[0]
	}

	f.R8Bcast = r8Bcast
	return f
}

// -----------------------------------------------------------------
// setupThroughRound9
// -----------------------------------------------------------------

// setupThroughRound9 runs keygen + rounds 1-9. Returns fixture with
// R9Bcast populated (broadcast partial signature shares).
func setupThroughRound9(t *testing.T) *SignFixture {
	t.Helper()
	const n = testN

	f := setupThroughRound8(t)

	r9Bcast := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		out, err := SignRound9(f.States[i], f.R7Bcast, f.R8Bcast)
		if err != nil {
			t.Fatalf("setupThroughRound9: SignRound9[%d]: %v", i, err)
		}
		r9Bcast[i] = out.Messages[0]
	}

	f.R9Bcast = r9Bcast
	return f
}

// =================================================================
// Message cloning helpers
// =================================================================
//
// Each clone function performs a deep copy of the message content so
// that negative tests can corrupt individual fields without affecting
// the original fixture data used by other parties.

// cloneBigInt returns a deep copy of a *big.Int (nil-safe).
func cloneBigInt(v *big.Int) *big.Int {
	if v == nil {
		return nil
	}
	return new(big.Int).Set(v)
}

// cloneBigInts returns a deep copy of a []*big.Int slice.
func cloneBigInts(vs []*big.Int) []*big.Int {
	if vs == nil {
		return nil
	}
	out := make([]*big.Int, len(vs))
	for i, v := range vs {
		out[i] = cloneBigInt(v)
	}
	return out
}

// cloneBytes returns a deep copy of a byte slice (nil-safe).
func cloneBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

// cloneMessage creates a shallow copy of a *tss.Message with the
// Content replaced by a deep-cloned version using the provided
// content cloner. The From and To pointers are shared (they are
// immutable party IDs).
func cloneMessage(m *tss.Message, clonedContent interface{}) *tss.Message {
	return &tss.Message{
		From:                    m.From,
		To:                      m.To,
		IsBroadcast:             m.IsBroadcast,
		IsToOldCommittee:        m.IsToOldCommittee,
		IsToOldAndNewCommittees: m.IsToOldAndNewCommittees,
		Content:                 clonedContent,
	}
}

// -----------------------------------------------------------------
// Round 1 Message 1 (P2P): Paillier ciphertext + range proof
// -----------------------------------------------------------------

// cloneRangeProofAlice deep-copies a *mta.RangeProofAlice.
func cloneRangeProofAlice(p *mta.RangeProofAlice) *mta.RangeProofAlice {
	if p == nil {
		return nil
	}
	return &mta.RangeProofAlice{
		Z:  cloneBigInt(p.Z),
		U:  cloneBigInt(p.U),
		W:  cloneBigInt(p.W),
		S:  cloneBigInt(p.S),
		S1: cloneBigInt(p.S1),
		S2: cloneBigInt(p.S2),
	}
}

// CloneSignRound1Message1 deep-copies a SignRound1Message1 content.
func CloneSignRound1Message1(m *SignRound1Message1) *SignRound1Message1 {
	if m == nil {
		return nil
	}
	return &SignRound1Message1{
		C:               cloneBigInt(m.C),
		RangeProofAlice: cloneRangeProofAlice(m.RangeProofAlice),
		ReceiverID:      cloneBytes(m.ReceiverID),
	}
}

// CloneR1P2PMsg deep-copies a Round 1 P2P message (SignRound1Message1).
func CloneR1P2PMsg(m *tss.Message) *tss.Message {
	return cloneMessage(m, CloneSignRound1Message1(m.Content.(*SignRound1Message1)))
}

// -----------------------------------------------------------------
// Round 1 Message 2 (Broadcast): commitment
// -----------------------------------------------------------------

// CloneSignRound1Message2 deep-copies a SignRound1Message2 content.
func CloneSignRound1Message2(m *SignRound1Message2) *SignRound1Message2 {
	if m == nil {
		return nil
	}
	return &SignRound1Message2{
		Commitment: cloneBigInt(m.Commitment),
	}
}

// CloneR1BcastMsg deep-copies a Round 1 broadcast message (SignRound1Message2).
func CloneR1BcastMsg(m *tss.Message) *tss.Message {
	return cloneMessage(m, CloneSignRound1Message2(m.Content.(*SignRound1Message2)))
}

// -----------------------------------------------------------------
// Round 2 Message (P2P): MtA ciphertexts + Bob proofs
// -----------------------------------------------------------------

// cloneProofBob deep-copies a *mta.ProofBob.
func cloneProofBob(p *mta.ProofBob) *mta.ProofBob {
	if p == nil {
		return nil
	}
	return &mta.ProofBob{
		Z:    cloneBigInt(p.Z),
		ZPrm: cloneBigInt(p.ZPrm),
		T:    cloneBigInt(p.T),
		V:    cloneBigInt(p.V),
		W:    cloneBigInt(p.W),
		S:    cloneBigInt(p.S),
		S1:   cloneBigInt(p.S1),
		S2:   cloneBigInt(p.S2),
		T1:   cloneBigInt(p.T1),
		T2:   cloneBigInt(p.T2),
	}
}

// cloneProofBobWC deep-copies a *mta.ProofBobWC.
func cloneProofBobWC(p *mta.ProofBobWC) *mta.ProofBobWC {
	if p == nil {
		return nil
	}
	var u *crypto.ECPoint
	if p.U != nil {
		// ECPoint.X() and .Y() return copies already
		u, _ = crypto.NewECPoint(p.U.Curve(), p.U.X(), p.U.Y())
	}
	return &mta.ProofBobWC{
		ProofBob: cloneProofBob(p.ProofBob),
		U:        u,
	}
}

// CloneSignRound2Message deep-copies a SignRound2Message content.
func CloneSignRound2Message(m *SignRound2Message) *SignRound2Message {
	if m == nil {
		return nil
	}
	return &SignRound2Message{
		C1:         cloneBigInt(m.C1),
		C2:         cloneBigInt(m.C2),
		ProofBob:   cloneProofBob(m.ProofBob),
		ProofBobWC: cloneProofBobWC(m.ProofBobWC),
		ReceiverID: cloneBytes(m.ReceiverID),
	}
}

// CloneR2P2PMsg deep-copies a Round 2 P2P message (SignRound2Message).
func CloneR2P2PMsg(m *tss.Message) *tss.Message {
	return cloneMessage(m, CloneSignRound2Message(m.Content.(*SignRound2Message)))
}

// -----------------------------------------------------------------
// Round 3 Message (Broadcast): theta share
// -----------------------------------------------------------------

// CloneSignRound3Message deep-copies a SignRound3Message content.
func CloneSignRound3Message(m *SignRound3Message) *SignRound3Message {
	if m == nil {
		return nil
	}
	return &SignRound3Message{
		Theta: cloneBigInt(m.Theta),
	}
}

// CloneR3BcastMsg deep-copies a Round 3 broadcast message (SignRound3Message).
func CloneR3BcastMsg(m *tss.Message) *tss.Message {
	return cloneMessage(m, CloneSignRound3Message(m.Content.(*SignRound3Message)))
}

// -----------------------------------------------------------------
// Round 4 Message (Broadcast): decommitment + ZK proof
// -----------------------------------------------------------------

// cloneZKProof deep-copies a *schnorr.ZKProof.
func cloneZKProof(p *schnorr.ZKProof) *schnorr.ZKProof {
	if p == nil {
		return nil
	}
	var alpha *crypto.ECPoint
	if p.Alpha != nil {
		alpha, _ = crypto.NewECPoint(p.Alpha.Curve(), p.Alpha.X(), p.Alpha.Y())
	}
	return &schnorr.ZKProof{
		Alpha: alpha,
		T:     cloneBigInt(p.T),
	}
}

// cloneDeCommitment deep-copies a commitments.HashDeCommitment ([]*big.Int).
func cloneDeCommitment(d commitments.HashDeCommitment) commitments.HashDeCommitment {
	return cloneBigInts(d)
}

// CloneSignRound4Message deep-copies a SignRound4Message content.
func CloneSignRound4Message(m *SignRound4Message) *SignRound4Message {
	if m == nil {
		return nil
	}
	return &SignRound4Message{
		DeCommitment: cloneDeCommitment(m.DeCommitment),
		ZKProof:      cloneZKProof(m.ZKProof),
	}
}

// CloneR4BcastMsg deep-copies a Round 4 broadcast message (SignRound4Message).
func CloneR4BcastMsg(m *tss.Message) *tss.Message {
	return cloneMessage(m, CloneSignRound4Message(m.Content.(*SignRound4Message)))
}

// -----------------------------------------------------------------
// Round 5 Message (Broadcast): commitment to blinding
// -----------------------------------------------------------------

// CloneSignRound5Message deep-copies a SignRound5Message content.
func CloneSignRound5Message(m *SignRound5Message) *SignRound5Message {
	if m == nil {
		return nil
	}
	return &SignRound5Message{
		Commitment: cloneBigInt(m.Commitment),
	}
}

// CloneR5BcastMsg deep-copies a Round 5 broadcast message (SignRound5Message).
func CloneR5BcastMsg(m *tss.Message) *tss.Message {
	return cloneMessage(m, CloneSignRound5Message(m.Content.(*SignRound5Message)))
}

// -----------------------------------------------------------------
// Round 6 Message (Broadcast): decommitment + ZK + ZKV proofs
// -----------------------------------------------------------------

// cloneZKVProof deep-copies a *schnorr.ZKVProof.
func cloneZKVProof(p *schnorr.ZKVProof) *schnorr.ZKVProof {
	if p == nil {
		return nil
	}
	var alpha *crypto.ECPoint
	if p.Alpha != nil {
		alpha, _ = crypto.NewECPoint(p.Alpha.Curve(), p.Alpha.X(), p.Alpha.Y())
	}
	return &schnorr.ZKVProof{
		Alpha: alpha,
		T:     cloneBigInt(p.T),
		U:     cloneBigInt(p.U),
	}
}

// CloneSignRound6Message deep-copies a SignRound6Message content.
func CloneSignRound6Message(m *SignRound6Message) *SignRound6Message {
	if m == nil {
		return nil
	}
	return &SignRound6Message{
		DeCommitment: cloneDeCommitment(m.DeCommitment),
		ZKProof:      cloneZKProof(m.ZKProof),
		ZKVProof:     cloneZKVProof(m.ZKVProof),
	}
}

// CloneR6BcastMsg deep-copies a Round 6 broadcast message (SignRound6Message).
func CloneR6BcastMsg(m *tss.Message) *tss.Message {
	return cloneMessage(m, CloneSignRound6Message(m.Content.(*SignRound6Message)))
}

// -----------------------------------------------------------------
// Round 7 Message (Broadcast): commitment to Ui/Ti
// -----------------------------------------------------------------

// CloneSignRound7Message deep-copies a SignRound7Message content.
func CloneSignRound7Message(m *SignRound7Message) *SignRound7Message {
	if m == nil {
		return nil
	}
	return &SignRound7Message{
		Commitment: cloneBigInt(m.Commitment),
	}
}

// CloneR7BcastMsg deep-copies a Round 7 broadcast message (SignRound7Message).
func CloneR7BcastMsg(m *tss.Message) *tss.Message {
	return cloneMessage(m, CloneSignRound7Message(m.Content.(*SignRound7Message)))
}

// -----------------------------------------------------------------
// Round 8 Message (Broadcast): decommitment of Ui/Ti
// -----------------------------------------------------------------

// CloneSignRound8Message deep-copies a SignRound8Message content.
func CloneSignRound8Message(m *SignRound8Message) *SignRound8Message {
	if m == nil {
		return nil
	}
	return &SignRound8Message{
		DeCommitment: cloneDeCommitment(m.DeCommitment),
	}
}

// CloneR8BcastMsg deep-copies a Round 8 broadcast message (SignRound8Message).
func CloneR8BcastMsg(m *tss.Message) *tss.Message {
	return cloneMessage(m, CloneSignRound8Message(m.Content.(*SignRound8Message)))
}

// -----------------------------------------------------------------
// Round 9 Message (Broadcast): partial signature share
// -----------------------------------------------------------------

// CloneSignRound9Message deep-copies a SignRound9Message content.
func CloneSignRound9Message(m *SignRound9Message) *SignRound9Message {
	if m == nil {
		return nil
	}
	return &SignRound9Message{
		S: cloneBigInt(m.S),
	}
}

// CloneR9BcastMsg deep-copies a Round 9 broadcast message (SignRound9Message).
func CloneR9BcastMsg(m *tss.Message) *tss.Message {
	return cloneMessage(m, CloneSignRound9Message(m.Content.(*SignRound9Message)))
}

// -----------------------------------------------------------------
// Broadcast/P2P slice cloning helpers
// -----------------------------------------------------------------

// CloneBcastSlice deep-copies a broadcast message slice using the
// provided per-message cloner.
func CloneBcastSlice(msgs []*tss.Message, cloner func(*tss.Message) *tss.Message) []*tss.Message {
	if msgs == nil {
		return nil
	}
	out := make([]*tss.Message, len(msgs))
	for i, m := range msgs {
		if m != nil {
			out[i] = cloner(m)
		}
	}
	return out
}

// CloneP2PSlice deep-copies a P2P message matrix (indexed as
// [recipient][sender]) using the provided per-message cloner.
func CloneP2PSlice(msgs [][]*tss.Message, cloner func(*tss.Message) *tss.Message) [][]*tss.Message {
	if msgs == nil {
		return nil
	}
	out := make([][]*tss.Message, len(msgs))
	for i, row := range msgs {
		if row != nil {
			out[i] = make([]*tss.Message, len(row))
			for j, m := range row {
				if m != nil {
					out[i][j] = cloner(m)
				}
			}
		}
	}
	return out
}
