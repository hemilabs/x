// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package keygen

import (
	"encoding/hex"
	"errors"
	"math/big"
	"sync"

	"github.com/hemilabs/x/tss-lib/v2/crypto/facproof"
	"github.com/hemilabs/x/tss-lib/v2/crypto/modproof"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

const (
	paillierBitsLen = 2048
)

func (round *round2) Start() *tss.Error {
	if round.started {
		return round.WrapError(errors.New("round already started"))
	}
	round.number = 2
	round.started = true
	round.resetOK()

	common.Logger.Debugf(
		"%s Setting up DLN verification with concurrency level of %d",
		round.PartyID(),
		round.Concurrency(),
	)
	dlnVerifier := NewDlnProofVerifier(round.Concurrency())

	i := round.PartyID().Index

	// [FORK] Comprehensive parameter validation battery. Upstream verifies DLN proofs and
	// performs basic structural checks (bit-length, H1!=H2, h1/h2 cross-party uniqueness).
	// We additionally add oddness, non-primality, non-perfect-square, H1/H2 != 1,
	// H1/H2 coprime with NTilde, N != NTilde, and Paillier-N/NTilde cross-party uniqueness.
	// These checks prevent a malicious party from using degenerate Paillier/Pedersen
	// parameters that would break ZK proof security (e.g., trivially factorable N).
	h1H2Map := make(map[string]struct{}, len(round.temp.kgRound1Messages)*2)
	paillierNMap := make(map[string]struct{}, len(round.temp.kgRound1Messages))
	nTildeMap := make(map[string]struct{}, len(round.temp.kgRound1Messages))
	dlnProof1FailCulprits := make([]*tss.PartyID, len(round.temp.kgRound1Messages))
	dlnProof2FailCulprits := make([]*tss.PartyID, len(round.temp.kgRound1Messages))
	wg := new(sync.WaitGroup)
	for j, msg := range round.temp.kgRound1Messages {
		r1msg := msg.Content().(*KGRound1Message)
		H1j, H2j, NTildej, paillierPKj := r1msg.UnmarshalH1(),
			r1msg.UnmarshalH2(),
			r1msg.UnmarshalNTilde(),
			r1msg.UnmarshalPaillierPK()
		if paillierPKj.N.BitLen() != paillierBitsLen {
			return round.WrapError(errors.New("got paillier modulus with insufficient bits for this party"), msg.GetFrom())
		}
		if paillierPKj.N.Bit(0) == 0 {
			return round.WrapError(errors.New("got even paillier modulus (trivially factorable)"), msg.GetFrom())
		}
		if paillierPKj.N.ProbablyPrime(20) {
			return round.WrapError(errors.New("got prime paillier modulus (degenerate Paillier)"), msg.GetFrom())
		}
		// Reject perfect squares: sqrt(N)^2 == N means N = p^2
		sqrtN := new(big.Int).Sqrt(paillierPKj.N)
		if new(big.Int).Mul(sqrtN, sqrtN).Cmp(paillierPKj.N) == 0 {
			return round.WrapError(errors.New("got perfect-square paillier modulus (trivially factorable)"), msg.GetFrom())
		}
		if H1j.Cmp(H2j) == 0 {
			return round.WrapError(errors.New("h1j and h2j were equal for this party"), msg.GetFrom())
		}
		if H1j.Cmp(big.NewInt(1)) == 0 || H2j.Cmp(big.NewInt(1)) == 0 {
			return round.WrapError(errors.New("h1j or h2j was 1 (degenerate Pedersen parameter)"), msg.GetFrom())
		}
		if NTildej.BitLen() != paillierBitsLen {
			return round.WrapError(errors.New("got NTildej with insufficient bits for this party"), msg.GetFrom())
		}
		if NTildej.Bit(0) == 0 {
			return round.WrapError(errors.New("got even NTildej (trivially factorable)"), msg.GetFrom())
		}
		if NTildej.ProbablyPrime(20) {
			return round.WrapError(errors.New("got prime NTildej (degenerate Pedersen parameters)"), msg.GetFrom())
		}
		sqrtNT := new(big.Int).Sqrt(NTildej)
		if new(big.Int).Mul(sqrtNT, sqrtNT).Cmp(NTildej) == 0 {
			return round.WrapError(errors.New("got perfect-square NTildej (trivially factorable)"), msg.GetFrom())
		}
		if paillierPKj.N.Cmp(NTildej) == 0 {
			return round.WrapError(errors.New("Paillier N must differ from NTilde"), msg.GetFrom())
		}
		// Pedersen parameters must be coprime with NTilde (non-trivial elements of Z*_NTilde)
		if new(big.Int).GCD(nil, nil, H1j, NTildej).Cmp(big.NewInt(1)) != 0 {
			return round.WrapError(errors.New("h1j is not coprime with NTildej"), msg.GetFrom())
		}
		if new(big.Int).GCD(nil, nil, H2j, NTildej).Cmp(big.NewInt(1)) != 0 {
			return round.WrapError(errors.New("h2j is not coprime with NTildej"), msg.GetFrom())
		}
		h1JHex, h2JHex := hex.EncodeToString(H1j.Bytes()), hex.EncodeToString(H2j.Bytes())
		if _, found := h1H2Map[h1JHex]; found {
			return round.WrapError(errors.New("this h1j was already used by another party"), msg.GetFrom())
		}
		if _, found := h1H2Map[h2JHex]; found {
			return round.WrapError(errors.New("this h2j was already used by another party"), msg.GetFrom())
		}
		h1H2Map[h1JHex], h1H2Map[h2JHex] = struct{}{}, struct{}{}
		// Reject duplicate Paillier moduli across parties
		paillierNHex := hex.EncodeToString(paillierPKj.N.Bytes())
		if _, found := paillierNMap[paillierNHex]; found {
			return round.WrapError(errors.New("this Paillier N was already used by another party"), msg.GetFrom())
		}
		paillierNMap[paillierNHex] = struct{}{}
		// Reject duplicate NTilde across parties
		nTildeHex := hex.EncodeToString(NTildej.Bytes())
		if _, found := nTildeMap[nTildeHex]; found {
			return round.WrapError(errors.New("this NTilde was already used by another party"), msg.GetFrom())
		}
		nTildeMap[nTildeHex] = struct{}{}

		// [FORK] DLN proof verification is gated by NoProofDLN(). In SNARK mode, classical
		// DLN proofs are replaced by per-participant SNARKs that cover the same properties.
		// ContextJ provides SSID domain separation to prevent cross-ceremony DLN proof replay.
		if !round.Params().NoProofDLN() {
			wg.Add(2)
			_j := j
			_msg := msg
			ContextJ := common.AppendBigIntToBytesSlice(round.temp.ssid, big.NewInt(int64(j)))

			dlnVerifier.VerifyDLNProof1(r1msg, ContextJ, H1j, H2j, NTildej, func(isValid bool) {
				if !isValid {
					dlnProof1FailCulprits[_j] = _msg.GetFrom()
				}
				wg.Done()
			})
			dlnVerifier.VerifyDLNProof2(r1msg, ContextJ, H2j, H1j, NTildej, func(isValid bool) {
				if !isValid {
					dlnProof2FailCulprits[_j] = _msg.GetFrom()
				}
				wg.Done()
			})
		}
	}
	wg.Wait()
	for _, culprit := range append(dlnProof1FailCulprits, dlnProof2FailCulprits...) {
		if culprit != nil {
			return round.WrapError(errors.New("dln proof verification failed"), culprit)
		}
	}
	// save NTilde_j, h1_j, h2_j, ...
	for j, msg := range round.temp.kgRound1Messages {
		if j == i {
			continue
		}
		r1msg := msg.Content().(*KGRound1Message)
		paillierPK, H1j, H2j, NTildej, KGC := r1msg.UnmarshalPaillierPK(),
			r1msg.UnmarshalH1(),
			r1msg.UnmarshalH2(),
			r1msg.UnmarshalNTilde(),
			r1msg.UnmarshalCommitment()
		round.save.PaillierPKs[j] = paillierPK // used in round 4
		round.save.NTildej[j] = NTildej
		round.save.H1j[j], round.save.H2j[j] = H1j, H2j
		round.temp.KGCs[j] = KGC
	}

	// 5. p2p send share ij to Pj
	shares := round.temp.shares
	// [FORK] ContextI: upstream also computes ContextI = SSID || i, but uses raw byte
	// concatenation (append). We use AppendBigIntToBytesSlice for length-prefixed encoding,
	// which prevents ambiguity when i has variable-length byte representations.
	ContextI := common.AppendBigIntToBytesSlice(round.temp.ssid, big.NewInt(int64(i)))
	for j, Pj := range round.Parties().IDs() {
		// [FORK] FacProof: upstream also gates generation by NoProofFac(), but sends a
		// zero-valued ProofFac when skipped. We send nil instead, which is cleaner and
		// avoids transmitting a structurally valid but semantically meaningless proof.
		var facProofObj *facproof.ProofFac
		if !round.Params().NoProofFac() {
			var err error
			facProofObj, err = facproof.NewProof(ContextI, round.EC(), round.save.PaillierSK.N, round.save.NTildej[j],
				round.save.H1j[j], round.save.H2j[j], round.save.PaillierSK.P, round.save.PaillierSK.Q, round.Rand())
			if err != nil {
				return round.WrapError(err, round.PartyID())
			}
		}
		r2msg1 := NewKGRound2Message1(Pj, round.PartyID(), shares[j], facProofObj)
		// do not send to this Pj, but store for round 3
		if j == i {
			round.temp.kgRound2Message1s[j] = r2msg1
			continue
		}
		round.out <- r2msg1
	}

	// 7. BROADCAST de-commitments of Shamir poly*G
	// [FORK] ModProof: upstream also gates generation by NoProofMod(), but sends a
	// zero-valued ProofMod when skipped. We send nil instead (same rationale as FacProof).
	var modProofObj *modproof.ProofMod
	if !round.Params().NoProofMod() {
		var err error
		modProofObj, err = modproof.NewProof(ContextI, round.save.PaillierSK.N,
			round.save.PaillierSK.P, round.save.PaillierSK.Q, round.Rand())
		if err != nil {
			return round.WrapError(err, round.PartyID())
		}
	}
	r2msg2 := NewKGRound2Message2(round.PartyID(), round.temp.deCommitPolyG, modProofObj)
	round.temp.kgRound2Message2s[i] = r2msg2
	round.out <- r2msg2

	return nil
}

func (round *round2) CanAccept(msg tss.ParsedMessage) bool {
	if _, ok := msg.Content().(*KGRound2Message1); ok {
		return !msg.IsBroadcast()
	}
	if _, ok := msg.Content().(*KGRound2Message2); ok {
		return msg.IsBroadcast()
	}
	return false
}

func (round *round2) Update() (bool, *tss.Error) {
	// guard - VERIFY de-commit for all Pj
	ret := true
	for j, msg := range round.temp.kgRound2Message1s {
		if round.ok[j] {
			continue
		}
		if msg == nil || !round.CanAccept(msg) {
			ret = false
			continue
		}
		msg2 := round.temp.kgRound2Message2s[j]
		if msg2 == nil || !round.CanAccept(msg2) {
			ret = false
			continue
		}
		round.ok[j] = true
	}
	return ret, nil
}

func (round *round2) NextRound() tss.Round {
	round.started = false
	return &round3{round}
}
