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

func (params *Parameters) EC() elliptic.Curve {
	return params.ec
}

func (params *Parameters) Parties() *PeerContext {
	return params.parties
}

func (params *Parameters) PartyID() *PartyID {
	return params.partyID
}

func (params *Parameters) PartyCount() int {
	return params.partyCount
}

func (params *Parameters) Threshold() int {
	return params.threshold
}

func (params *Parameters) Concurrency() int {
	return params.concurrency
}

func (params *Parameters) SafePrimeGenTimeout() time.Duration {
	return params.safePrimeGenTimeout
}

// The concurrency level must be >= 1.
func (params *Parameters) SetConcurrency(concurrency int) {
	params.concurrency = concurrency
}

func (params *Parameters) SetSafePrimeGenTimeout(timeout time.Duration) {
	params.safePrimeGenTimeout = timeout
}

func (params *Parameters) NoProofMod() bool {
	return params.noProofMod
}

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

func (params *Parameters) PartialKeyRand() io.Reader {
	return params.partialKeyRand
}

func (params *Parameters) Rand() io.Reader {
	return params.rand
}

func (params *Parameters) SetPartialKeyRand(rand io.Reader) {
	params.partialKeyRand = rand
}

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

func (rgParams *ReSharingParameters) OldParties() *PeerContext {
	return rgParams.Parties() // wr use the original method for old parties
}

func (rgParams *ReSharingParameters) OldPartyCount() int {
	return rgParams.partyCount
}

func (rgParams *ReSharingParameters) NewParties() *PeerContext {
	return rgParams.newParties
}

func (rgParams *ReSharingParameters) NewPartyCount() int {
	return rgParams.newPartyCount
}

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

func (rgParams *ReSharingParameters) OldAndNewPartyCount() int {
	return rgParams.OldPartyCount() + rgParams.NewPartyCount()
}

func (rgParams *ReSharingParameters) IsOldCommittee() bool {
	partyID := rgParams.partyID
	for _, Pj := range rgParams.parties.IDs() {
		if partyID.KeyInt().Cmp(Pj.KeyInt()) == 0 {
			return true
		}
	}
	return false
}

func (rgParams *ReSharingParameters) IsNewCommittee() bool {
	partyID := rgParams.partyID
	for _, Pj := range rgParams.newParties.IDs() {
		if partyID.KeyInt().Cmp(Pj.KeyInt()) == 0 {
			return true
		}
	}
	return false
}
