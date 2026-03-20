# Security Fixes — v2 Audit → v3 Implementation

Security fixes from the audit of upstream
[binance/tss-lib](https://github.com/bnb-chain/tss-lib), implemented
in the v3 channel-free round function architecture.

76 fixes in production code across 24 files, all annotated with
`[FORK]` comments in source.  Grep for `[FORK]` to find them all.

The v2 base had 260 annotations across 88 files — the difference is
that v3 deleted the channel-based `Party`/`Round`/`BaseUpdate` API
entirely, eliminating the attack surface those fixes protected.

---

## Cross-Ceremony Replay Prevention

**Problem:** v2 used hardcoded nonce=0 in all Fiat-Shamir challenges.
Two concurrent ceremonies with the same parties could replay proofs
across each other.

**Fix:** Full SSID domain separation — every proof challenge includes
a protocol tag, curve params, party keys, party count, threshold,
round number, caller-supplied nonce, and ceremony ID.  All encoded
with length-prefixed big.Int (not ambiguous raw concatenation).  MtA
uses per-party directional session IDs.

**Where:** `tss/params.go`, all `getSSID()`/`getSigningSSID()`/
`getReshareSSID()` in `ecdsa/*/round_fn.go` and `eddsa/*/round_fn.go`

---

## Message Redirection Prevention

**Problem:** v2 P2P messages had no receiver binding.  A malicious
relay could swap envelopes between parties, causing them to use each
other's shares silently.

**Fix:** `ReceiverID` field on every P2P message.  Receiver verifies
`msg.ReceiverID == myKey` before processing.

**Where:** `ecdsa/*/messages.go`, `eddsa/keygen/messages.go`,
`eddsa/resharing/messages.go` — all P2P message types

---

## Message Bounds Hardening

**Problem:** v2 `ValidateBasic()` methods checked nil but not size.
An attacker could send 100MB coordinates and exhaust memory before
any crypto runs.

**Fix:** Upper bounds on all fields: pubkey coords ≤ 33B, scalars ≤
32B, commitments ≤ 32B, decommitments bounded per-element, Paillier
moduli ≤ 512B, proof arrays at exact expected sizes.

**Where:** all `ValidateBasic()` in `ecdsa/*/messages.go` and
`eddsa/*/messages.go`

---

## Identity Point / Zero Share Rejection

**Problem:** v2 did not check for degenerate values.  A zero Xi means
the party's contribution is annihilated during signing.  An identity-
point public key makes verification equations trivially satisfiable.

**Fix:** Every round that computes or receives key material checks:
Xi != 0, ECDSAPub/EDDSAPub != identity, BigXj != identity, nonce R
!= identity, accumulated S != 0.

**Where:** `ecdsa/keygen/round_fn.go` (Round3/Round4),
`ecdsa/signing/round_fn.go` (SignRound5/7/9/Finalize),
`ecdsa/resharing/round_fn.go` (ReshareRound4/5),
`eddsa/keygen/round_fn.go` (Round3), `eddsa/signing/round_fn.go`
(SignRound3/Finalize), `eddsa/resharing/round_fn.go` (ReshareRound4)

---

## Secret Zeroing

**Problem:** v2 left secret key material in memory after use.  A
memory disclosure bug would leak signing nonces (enabling share
recovery) or old Xi after resharing (enabling threshold reduction).

**Fix:** Explicit zero after last use: ui after VSS, k/gamma/w/sigma
after signing, ri/wi after EdDSA signing, old Xi after resharing
(including dual-committee parties — v2 missed those).

**Where:** `ecdsa/keygen/round_fn.go` (Round1), `ecdsa/signing/
round_fn.go` (SignRound5), `ecdsa/resharing/round_fn.go`
(ReshareRound5), `eddsa/keygen/round_fn.go` (Round2),
`eddsa/signing/round_fn.go` (SignRound3), `eddsa/resharing/round_fn.go`
(ReshareRound5)

---

## Modulus Validation Battery

**Problem:** v2 accepted any Paillier N and NTilde without structural
checks.  An attacker could submit a weak modulus (prime, small, even,
perfect square) and break the security assumptions of the proofs.

**Fix:** Comprehensive battery in keygen Round 2: reject ≤2048-bit,
even, prime, perfect-square moduli.  Reject duplicate/equal H1/H2/
NTilde/PaillierN.  Reject non-coprime H1/H2 with NTilde.

**Where:** `ecdsa/keygen/round_fn.go` (Round2)

---

## ZK Proof Hardening

**Problem:** Multiple issues across MtA, Schnorr, DLN, Fac, Mod
proofs — missing range checks, missing nil validation, degenerate
Pedersen parameters, ciphertext coprimality bypass, sign-magnitude
encoding bug (~50% honest failure rate for FacProof).

**Fix:** Per-proof fixes:
- MtA: s2/t2 upper bounds, Pedersen rejection, coprimality check,
  nil validation, Paillier N minimum bitlen
- Schnorr: reject scalars outside [0,q), handle Add() error
- DLN: SSID session parameter, reject undersized moduli
- ModProof: reject undersized N, fail-fast on no quadratic residue
- FacProof: sign-magnitude V encoding (fixes ~50% honest failure)

**Where:** `crypto/mta/*`, `crypto/schnorr/*`, `crypto/dlnproof/*`,
`crypto/facproof/*`, `crypto/modproof/*`

---

## VSS Hardening

**Problem:** v2 accepted zero/out-of-range shares and nil share IDs.
Duplicate share IDs (reduced mod q) could cause silent collisions.

**Fix:** Reject zero/out-of-range shares, nil/zero share IDs.  Detect
duplicate IDs.  Nil-check ModInverse during Lagrange interpolation.
wi == 0 post-check after interpolation.

**Where:** `crypto/vss/feldman_vss.go`, `ecdsa/signing/prepare.go`,
`eddsa/signing/prepare.go`

---

## Signing Protocol Hardening

**Problem:** Multiple edge cases in ECDSA signing: negative message,
zero theta, zero r (R.x mod N = 0), out-of-range partial sigs,
non-canonical S values.

**Fix:** m >= 0 check, theta zero-check, r zero-check, per-party s_j
range check [0,N), S != 0, low-S normalization, ceiling division for
byte-length.

**Where:** `ecdsa/signing/round_fn.go`

---

## EdDSA-Specific Fixes

**Problem:** EdDSA signing nonce R as identity leaks the full private
key (s = H(R,A,M)*a with no blinding).  EdDSA cofactor-8 curve
allows small-order torsion points in commitments.

**Fix:** R identity check in SignRound3.  Cofactor clearing via
`EightInvEight()` on all unflattened VSS polynomial points in keygen
and resharing.  Edwards curve singleton (fix for pointer identity
comparison in `ECPoint.Add()`).

**Where:** `eddsa/signing/round_fn.go` (SignRound3),
`eddsa/keygen/round_fn.go` (Round3), `eddsa/resharing/round_fn.go`
(ReshareRound4), `tss/curve.go` (Edwards singleton)

---

## Eliminated by v3 Architecture

These v2 fixes are no longer needed because v3 removes the channel-
based state machine entirely:

- **Key-at-Index verification** — v3 caller validates sender identity
  before delivering to round functions
- **Duplicate message rejection** — v3 caller's indexed slot array
  makes duplicates idempotent
- **local_party.go nil guards** — no local_party in v3
- **Channel close races** — no channels in v3 protocol layer

---

## Deferred Items

Low-impact items carried from the v2 audit that don't affect
correctness or security in practice:

1. MtA inverted lower-bound checks — don't reject honest proofs
2. ProofIters=13 — matches GG18 spec
3. BuildLocalSaveDataSubset panic — changing signature breaks callers
4. NSquare() caching — performance optimization only
5. MustGetRandomInt off-by-one — negligible for 256-bit
6. Concurrent io.Reader — safe with crypto/rand.Reader
7. CKD only for secp256k1 — by design
8. Commitment no domain separation — 256-bit nonce prevents collisions
9. Threshold=0 accepted — VSS rejects downstream
10. SHA512_256i sign-blindness — all callers pass non-negative
