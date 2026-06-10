// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package resharing

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/hemilabs/x/tss/v3/tss"
)

// expectRound4ParamError calls ReshareRound4 on new-committee party 0 with the
// given R2 messages and asserts that the error contains the expected substring
// and that the culprit has the expected index.
func expectRound4ParamError(t *testing.T, fix *ReshareFixture, msgs []*tss.Message, wantErrSubstr string, wantCulpritIdx int) {
	t.Helper()
	_, err := ReshareRound4(context.Background(), fix.NewStates[0], msgs, fix.OldR3P2P[0], fix.OldR3Bcast)
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", wantErrSubstr)
	}
	if !strings.Contains(err.Error(), wantErrSubstr) {
		t.Fatalf("expected error containing %q, got: %v", wantErrSubstr, err)
	}
	requireCulprit(t, err, wantCulpritIdx)
}

// r4VictimIdx is the index of the party whose R2 message we corrupt.
// We pick party 1 (not party 0, which is the verifier).
const r4VictimIdx = 1

// ---------- Individual parameter validation tests ----------

func TestRound4RejectsPaillierNInsufficientBits(t *testing.T) {
	fix := setupThroughRound3(t)
	msgs := copyR2Msg1Slice(fix.NewR2Msg1s)
	bad := cloneDGRound2Message1(msgs[r4VictimIdx])
	bad.Content.(*DGRound2Message1).PaillierPK.N = big.NewInt(1023) // tiny
	msgs[r4VictimIdx] = bad
	expectRound4ParamError(t, fix, msgs, "paillier N insufficient bits", r4VictimIdx)
}

func TestRound4RejectsEvenPaillierN(t *testing.T) {
	fix := setupThroughRound3(t)
	msgs := copyR2Msg1Slice(fix.NewR2Msg1s)
	bad := cloneDGRound2Message1(msgs[r4VictimIdx])
	n := bad.Content.(*DGRound2Message1).PaillierPK.N
	n.SetBit(n, 0, 0)    // make even
	n.SetBit(n, 2047, 1) // keep >= 2048 bits
	msgs[r4VictimIdx] = bad
	expectRound4ParamError(t, fix, msgs, "even paillier N", r4VictimIdx)
}

func TestRound4RejectsPrimePaillierN(t *testing.T) {
	fix := setupThroughRound3(t)
	msgs := copyR2Msg1Slice(fix.NewR2Msg1s)
	bad := cloneDGRound2Message1(msgs[r4VictimIdx])
	// RFC 3526 2048-bit MODP prime.
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
	if prime.BitLen() != 2048 || !prime.ProbablyPrime(20) {
		t.Fatal("test setup: expected a 2048-bit prime")
	}
	bad.Content.(*DGRound2Message1).PaillierPK.N = prime
	msgs[r4VictimIdx] = bad
	expectRound4ParamError(t, fix, msgs, "prime paillier N", r4VictimIdx)
}

func TestRound4RejectsPerfectSquarePaillierN(t *testing.T) {
	fix := setupThroughRound3(t)
	msgs := copyR2Msg1Slice(fix.NewR2Msg1s)
	bad := cloneDGRound2Message1(msgs[r4VictimIdx])
	// Use the victim's own P to construct p*p (a perfect square).
	p := fix.NewStates[r4VictimIdx].save.P
	pp := new(big.Int).Mul(p, p)
	if pp.BitLen() < 2048 {
		q := fix.NewStates[r4VictimIdx].save.Q
		pp = new(big.Int).Mul(q, q)
	}
	if pp.BitLen() < 2048 {
		t.Skip("neither p*p nor q*q is >= 2048 bits; skipping")
	}
	bad.Content.(*DGRound2Message1).PaillierPK.N = pp
	msgs[r4VictimIdx] = bad
	expectRound4ParamError(t, fix, msgs, "perfect-square paillier N", r4VictimIdx)
}

func TestRound4RejectsH1EqualsH2(t *testing.T) {
	fix := setupThroughRound3(t)
	msgs := copyR2Msg1Slice(fix.NewR2Msg1s)
	bad := cloneDGRound2Message1(msgs[r4VictimIdx])
	content := bad.Content.(*DGRound2Message1)
	content.H2 = new(big.Int).Set(content.H1) // H1 == H2
	msgs[r4VictimIdx] = bad
	expectRound4ParamError(t, fix, msgs, "h1j == h2j", r4VictimIdx)
}

func TestRound4RejectsH1IsOne(t *testing.T) {
	fix := setupThroughRound3(t)
	msgs := copyR2Msg1Slice(fix.NewR2Msg1s)
	bad := cloneDGRound2Message1(msgs[r4VictimIdx])
	bad.Content.(*DGRound2Message1).H1 = big.NewInt(1)
	msgs[r4VictimIdx] = bad
	expectRound4ParamError(t, fix, msgs, "h1j or h2j is 1", r4VictimIdx)
}

func TestRound4RejectsH2IsOne(t *testing.T) {
	fix := setupThroughRound3(t)
	msgs := copyR2Msg1Slice(fix.NewR2Msg1s)
	bad := cloneDGRound2Message1(msgs[r4VictimIdx])
	bad.Content.(*DGRound2Message1).H2 = big.NewInt(1)
	msgs[r4VictimIdx] = bad
	expectRound4ParamError(t, fix, msgs, "h1j or h2j is 1", r4VictimIdx)
}

func TestRound4RejectsNTildeInsufficientBits(t *testing.T) {
	fix := setupThroughRound3(t)
	msgs := copyR2Msg1Slice(fix.NewR2Msg1s)
	bad := cloneDGRound2Message1(msgs[r4VictimIdx])
	bad.Content.(*DGRound2Message1).NTilde = big.NewInt(999) // tiny
	msgs[r4VictimIdx] = bad
	expectRound4ParamError(t, fix, msgs, "NTildej insufficient bits", r4VictimIdx)
}

func TestRound4RejectsEvenNTilde(t *testing.T) {
	fix := setupThroughRound3(t)
	msgs := copyR2Msg1Slice(fix.NewR2Msg1s)
	bad := cloneDGRound2Message1(msgs[r4VictimIdx])
	nt := bad.Content.(*DGRound2Message1).NTilde
	nt.SetBit(nt, 0, 0)    // make even
	nt.SetBit(nt, 2047, 1) // keep >= 2048 bits
	msgs[r4VictimIdx] = bad
	expectRound4ParamError(t, fix, msgs, "even NTildej", r4VictimIdx)
}

func TestRound4RejectsPrimeNTilde(t *testing.T) {
	fix := setupThroughRound3(t)
	msgs := copyR2Msg1Slice(fix.NewR2Msg1s)
	bad := cloneDGRound2Message1(msgs[r4VictimIdx])
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
	bad.Content.(*DGRound2Message1).NTilde = prime
	msgs[r4VictimIdx] = bad
	expectRound4ParamError(t, fix, msgs, "prime NTildej", r4VictimIdx)
}

func TestRound4RejectsPerfectSquareNTilde(t *testing.T) {
	fix := setupThroughRound3(t)
	msgs := copyR2Msg1Slice(fix.NewR2Msg1s)
	bad := cloneDGRound2Message1(msgs[r4VictimIdx])
	p := fix.NewStates[r4VictimIdx].save.P
	pp := new(big.Int).Mul(p, p)
	if pp.BitLen() < 2048 {
		q := fix.NewStates[r4VictimIdx].save.Q
		pp = new(big.Int).Mul(q, q)
	}
	if pp.BitLen() < 2048 {
		t.Skip("neither p*p nor q*q is >= 2048 bits; skipping")
	}
	bad.Content.(*DGRound2Message1).NTilde = pp
	msgs[r4VictimIdx] = bad
	expectRound4ParamError(t, fix, msgs, "perfect-square NTildej", r4VictimIdx)
}

func TestRound4RejectsPaillierNEqualsNTilde(t *testing.T) {
	fix := setupThroughRound3(t)
	msgs := copyR2Msg1Slice(fix.NewR2Msg1s)
	bad := cloneDGRound2Message1(msgs[r4VictimIdx])
	content := bad.Content.(*DGRound2Message1)
	content.NTilde = new(big.Int).Set(content.PaillierPK.N)
	msgs[r4VictimIdx] = bad
	expectRound4ParamError(t, fix, msgs, "paillier N == NTilde", r4VictimIdx)
}

func TestRound4RejectsH1NotCoprimeNTilde(t *testing.T) {
	fix := setupThroughRound3(t)
	msgs := copyR2Msg1Slice(fix.NewR2Msg1s)
	bad := cloneDGRound2Message1(msgs[r4VictimIdx])
	content := bad.Content.(*DGRound2Message1)
	// NTilde = (2P+1)(2Q+1).  Set H1 = 2P+1 so gcd(H1, NTilde) != 1.
	pPrep := fix.NewStates[r4VictimIdx].save.P
	safeP := new(big.Int).Mul(pPrep, big.NewInt(2))
	safeP.Add(safeP, big.NewInt(1))
	content.H1 = safeP
	msgs[r4VictimIdx] = bad
	expectRound4ParamError(t, fix, msgs, "h1j not coprime with NTildej", r4VictimIdx)
}

func TestRound4RejectsH2NotCoprimeNTilde(t *testing.T) {
	fix := setupThroughRound3(t)
	msgs := copyR2Msg1Slice(fix.NewR2Msg1s)
	bad := cloneDGRound2Message1(msgs[r4VictimIdx])
	content := bad.Content.(*DGRound2Message1)
	// Same approach: set H2 to a factor of NTilde.
	pPrep := fix.NewStates[r4VictimIdx].save.P
	safeP := new(big.Int).Mul(pPrep, big.NewInt(2))
	safeP.Add(safeP, big.NewInt(1))
	content.H2 = safeP
	msgs[r4VictimIdx] = bad
	expectRound4ParamError(t, fix, msgs, "h2j not coprime with NTildej", r4VictimIdx)
}

func TestRound4RejectsDuplicateH1(t *testing.T) {
	fix := setupThroughRound3(t)
	msgs := copyR2Msg1Slice(fix.NewR2Msg1s)
	// Clone party 2's message and set its H1 to party 0's H1.
	bad := cloneDGRound2Message1(msgs[2])
	party0Content := msgs[0].Content.(*DGRound2Message1)
	bad.Content.(*DGRound2Message1).H1 = new(big.Int).Set(party0Content.H1)
	msgs[2] = bad
	expectRound4ParamError(t, fix, msgs, "duplicate h1j", 2)
}

func TestRound4RejectsDuplicateH2(t *testing.T) {
	fix := setupThroughRound3(t)
	msgs := copyR2Msg1Slice(fix.NewR2Msg1s)
	// Clone party 2's message and set its H2 to party 0's H2.
	bad := cloneDGRound2Message1(msgs[2])
	party0Content := msgs[0].Content.(*DGRound2Message1)
	bad.Content.(*DGRound2Message1).H2 = new(big.Int).Set(party0Content.H2)
	msgs[2] = bad
	expectRound4ParamError(t, fix, msgs, "duplicate h2j", 2)
}

func TestRound4RejectsDuplicatePaillierN(t *testing.T) {
	fix := setupThroughRound3(t)
	msgs := copyR2Msg1Slice(fix.NewR2Msg1s)
	// Clone party 2's message and set its PaillierPK.N to party 0's N.
	bad := cloneDGRound2Message1(msgs[2])
	party0Content := msgs[0].Content.(*DGRound2Message1)
	bad.Content.(*DGRound2Message1).PaillierPK.N = new(big.Int).Set(party0Content.PaillierPK.N)
	msgs[2] = bad
	expectRound4ParamError(t, fix, msgs, "duplicate modulus (paillier N)", 2)
}

func TestRound4RejectsDuplicateNTilde(t *testing.T) {
	fix := setupThroughRound3(t)
	msgs := copyR2Msg1Slice(fix.NewR2Msg1s)
	// Clone party 2's message and set its NTilde to party 0's NTilde.
	bad := cloneDGRound2Message1(msgs[2])
	party0Content := msgs[0].Content.(*DGRound2Message1)
	bad.Content.(*DGRound2Message1).NTilde = new(big.Int).Set(party0Content.NTilde)
	msgs[2] = bad
	expectRound4ParamError(t, fix, msgs, "duplicate modulus (NTilde)", 2)
}

// TestRound4RejectsCrossDuplicateH1H2 verifies that the shared h1H2Map
// catches the case where party A's H1 equals party B's H2.
func TestRound4RejectsCrossDuplicateH1H2(t *testing.T) {
	fix := setupThroughRound3(t)
	msgs := copyR2Msg1Slice(fix.NewR2Msg1s)
	// Set party 2's H1 to party 0's H2. Since party 0 is processed first,
	// its H2 is already in the shared map when party 2's H1 is checked.
	bad := cloneDGRound2Message1(msgs[2])
	party0Content := msgs[0].Content.(*DGRound2Message1)
	bad.Content.(*DGRound2Message1).H1 = new(big.Int).Set(party0Content.H2)
	msgs[2] = bad
	expectRound4ParamError(t, fix, msgs, "duplicate h1j", 2)
}

// TestRound4RejectsCrossPartyPaillierNEqualsNTilde verifies that the merged
// modulusMap catches cross-party collisions where Party A's PaillierN equals
// Party B's NTilde.
func TestRound4RejectsCrossPartyPaillierNEqualsNTilde(t *testing.T) {
	fix := setupThroughRound3(t)
	msgs := copyR2Msg1Slice(fix.NewR2Msg1s)
	// Set party 2's PaillierPK.N to party 0's NTilde.
	bad := cloneDGRound2Message1(msgs[2])
	party0Content := msgs[0].Content.(*DGRound2Message1)
	bad.Content.(*DGRound2Message1).PaillierPK.N = new(big.Int).Set(party0Content.NTilde)
	msgs[2] = bad
	expectRound4ParamError(t, fix, msgs, "duplicate modulus (paillier N)", 2)
}

// TestRound4RejectsCrossPartyNTildeEqualsPaillierN verifies the reverse
// direction: Party A's NTilde equals Party B's PaillierN.
func TestRound4RejectsCrossPartyNTildeEqualsPaillierN(t *testing.T) {
	fix := setupThroughRound3(t)
	msgs := copyR2Msg1Slice(fix.NewR2Msg1s)
	// Set party 2's NTilde to party 0's PaillierN.
	bad := cloneDGRound2Message1(msgs[2])
	party0Content := msgs[0].Content.(*DGRound2Message1)
	bad.Content.(*DGRound2Message1).NTilde = new(big.Int).Set(party0Content.PaillierPK.N)
	msgs[2] = bad
	expectRound4ParamError(t, fix, msgs, "duplicate modulus (NTilde)", 2)
}
