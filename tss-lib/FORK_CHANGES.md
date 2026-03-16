# TSS-Lib v3 Fork — Security Audit Changes

~112 security fixes from Max's audit of the upstream binance/tss-lib,
remapped to the v3 channel-free round function architecture.

All fixes are annotated with `[FORK]` comments in source.

## v3 Architecture Note

v3 deletes the channel-based `Party`/`Round`/`BaseUpdate` API.  Each
ceremony round is a pure function: takes state + inbound messages,
returns outbound messages.  No channels, no goroutines in the protocol
layer.  Some v2 fixes (marked below) are eliminated by the v3
architecture rather than ported — the attack surface they addressed
no longer exists.

---

## 1. SSID Domain Separation (Cross-Ceremony Replay Prevention)

**v3 location:** `tss/params.go` (SetSSIDNonce, SetCeremonyID),
`ecdsa/keygen/round_fn.go` getSSID, `ecdsa/signing/round_fn.go`
getSigningSSID, `ecdsa/resharing/round_fn.go` getReshareSSID

- SSIDNonce field and getter/setter on Parameters
- Nonce type `uint` (not signed int) — prevents ambiguous encoding
- All Fiat-Shamir challenges use SHA512_256i_TAGGED(Session, ...)
- SSID includes: protocol tag, curve params (P,N,B,Gx,Gy), party keys,
  partyCount, threshold, round number, nonce, ceremonyID
- Length-prefixed big.Int encoding (not raw Bytes() concatenation)
- MtA: per-party AliceSession/BobSession directional separation

**EdDSA (pending):** SSID pattern will be replicated in eddsa/round_fn.go.

## 2. ReceiverID Binding (Message Redirection Prevention)

**v3 location:** `ecdsa/*/messages.go` (proto fields),
`ecdsa/signing/round_fn.go` SignRound2/SignRound3 (verification)

- ReceiverId field on all P2P messages
- Receiver verified on receipt: `receiverId == myKey`
- UnmarshalReceiverId() methods on all P2P message types

## 3. ValidateBasic Hardening (Message Bounds)

**v3 location:** `ecdsa/*/messages.go` (unchanged from v2)

- Upper bounds on all fields: pubkey coords ≤ 33B, shares ≤ 32B,
  commitments ≤ 32B, decommitments bounded per-element
- Fixed unconditional-true ValidateBasic() methods
- Prevents memory exhaustion from oversized fields

## 4. Key-at-Index Verification + Duplicate Message Rejection

**v3 status:** Eliminated by architecture.

v2 implemented Key-at-Index in `local_party.go` StoreMessage and
duplicate (round, sender) rejection in the same method.  In v3, the
caller (continuum's HandleMessage) validates sender identity against
the ceremony PID set before delivering to the round function.  Message
dedup is handled by the caller's indexed slot array (msgBuf) — a
duplicate sender overwrites its own slot, which is idempotent.

## 5. Nil/Zero Guards (Panic Prevention)

**v3 location:** `tss/wire.go`, `common/hash.go`, `common/int.go`,
`crypto/ecpoint.go`, `crypto/paillier/paillier.go` (all unchanged)

- wire.go: nil guard on `from`
- hash.go: nil guard on big.Int inputs
- int.go: nil guard on ModInt operations, IsInInterval nil check (fix 112)
- ecpoint.go: nil guard on p1 in Add(), identity handling in ScalarMult
- paillier.go: nil check on ModInverse result

**v2 only (eliminated):** signing local_party.go nil guard on incoming
messages — v3 has no local_party; caller validates before delivery.

## 6. Identity Point Checks

**v3 location:** `crypto/ecpoint.go` (IsIdentity method),
`ecdsa/keygen/round_fn.go` Round3/Round4 (Xi zero, ECDSAPub identity,
BigXj identity — fixes 95-97), `ecdsa/signing/round_fn.go` SignRound5
(R identity), SignRound7 (Vj/Aj — fix 108), SignRound9 (Uj/Tj — fix 109),
`ecdsa/resharing/round_fn.go` ReshareRound4/5 (newXi zero, newBigXj
identity — fix 104)

- Reject identity-point public key shares, aggregate pubkeys, nonce R
- Reject zero Xi (private key share)

**EdDSA (pending):** Same checks for EDDSAPub, BigXj (fix 98), newXi/
newBigXj (fix 105).

## 7. Secret Zeroing (Memory Hygiene)

**v3 location:** `ecdsa/keygen/round_fn.go` Round1 (clear ui after VSS
— fix 102), `ecdsa/signing/round_fn.go` SignRound5 (clear k, gamma, w,
sigma — fix 106), `ecdsa/resharing/round_fn.go` ReshareRound5
(unconditionally zero old Xi)

- Explicit pointer copy instead of aliasing (wi = new(big.Int).Set(xi))

**EdDSA (pending):** Clear ui in keygen round 2, clear ri/wi in signing
round 3 (fix 110).

## 8. Parameter / Modulus Validation

**v3 location:** `tss/params.go` (threshold/partyCount validation,
sort comparator fix, duplicate key rejection),
`ecdsa/keygen/round_fn.go` Round2 (comprehensive parameter validation
battery — Paillier N, NTilde, H1/H2, DLN proofs),
`ecdsa/resharing/round_fn.go` ReshareRound4 (same battery for new
committee)

- Reject ≤2048-bit moduli, even moduli, prime moduli, perfect squares
- Reject duplicate/equal H1/H2/NTilde/PaillierN
- Reject non-coprime H1/H2 with NTilde

## 9. ZK Proof Hardening

**v3 location:** `crypto/mta/*`, `crypto/schnorr/*`,
`crypto/dlnproof/*`, `crypto/facproof/*`, `crypto/modproof/*`
(all unchanged from v2)

- MtA Alice/Bob: s2/t2 upper bounds, degenerate Pedersen rejection,
  ciphertext coprimality check, nil input validation (fixes 99-100),
  Paillier N minimum bitlen (fix 101)
- Schnorr: reject scalars outside [0,q), check Add() error (fix 103)
- DLN: session parameter for SSID, reject undersized moduli
- ModProof: reject undersized N, fail-fast on no quadratic residue
- FacProof: sign-magnitude V encoding (upstream drops sign → ~50%
  honest failure), reject undersized N0/NCap

## 10. Wire Format / Serialization

**v3 location:** `tss/wire.go`, `ecdsa/*/messages.go`
(unchanged from v2)

- Deterministic protobuf marshaling
- Propagate anypb.New errors
- Length-prefixed big.Int encoding
- O(n) zero-padding (not O(n²) prepend)
- EC point deserialization: bound coordinate length

## 11. VSS Hardening

**v3 location:** `crypto/vss/feldman_vss.go` (unchanged from v2),
`ecdsa/keygen/round_fn.go` Round1 (polynomial coefficients in
RoundOutput.Poly for SNARK witness)

- Reject zero/out-of-range shares, nil/zero share IDs
- Detect duplicate share IDs (reduced mod q)
- Nil-check ModInverse during Lagrange interpolation

## 12. Lagrange Interpolation (PrepareForSigning)

**v3 location:** `ecdsa/signing/prepare.go` (unchanged from v2)

- Explicit pointer copy (wi = new(big.Int).Set(xi))
- Nil-check ModInverse for colliding party keys
- wi == 0 check (zero Lagrange coefficient)

## 13. SNARK Integration Seams

**v3 location:** `tss/params.go` (NoProofDLN/NoProofMod/NoProofFac
flags), `ecdsa/keygen/round_state.go` (RoundOutput.Poly),
`ecdsa/resharing/round_fn.go` (ReshareRoundOutput.Poly, NewVs),
`ecdsa/keygen/round_fn.go` Round2/Round3 (conditional proof skip)

- Skip classical ZK proofs when SNARK covers the same property
- Expose VSS polynomial/commitments for SNARK witness extraction

## 14. Save Data Validation

**v3 location:** `ecdsa/keygen/save_data.go` (unchanged from v2)

- ValidateWithProof(): P!=Q, NTilde consistency, H2=H1^Alpha
- ValidateSaveData(): nil checks, array consistency, on-curve, Feldman invariant

## 15. Signing Protocol Hardening

**v3 location:** `ecdsa/signing/round_fn.go`

- Message range check: m >= 0
- Theta zero-check in SignRound4
- Zero-r check in SignRound5 (R.x mod N != 0)
- Range check on each s_j share in SignFinalize
- Zero-S rejection in SignFinalize
- Ceiling division for byte-length: (BitSize+7)/8
- Low-S normalization in SignFinalize

## 16. Commitment Scheme

**v3 location:** `crypto/commitments/commitment.go` (unchanged from v2)

- Reject decommitments with len(D) < 2

## 17. Miscellaneous

**v3 location:** `tss/params.go` (OldAndNewParties append-aliasing —
fix 107), `crypto/utils.go` (GenerateNTildei distinct primes — fix 111)

---

## Deferred Items (carried from v2 audit)

1. MtA inverted lower-bound checks (proofs.go, range_proof.go) — don't
   reject honest proofs in practice
2. ProofIters=13 (paillier.go) — matches GG18 spec
3. BuildLocalSaveDataSubset panic (save_data.go) — changing signature
   breaks callers
4. NSquare() caching (paillier.go) — performance only
5. MustGetRandomInt off-by-one (random.go) — negligible for 256-bit
6. Concurrent io.Reader in safe prime gen — safe with crypto/rand.Reader
7. CKD only works for secp256k1 — non-blocking
8. Commitment scheme no domain separation — prevented by 256-bit nonce
9. Threshold=0 accepted — VSS rejects downstream
10. SHA512_256i sign-blindness — all callers pass non-negative
