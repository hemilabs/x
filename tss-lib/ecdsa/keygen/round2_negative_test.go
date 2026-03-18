// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/hemilabs/x/tss-lib/v3/crypto/paillier"
	"github.com/hemilabs/x/tss-lib/v3/tss"
)

// round1Fixture holds the state and messages produced by a valid 3-party
// keygen Round1.  Negative tests corrupt one message and call Round2 on
// party 0 to assert the expected rejection.
type round1Fixture struct {
	states  []*KeygenState
	allR1   []*tss.Message // allR1[j] = party j's broadcast
	n       int
	pIDs    tss.SortedPartyIDs
	peerCtx *tss.PeerContext
}

// setupRound1ForNegativeTests runs a real 3-party keygen Round1 with
// all DLN proofs disabled (NoProofDLN mode) so that corrupted parameters
// trigger the parameter-validation checks rather than DLN proof failures.
// Returns a fixture whose allR1 slice the caller can mutate before
// calling Round2 on states[0].
func setupRound1ForNegativeTests(t *testing.T) *round1Fixture {
	t.Helper()
	const n = 3
	const threshold = 1 // 2-of-3

	preParams := make([]LocalPreParams, n)
	for i := 0; i < n; i++ {
		pp, err := GeneratePreParams(5 * time.Minute)
		if err != nil {
			t.Fatalf("GeneratePreParams[%d]: %v", i, err)
		}
		preParams[i] = *pp
	}

	pIDs := tss.GenerateTestPartyIDs(n)
	peerCtx := tss.NewPeerContext(pIDs)

	states := make([]*KeygenState, n)
	allR1 := make([]*tss.Message, n)
	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.S256(), peerCtx, pIDs[i], n, threshold)
		params.SetNoProofDLN()
		params.SetNoProofMod()
		params.SetNoProofFac()
		st, out, err := Round1(context.Background(), params, preParams[i])
		if err != nil {
			t.Fatalf("Round1[%d]: %v", i, err)
		}
		states[i] = st
		allR1[i] = out.Messages[0]
	}

	return &round1Fixture{
		states:  states,
		allR1:   allR1,
		n:       n,
		pIDs:    pIDs,
		peerCtx: peerCtx,
	}
}

// cloneR1Msgs returns a shallow copy of the allR1 slice so that the
// caller can replace individual entries without affecting the original.
func cloneR1Msgs(msgs []*tss.Message) []*tss.Message {
	out := make([]*tss.Message, len(msgs))
	copy(out, msgs)
	return out
}

// cloneR1MsgContent deep-copies the KGRound1Message content of
// allR1[idx] so that field mutations do not corrupt the original.
func cloneR1MsgContent(msg *tss.Message) *tss.Message {
	orig := msg.Content.(*KGRound1Message)
	clone := &KGRound1Message{
		Commitment: new(big.Int).Set(orig.Commitment),
		PaillierPK: &paillier.PublicKey{N: new(big.Int).Set(orig.PaillierPK.N)},
		NTilde:     new(big.Int).Set(orig.NTilde),
		H1:         new(big.Int).Set(orig.H1),
		H2:         new(big.Int).Set(orig.H2),
		DLNProof1:  orig.DLNProof1,
		DLNProof2:  orig.DLNProof2,
	}
	return &tss.Message{
		From:        msg.From,
		To:          msg.To,
		IsBroadcast: msg.IsBroadcast,
		Content:     clone,
	}
}

// expectRound2Error calls Round2 on party 0 with the given messages and
// asserts that the returned error contains the expected substring.
func expectRound2Error(t *testing.T, fix *round1Fixture, msgs []*tss.Message, wantErrSubstr string) {
	t.Helper()
	_, err := Round2(context.Background(), fix.states[0], msgs)
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", wantErrSubstr)
	}
	if !strings.Contains(err.Error(), wantErrSubstr) {
		t.Fatalf("expected error containing %q, got: %v", wantErrSubstr, err)
	}
}

// victimIndex returns the index of a party whose message we corrupt.
// We pick party 1 (not party 0, which is the verifier).
const victimIdx = 1

// --- Individual parameter validation tests ---

func TestRound2RejectsPaillierNInsufficientBits(t *testing.T) {
	fix := setupRound1ForNegativeTests(t)
	msgs := cloneR1Msgs(fix.allR1)
	bad := cloneR1MsgContent(msgs[victimIdx])
	bad.Content.(*KGRound1Message).PaillierPK.N = big.NewInt(1023) // tiny, far below 2048 bits
	msgs[victimIdx] = bad
	expectRound2Error(t, fix, msgs, "paillier modulus insufficient bits")
}

func TestRound2RejectsEvenPaillierN(t *testing.T) {
	fix := setupRound1ForNegativeTests(t)
	msgs := cloneR1Msgs(fix.allR1)
	bad := cloneR1MsgContent(msgs[victimIdx])
	// Make N even but keep 2048 bits: set bit 0 to 0.
	n := bad.Content.(*KGRound1Message).PaillierPK.N
	n.SetBit(n, 0, 0)
	// Ensure still 2048 bits by setting bit 2047.
	n.SetBit(n, 2047, 1)
	msgs[victimIdx] = bad
	expectRound2Error(t, fix, msgs, "even paillier modulus")
}

func TestRound2RejectsPrimePaillierN(t *testing.T) {
	fix := setupRound1ForNegativeTests(t)
	msgs := cloneR1Msgs(fix.allR1)
	bad := cloneR1MsgContent(msgs[victimIdx])
	// Use a known 2048-bit prime.  We construct one by taking a random
	// 2048-bit odd number and finding the next prime.  For test
	// determinism, use a fixed seed.
	prime, _ := new(big.Int).SetString(
		"FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD1"+
			"29024E088A67CC74020BBEA63B139B22514A08798E3404DD"+
			"EF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245"+
			"E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7ED"+
			"EE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3D"+
			"C2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F"+
			"83655D23DCA3AD961C62F356208552BB9ED529077096966D"+
			"670C354E4ABC9804F1746C08CA18217C32905E462E36CE3B"+
			"E39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9"+
			"DE2BCBF6955817183995497CEA956AE515D2261898FA0510"+
			"15728E5A8AACAA68FFFFFFFFFFFFFFFF", 16)
	// This is the 2048-bit MODP prime from RFC 3526. It is indeed prime.
	if prime.BitLen() != 2048 || !prime.ProbablyPrime(20) {
		t.Fatal("test setup: expected a 2048-bit prime")
	}
	bad.Content.(*KGRound1Message).PaillierPK.N = prime
	msgs[victimIdx] = bad
	expectRound2Error(t, fix, msgs, "prime paillier modulus")
}

func TestRound2RejectsPerfectSquarePaillierN(t *testing.T) {
	fix := setupRound1ForNegativeTests(t)
	msgs := cloneR1Msgs(fix.allR1)
	bad := cloneR1MsgContent(msgs[victimIdx])
	// Use a 1024-bit prime p, then set N = p*p which should be 2048 bits.
	// Try P first, then Q — one of them will produce a 2048-bit square.
	p := fix.states[0].save.PaillierSK.P
	pp := new(big.Int).Mul(p, p)
	if pp.BitLen() != 2048 {
		q := fix.states[0].save.PaillierSK.Q
		pp = new(big.Int).Mul(q, q)
	}
	if pp.BitLen() != 2048 {
		t.Skip("neither p*p nor q*q is 2048 bits; skipping")
	}
	bad.Content.(*KGRound1Message).PaillierPK.N = pp
	msgs[victimIdx] = bad
	expectRound2Error(t, fix, msgs, "perfect-square paillier modulus")
}

func TestRound2RejectsH1EqualsH2(t *testing.T) {
	fix := setupRound1ForNegativeTests(t)
	msgs := cloneR1Msgs(fix.allR1)
	bad := cloneR1MsgContent(msgs[victimIdx])
	content := bad.Content.(*KGRound1Message)
	content.H2 = new(big.Int).Set(content.H1) // H1 == H2
	msgs[victimIdx] = bad
	expectRound2Error(t, fix, msgs, "h1j == h2j")
}

func TestRound2RejectsH1IsOne(t *testing.T) {
	fix := setupRound1ForNegativeTests(t)
	msgs := cloneR1Msgs(fix.allR1)
	bad := cloneR1MsgContent(msgs[victimIdx])
	bad.Content.(*KGRound1Message).H1 = big.NewInt(1)
	msgs[victimIdx] = bad
	expectRound2Error(t, fix, msgs, "h1j or h2j is 1")
}

func TestRound2RejectsH2IsOne(t *testing.T) {
	fix := setupRound1ForNegativeTests(t)
	msgs := cloneR1Msgs(fix.allR1)
	bad := cloneR1MsgContent(msgs[victimIdx])
	bad.Content.(*KGRound1Message).H2 = big.NewInt(1)
	msgs[victimIdx] = bad
	expectRound2Error(t, fix, msgs, "h1j or h2j is 1")
}

func TestRound2RejectsNTildeInsufficientBits(t *testing.T) {
	fix := setupRound1ForNegativeTests(t)
	msgs := cloneR1Msgs(fix.allR1)
	bad := cloneR1MsgContent(msgs[victimIdx])
	bad.Content.(*KGRound1Message).NTilde = big.NewInt(999) // tiny
	msgs[victimIdx] = bad
	expectRound2Error(t, fix, msgs, "NTildej insufficient bits")
}

func TestRound2RejectsEvenNTilde(t *testing.T) {
	fix := setupRound1ForNegativeTests(t)
	msgs := cloneR1Msgs(fix.allR1)
	bad := cloneR1MsgContent(msgs[victimIdx])
	nt := bad.Content.(*KGRound1Message).NTilde
	nt.SetBit(nt, 0, 0)    // make even
	nt.SetBit(nt, 2047, 1) // keep 2048 bits
	msgs[victimIdx] = bad
	expectRound2Error(t, fix, msgs, "even NTildej")
}

func TestRound2RejectsPrimeNTilde(t *testing.T) {
	fix := setupRound1ForNegativeTests(t)
	msgs := cloneR1Msgs(fix.allR1)
	bad := cloneR1MsgContent(msgs[victimIdx])
	// Reuse the RFC 3526 2048-bit prime.
	prime, _ := new(big.Int).SetString(
		"FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD1"+
			"29024E088A67CC74020BBEA63B139B22514A08798E3404DD"+
			"EF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245"+
			"E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7ED"+
			"EE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3D"+
			"C2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F"+
			"83655D23DCA3AD961C62F356208552BB9ED529077096966D"+
			"670C354E4ABC9804F1746C08CA18217C32905E462E36CE3B"+
			"E39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9"+
			"DE2BCBF6955817183995497CEA956AE515D2261898FA0510"+
			"15728E5A8AACAA68FFFFFFFFFFFFFFFF", 16)
	bad.Content.(*KGRound1Message).NTilde = prime
	msgs[victimIdx] = bad
	expectRound2Error(t, fix, msgs, "prime NTildej")
}

func TestRound2RejectsPerfectSquareNTilde(t *testing.T) {
	fix := setupRound1ForNegativeTests(t)
	msgs := cloneR1Msgs(fix.allR1)
	bad := cloneR1MsgContent(msgs[victimIdx])
	p := fix.states[0].save.PaillierSK.P
	pp := new(big.Int).Mul(p, p)
	if pp.BitLen() != 2048 {
		q := fix.states[0].save.PaillierSK.Q
		pp = new(big.Int).Mul(q, q)
	}
	if pp.BitLen() != 2048 {
		t.Skip("neither p*p nor q*q is 2048 bits; skipping")
	}
	bad.Content.(*KGRound1Message).NTilde = pp
	msgs[victimIdx] = bad
	expectRound2Error(t, fix, msgs, "perfect-square NTildej")
}

func TestRound2RejectsPaillierNEqualsNTilde(t *testing.T) {
	fix := setupRound1ForNegativeTests(t)
	msgs := cloneR1Msgs(fix.allR1)
	bad := cloneR1MsgContent(msgs[victimIdx])
	content := bad.Content.(*KGRound1Message)
	// Set NTilde = PaillierPK.N (both already 2048-bit semiprimes).
	content.NTilde = new(big.Int).Set(content.PaillierPK.N)
	msgs[victimIdx] = bad
	expectRound2Error(t, fix, msgs, "paillier N == NTilde")
}

func TestRound2RejectsH1NotCoprimeNTilde(t *testing.T) {
	fix := setupRound1ForNegativeTests(t)
	msgs := cloneR1Msgs(fix.allR1)
	bad := cloneR1MsgContent(msgs[victimIdx])
	content := bad.Content.(*KGRound1Message)
	// NTilde = safePrime_P * safePrime_Q.  We grab one of the safe prime
	// factors from the victim's pre-params (party 1) and set H1 to it,
	// so gcd(H1, NTilde) != 1.
	//
	// party 1's NTilde = (2*P+1)*(2*Q+1).  We need a factor.  The
	// simplest approach: set H1 = safeP = 2*P+1.
	pPrep := fix.states[victimIdx].save.P
	safeP := new(big.Int).Mul(pPrep, big.NewInt(2))
	safeP.Add(safeP, big.NewInt(1))
	content.H1 = safeP
	msgs[victimIdx] = bad
	expectRound2Error(t, fix, msgs, "h1j not coprime with NTildej")
}

func TestRound2RejectsH2NotCoprimeNTilde(t *testing.T) {
	fix := setupRound1ForNegativeTests(t)
	msgs := cloneR1Msgs(fix.allR1)
	bad := cloneR1MsgContent(msgs[victimIdx])
	content := bad.Content.(*KGRound1Message)
	// Same approach as H1 test but corrupt H2.
	pPrep := fix.states[victimIdx].save.P
	safeP := new(big.Int).Mul(pPrep, big.NewInt(2))
	safeP.Add(safeP, big.NewInt(1))
	// Keep H1 valid, set H2 = safeP (a factor of NTilde).
	content.H2 = safeP
	msgs[victimIdx] = bad
	expectRound2Error(t, fix, msgs, "h2j not coprime with NTildej")
}

func TestRound2RejectsDuplicateH2(t *testing.T) {
	fix := setupRound1ForNegativeTests(t)
	msgs := cloneR1Msgs(fix.allR1)
	// Clone party 2's message and set its H2 to party 0's H2.
	bad := cloneR1MsgContent(msgs[2])
	party0Content := msgs[0].Content.(*KGRound1Message)
	bad.Content.(*KGRound1Message).H2 = new(big.Int).Set(party0Content.H2)
	msgs[2] = bad
	expectRound2Error(t, fix, msgs, "duplicate h2j")
}

func TestRound2RejectsDuplicateH1(t *testing.T) {
	fix := setupRound1ForNegativeTests(t)
	msgs := cloneR1Msgs(fix.allR1)
	// Clone party 2's message and set its H1 to party 0's H1.
	bad := cloneR1MsgContent(msgs[2])
	party0Content := msgs[0].Content.(*KGRound1Message)
	bad.Content.(*KGRound1Message).H1 = new(big.Int).Set(party0Content.H1)
	msgs[2] = bad
	expectRound2Error(t, fix, msgs, "duplicate h1j")
}

func TestRound2RejectsDuplicatePaillierN(t *testing.T) {
	fix := setupRound1ForNegativeTests(t)
	msgs := cloneR1Msgs(fix.allR1)
	// Clone party 2's message and set its PaillierPK.N to party 0's N.
	bad := cloneR1MsgContent(msgs[2])
	party0Content := msgs[0].Content.(*KGRound1Message)
	bad.Content.(*KGRound1Message).PaillierPK.N = new(big.Int).Set(party0Content.PaillierPK.N)
	// Also need to update NTilde to avoid hitting "paillier N == NTilde" first
	// (the original NTilde is party 2's, which is different from party 0's N).
	// Actually the loop checks paillierN == NTilde within the SAME message,
	// and we only changed paillierN, so NTilde is still party 2's original
	// which differs from party 0's N.  Should be fine.
	msgs[2] = bad
	expectRound2Error(t, fix, msgs, "duplicate Paillier N")
}

func TestRound2RejectsDuplicateNTilde(t *testing.T) {
	fix := setupRound1ForNegativeTests(t)
	msgs := cloneR1Msgs(fix.allR1)
	// Clone party 2's message and set its NTilde to party 0's NTilde.
	bad := cloneR1MsgContent(msgs[2])
	party0Content := msgs[0].Content.(*KGRound1Message)
	bad.Content.(*KGRound1Message).NTilde = new(big.Int).Set(party0Content.NTilde)
	msgs[2] = bad
	expectRound2Error(t, fix, msgs, "duplicate NTilde")
}

// TestRound2RejectsCrossDuplicateH1H2 verifies that the shared h1H2Map
// catches the case where party A's H1 equals party B's H2.
func TestRound2RejectsCrossDuplicateH1H2(t *testing.T) {
	fix := setupRound1ForNegativeTests(t)
	msgs := cloneR1Msgs(fix.allR1)
	// Set party 2's H1 to party 0's H2. Since party 0 is processed first,
	// its H2 is already in the shared map when party 2's H1 is checked.
	bad := cloneR1MsgContent(msgs[2])
	party0Content := msgs[0].Content.(*KGRound1Message)
	bad.Content.(*KGRound1Message).H1 = new(big.Int).Set(party0Content.H2)
	msgs[2] = bad
	expectRound2Error(t, fix, msgs, "duplicate h1j")
}

// TestRound2RejectsOversizedPaillierN verifies that the != 2048 check
// rejects oversized N (not just undersized).
func TestRound2RejectsOversizedPaillierN(t *testing.T) {
	fix := setupRound1ForNegativeTests(t)
	msgs := cloneR1Msgs(fix.allR1)
	bad := cloneR1MsgContent(msgs[victimIdx])
	n := bad.Content.(*KGRound1Message).PaillierPK.N
	n.SetBit(n, 2048, 1) // set bit 2048 → 2049-bit value
	msgs[victimIdx] = bad
	expectRound2Error(t, fix, msgs, "paillier modulus insufficient bits")
}

// TestRound2RejectsOversizedNTilde verifies that the != 2048 check
// rejects oversized NTilde.
func TestRound2RejectsOversizedNTilde(t *testing.T) {
	fix := setupRound1ForNegativeTests(t)
	msgs := cloneR1Msgs(fix.allR1)
	bad := cloneR1MsgContent(msgs[victimIdx])
	nt := bad.Content.(*KGRound1Message).NTilde
	nt.SetBit(nt, 2048, 1) // set bit 2048 → 2049-bit value
	msgs[victimIdx] = bad
	expectRound2Error(t, fix, msgs, "NTildej insufficient bits")
}
