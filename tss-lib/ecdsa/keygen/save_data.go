// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package keygen

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/hemilabs/x/tss-lib/v2/crypto"
	"github.com/hemilabs/x/tss-lib/v2/crypto/paillier"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

type (
	LocalPreParams struct {
		PaillierSK *paillier.PrivateKey // ski
		NTildei,
		H1i, H2i,
		Alpha, Beta,
		P, Q *big.Int
	}

	LocalSecrets struct {
		// secret fields (not shared, but stored locally)
		Xi, ShareID *big.Int // xi, kj
	}

	// Everything in LocalPartySaveData is saved locally to user's HD when done
	LocalPartySaveData struct {
		LocalPreParams
		LocalSecrets

		// original indexes (ki in signing preparation phase)
		Ks []*big.Int

		// n-tilde, h1, h2 for range proofs
		NTildej, H1j, H2j []*big.Int

		// public keys (Xj = uj*G for each Pj)
		BigXj       []*crypto.ECPoint     // Xj
		PaillierPKs []*paillier.PublicKey // pkj

		// used for test assertions (may be discarded)
		ECDSAPub *crypto.ECPoint // y
	}
)

func NewLocalPartySaveData(partyCount int) (saveData LocalPartySaveData) {
	saveData.Ks = make([]*big.Int, partyCount)
	saveData.NTildej = make([]*big.Int, partyCount)
	saveData.H1j, saveData.H2j = make([]*big.Int, partyCount), make([]*big.Int, partyCount)
	saveData.BigXj = make([]*crypto.ECPoint, partyCount)
	saveData.PaillierPKs = make([]*paillier.PublicKey, partyCount)
	return
}

func (preParams LocalPreParams) Validate() bool {
	return preParams.PaillierSK != nil &&
		preParams.NTildei != nil &&
		preParams.H1i != nil &&
		preParams.H2i != nil
}

// [FORK] ValidateWithProof: upstream only checked for nil fields. This hardened version
// adds algebraic consistency checks to verify that the pre-params are internally consistent:
// (1) NTilde = (2P+1)(2Q+1) — ensures NTilde is the product of safe primes derived from P, Q.
// (2) H2 = H1^Alpha mod NTilde — ensures the discrete-log relationship needed for DLN proofs.
// Without these checks, corrupted or tampered pre-params could silently produce invalid proofs
// that would be rejected by honest verifiers, wasting an entire keygen ceremony.
func (preParams LocalPreParams) ValidateWithProof() bool {
	if !(preParams.Validate() &&
		preParams.PaillierSK.P != nil &&
		preParams.PaillierSK.Q != nil &&
		preParams.Alpha != nil &&
		preParams.Beta != nil &&
		preParams.P != nil &&
		preParams.Q != nil) {
		return false
	}
	// [FORK] Defense-in-depth: P == Q would make NTilde = (2P+1)^2, a perfect square,
	// which completely breaks the DLN proof (the prover can trivially compute the order
	// of (Z/NTilde·Z)*). This condition is unreachable under normal operation: P and Q
	// are independently generated 1024-bit safe primes via GetRandomSafePrimesConcurrent.
	// The probability of collision is ~2^{-1003}. Retained to guard against RNG failure
	// or storage corruption.
	if preParams.P.Cmp(preParams.Q) == 0 {
		return false
	}
	// Verify P, Q are the Sophie Germain primes corresponding to NTilde
	// NTilde should equal (2*P+1) * (2*Q+1)
	safeP := new(big.Int).Mul(preParams.P, big.NewInt(2))
	safeP.Add(safeP, big.NewInt(1))
	safeQ := new(big.Int).Mul(preParams.Q, big.NewInt(2))
	safeQ.Add(safeQ, big.NewInt(1))
	expectedNTilde := new(big.Int).Mul(safeP, safeQ)
	if expectedNTilde.Cmp(preParams.NTildei) != 0 {
		return false
	}
	// Verify H2 = H1^Alpha mod NTilde
	expectedH2 := new(big.Int).Exp(preParams.H1i, preParams.Alpha, preParams.NTildei)
	if expectedH2.Cmp(preParams.H2i) != 0 {
		return false
	}
	return true
}

// [FORK] ValidateSaveData performs comprehensive validation of loaded ECDSA save data,
// checking for nil fields, array consistency, curve membership of public keys,
// and the Feldman VSS invariant (Xi·G == BigXj[ownIndex]).
//
// Call this after loading save data from storage and before using it in signing
// or resharing. Without this, corrupted or tampered save data could silently produce
// invalid signatures or proofs, which would be indistinguishable from a protocol
// failure at a remote party.
//
// Returns a descriptive error or nil if all checks pass.
func (saveData LocalPartySaveData) ValidateSaveData() error {
	// Secret fields.
	if saveData.Xi == nil || saveData.ShareID == nil {
		return errors.New("ValidateSaveData: Xi or ShareID is nil")
	}
	if saveData.ECDSAPub == nil {
		return errors.New("ValidateSaveData: ECDSAPub is nil")
	}

	// Array consistency.
	n := len(saveData.Ks)
	if n < 2 {
		return fmt.Errorf("ValidateSaveData: party count %d is less than 2", n)
	}
	if len(saveData.BigXj) != n || len(saveData.NTildej) != n ||
		len(saveData.H1j) != n || len(saveData.H2j) != n ||
		len(saveData.PaillierPKs) != n {
		return errors.New("ValidateSaveData: array length mismatch")
	}

	// Per-party field nil checks.
	for i := 0; i < n; i++ {
		if saveData.Ks[i] == nil {
			return fmt.Errorf("ValidateSaveData: Ks[%d] is nil", i)
		}
		if saveData.BigXj[i] == nil {
			return fmt.Errorf("ValidateSaveData: BigXj[%d] is nil", i)
		}
		if !saveData.BigXj[i].IsOnCurve() {
			return fmt.Errorf("ValidateSaveData: BigXj[%d] is not on curve", i)
		}
		if saveData.NTildej[i] == nil || saveData.H1j[i] == nil || saveData.H2j[i] == nil {
			return fmt.Errorf("ValidateSaveData: NTildej/H1j/H2j[%d] is nil", i)
		}
		if saveData.PaillierPKs[i] == nil {
			return fmt.Errorf("ValidateSaveData: PaillierPKs[%d] is nil", i)
		}
	}

	// Find own index from Ks using ShareID.
	ownIdx := -1
	for i, k := range saveData.Ks {
		if k.Cmp(saveData.ShareID) == 0 {
			ownIdx = i
			break
		}
	}
	if ownIdx == -1 {
		return errors.New("ValidateSaveData: ShareID not found in Ks")
	}

	// Feldman VSS invariant: Xi·G must equal BigXj[ownIndex].
	// [FORK] Guard Xi=0: ScalarBaseMult(0) panics (identity point). A zero Xi means
	// the party's secret share is trivially known.
	if saveData.Xi.Sign() == 0 {
		return errors.New("ValidateSaveData: Xi is zero")
	}
	ec := saveData.BigXj[ownIdx].Curve()
	xiG := crypto.ScalarBaseMult(ec, saveData.Xi)
	if !xiG.Equals(saveData.BigXj[ownIdx]) {
		return errors.New("ValidateSaveData: Feldman VSS check failed: Xi·G != BigXj[ownIndex]")
	}

	return nil
}

// BuildLocalSaveDataSubset re-creates the LocalPartySaveData to contain data for only the list of signing parties.
func BuildLocalSaveDataSubset(sourceData LocalPartySaveData, sortedIDs tss.SortedPartyIDs) LocalPartySaveData {
	keysToIndices := make(map[string]int, len(sourceData.Ks))
	for j, kj := range sourceData.Ks {
		keysToIndices[hex.EncodeToString(kj.Bytes())] = j
	}
	newData := NewLocalPartySaveData(sortedIDs.Len())
	newData.LocalPreParams = sourceData.LocalPreParams
	newData.LocalSecrets = sourceData.LocalSecrets
	newData.ECDSAPub = sourceData.ECDSAPub
	for j, id := range sortedIDs {
		savedIdx, ok := keysToIndices[hex.EncodeToString(id.Key)]
		if !ok {
			panic(errors.New("BuildLocalSaveDataSubset: unable to find a signer party in the local save data"))
		}
		newData.Ks[j] = sourceData.Ks[savedIdx]
		newData.NTildej[j] = sourceData.NTildej[savedIdx]
		newData.H1j[j] = sourceData.H1j[savedIdx]
		newData.H2j[j] = sourceData.H2j[savedIdx]
		newData.BigXj[j] = sourceData.BigXj[savedIdx]
		newData.PaillierPKs[j] = sourceData.PaillierPKs[savedIdx]
	}
	return newData
}
