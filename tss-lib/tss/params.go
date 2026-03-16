// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package tss

import (
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"io"
	"runtime"
	"time"
)

type (
	Parameters struct {
		ec                  elliptic.Curve
		partyID             *PartyID
		parties             *PeerContext
		partyCount          int
		threshold           int
		concurrency         int
		safePrimeGenTimeout time.Duration
		// proof session info
		// [FORK] Changed from `nonce int` to `nonce uint`. Upstream uses signed int,
		// which allows negative nonces that are nonsensical for a session counter and
		// could produce ambiguous SSID encodings (negative two's-complement vs positive).
		nonce uint
		// for keygen
		noProofMod bool
		noProofFac bool
		noProofDLN bool // [FORK] Added: allows disabling DLN proofs when replaced by SNARK coverage
		// ceremonyID binds SSID to a specific ceremony instance.
		// Two concurrent ceremonies with the same parties produce
		// different SSIDs because their ceremonyIDs differ.  The
		// nonce field is orthogonal: it tracks retry attempts of
		// the same logical ceremony.
		ceremonyID []byte
		// random sources
		partialKeyRand, rand io.Reader
	}

	ReSharingParameters struct {
		*Parameters
		newParties    *PeerContext
		newPartyCount int
		newThreshold  int
	}
)

const (
	defaultSafePrimeGenTimeout = 5 * time.Minute
)

// Exported, used in `tss` client
//
// [FORK] Added parameter validation panics. Upstream silently accepts invalid
// partyCount/threshold (e.g., threshold >= partyCount), which violates the
// (t,n)-threshold assumption and leads to subtle failures deeper in the protocol.
func NewParameters(ec elliptic.Curve, ctx *PeerContext, partyID *PartyID, partyCount, threshold int) *Parameters {
	if partyCount < 1 {
		panic(fmt.Sprintf("NewParameters: partyCount must be >= 1, got %d", partyCount))
	}
	if threshold < 0 || threshold >= partyCount {
		panic(fmt.Sprintf("NewParameters: threshold must be in [0, partyCount), got threshold=%d, partyCount=%d", threshold, partyCount))
	}
	return &Parameters{
		ec:                  ec,
		parties:             ctx,
		partyID:             partyID,
		partyCount:          partyCount,
		threshold:           threshold,
		concurrency:         runtime.GOMAXPROCS(0),
		safePrimeGenTimeout: defaultSafePrimeGenTimeout,
		partialKeyRand:      rand.Reader,
		rand:                rand.Reader,
	}
}

// EC returns the elliptic curve for this protocol run.
func (params *Parameters) EC() elliptic.Curve {
	return params.ec
}

// Parties returns the peer context with all party IDs.
func (params *Parameters) Parties() *PeerContext {
	return params.parties
}

// PartyID returns this party's ID.
func (params *Parameters) PartyID() *PartyID {
	return params.partyID
}

// PartyCount returns the total number of parties.
func (params *Parameters) PartyCount() int {
	return params.partyCount
}

// Threshold returns the signing threshold (t in t+1-of-n).
func (params *Parameters) Threshold() int {
	return params.threshold
}

// Concurrency returns the parallelism level for proof verification.
func (params *Parameters) Concurrency() int {
	return params.concurrency
}

// SafePrimeGenTimeout returns the timeout for safe prime generation.
func (params *Parameters) SafePrimeGenTimeout() time.Duration {
	return params.safePrimeGenTimeout
}

// The concurrency level must be >= 1.
func (params *Parameters) SetConcurrency(concurrency int) {
	params.concurrency = concurrency
}

// SetSafePrimeGenTimeout sets the timeout for safe prime generation.
func (params *Parameters) SetSafePrimeGenTimeout(timeout time.Duration) {
	params.safePrimeGenTimeout = timeout
}

// NoProofMod returns true if modular proof verification is disabled.
func (params *Parameters) NoProofMod() bool {
	return params.noProofMod
}

// NoProofFac returns true if factorization proof verification is disabled.
func (params *Parameters) NoProofFac() bool {
	return params.noProofFac
}

// [FORK] Added SSIDNonce getter/setter pair. Upstream has no SSID nonce
// mechanism; all ceremony attempts share the same session ID, enabling
// cross-attempt proof replay attacks.
func (params *Parameters) SSIDNonce() uint {
	return params.nonce
}

// SetSSIDNonce sets the session nonce for SSID domain separation.
// Each retry of a ceremony MUST use a distinct nonce to prevent
// cross-attempt proof replay.
func (params *Parameters) SetSSIDNonce(n uint) {
	params.nonce = n
}

// CeremonyID returns the ceremony identifier bound into the SSID.
func (params *Parameters) CeremonyID() []byte {
	return params.ceremonyID
}

// SetCeremonyID binds a ceremony identifier into the SSID hash.
// This distinguishes concurrent ceremonies that share the same
// parties, threshold, and curve.  The caller MUST set this before
// starting any round.
func (params *Parameters) SetCeremonyID(id []byte) {
	params.ceremonyID = id
}

// NoProofDLN returns true if DLN proof verification is disabled.
func (params *Parameters) NoProofDLN() bool {
	return params.noProofDLN
}

// SetNoProofDLN disables DLN proof generation and validation.
// WARNING: Only use in on-chain SNARK mode where DLN proofs are
// replaced by a SNARK covering the same security properties.
func (params *Parameters) SetNoProofDLN() {
	params.noProofDLN = true
}

// SetNoProofMod disables MOD proof generation and validation.
// WARNING: This is for testing/development ONLY. Disabling MOD proofs in
// production removes a critical security check that prevents a malicious party
// from using a non-safe-prime Paillier modulus, which breaks the security
// assumptions of the GG18 protocol. Never use in production deployments.
func (params *Parameters) SetNoProofMod() {
	params.noProofMod = true
}

// SetNoProofFac disables FAC proof generation and validation.
// WARNING: This is for testing/development ONLY. Disabling FAC proofs in
// production removes a critical security check that proves the prover's
// Paillier key factors are sufficiently large, which is required for the
// MtA (multiplicative-to-additive) protocol's soundness. Never use in
// production deployments.
func (params *Parameters) SetNoProofFac() {
	params.noProofFac = true
}

// PartialKeyRand returns the randomness source for partial key generation.
func (params *Parameters) PartialKeyRand() io.Reader {
	return params.partialKeyRand
}

// Rand returns the randomness source for protocol operations.
func (params *Parameters) Rand() io.Reader {
	return params.rand
}

// SetPartialKeyRand sets the randomness source for partial key generation.
func (params *Parameters) SetPartialKeyRand(rand io.Reader) {
	params.partialKeyRand = rand
}

// SetRand sets the randomness source for protocol operations.
func (params *Parameters) SetRand(rand io.Reader) {
	params.rand = rand
}

// ----- //

// Exported, used in `tss` client
//
// [FORK] Added newPartyCount/newThreshold validation panics, same rationale as
// NewParameters above. Upstream silently accepts invalid resharing parameters.
func NewReSharingParameters(ec elliptic.Curve, ctx, newCtx *PeerContext, partyID *PartyID, partyCount, threshold, newPartyCount, newThreshold int) *ReSharingParameters {
	params := NewParameters(ec, ctx, partyID, partyCount, threshold)
	if newPartyCount < 1 {
		panic(fmt.Sprintf("NewReSharingParameters: newPartyCount must be >= 1, got %d", newPartyCount))
	}
	if newThreshold < 0 || newThreshold >= newPartyCount {
		panic(fmt.Sprintf("NewReSharingParameters: newThreshold must be in [0, newPartyCount), got newThreshold=%d, newPartyCount=%d", newThreshold, newPartyCount))
	}
	return &ReSharingParameters{
		Parameters:    params,
		newParties:    newCtx,
		newPartyCount: newPartyCount,
		newThreshold:  newThreshold,
	}
}

// OldParties returns the peer context for the old committee.
func (rgParams *ReSharingParameters) OldParties() *PeerContext {
	return rgParams.Parties() // wr use the original method for old parties
}

// OldPartyCount returns the number of parties in the old committee.
func (rgParams *ReSharingParameters) OldPartyCount() int {
	return rgParams.partyCount
}

// NewParties returns the peer context for the new committee.
func (rgParams *ReSharingParameters) NewParties() *PeerContext {
	return rgParams.newParties
}

// NewPartyCount returns the number of parties in the new committee.
func (rgParams *ReSharingParameters) NewPartyCount() int {
	return rgParams.newPartyCount
}

// NewThreshold returns the new signing threshold.
func (rgParams *ReSharingParameters) NewThreshold() int {
	return rgParams.newThreshold
}

// [FORK] Append-aliasing fix: upstream does `append(old, newParties...)` directly,
// which can corrupt the OldParties backing array if old has spare capacity. We
// allocate a fresh slice before appending to avoid this aliasing corruption.
func (rgParams *ReSharingParameters) OldAndNewParties() []*PartyID {
	old := rgParams.OldParties().IDs()
	out := make([]*PartyID, len(old), len(old)+len(rgParams.NewParties().IDs()))
	copy(out, old)
	return append(out, rgParams.NewParties().IDs()...)
}

// OldAndNewPartyCount returns the total unique party count across both committees.
func (rgParams *ReSharingParameters) OldAndNewPartyCount() int {
	return rgParams.OldPartyCount() + rgParams.NewPartyCount()
}

// IsOldCommittee returns true if this party is in the old committee.
func (rgParams *ReSharingParameters) IsOldCommittee() bool {
	partyID := rgParams.partyID
	for _, Pj := range rgParams.parties.IDs() {
		if partyID.KeyInt().Cmp(Pj.KeyInt()) == 0 {
			return true
		}
	}
	return false
}

// IsNewCommittee returns true if this party is in the new committee.
func (rgParams *ReSharingParameters) IsNewCommittee() bool {
	partyID := rgParams.partyID
	for _, Pj := range rgParams.newParties.IDs() {
		if partyID.KeyInt().Cmp(Pj.KeyInt()) == 0 {
			return true
		}
	}
	return false
}
