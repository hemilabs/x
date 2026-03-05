# TSS-Lib Fork Changes Summary

~112 distinct changes across 72 files, all annotated with `[FORK]` comments.

## 1. SSID Domain Separation (Cross-Ceremony Replay Prevention)

- Added `SSIDNonce` field and getter/setter to `Parameters` — upstream uses hardcoded 0, enabling cross-attempt replay
- Changed nonce type from signed `int` to `uint` — prevents negative nonces producing ambiguous encodings
- All Fiat-Shamir challenges now use `SHA512_256i_TAGGED(Session, ...)` instead of untagged hashes
- SSID includes curve params, party keys, threshold, round number, and nonce
- Length-prefixed `big.Int` encoding in SSID computation — upstream's raw `Bytes()` concatenation is ambiguous (e.g., `[0x01, 0x02]` vs `[0x0102]`)
- EdDSA resharing: added SSID from scratch (upstream had none at all)
- EdDSA signing: binds the message being signed into SSID to prevent cross-session reuse
- MtA: split single Session into per-party `AliceSession`/`BobSession` for directional domain separation

## 2. ReceiverID Binding (Message Redirection Prevention)

- Added `ReceiverId` field to all P2P messages — upstream doesn't bind intended recipient, allowing share redirection attacks
- Receiver verified on receipt: each round checks `receiverId == myKey` before processing
- New `UnmarshalReceiverId()` methods on all P2P message types

## 3. ValidateBasic Hardening (Message Bounds)

- Upstream `ValidateBasic()` typically checks `m != nil` and `NonEmptyBytes` only
- Fork adds upper bounds on all fields: pubkey coordinates <= 33B, shares <= 32B, commitments <= 32B, decommitments bounded per-element
- Several upstream `ValidateBasic()` returned `true` unconditionally (no nil check) — all fixed
- Prevents memory exhaustion from oversized message fields

## 4. Key-at-Index Verification + Duplicate Message Rejection

- **Key-at-Index**: upstream only checked index bounds; fork verifies `party.Key == Ks[index]` to prevent party impersonation
- **Dedup**: reject duplicate `(round, sender)` pairs — upstream processes duplicates, enabling equivocation attacks

## 5. Nil/Zero Guards (Panic Prevention)

- `wire.go`: nil guard on `from` — upstream dereferences without checking
- `hash.go`: nil guard on `big.Int` inputs — upstream panics on nil
- `int.go`: nil guard on `ModInt` operations — upstream panics on nil bound
- `ecpoint.go`: nil guard on `p1` in `Add()`, identity-point handling in `ScalarMult` and `ScalarBaseMult`
- `paillier.go`: nil check on `ModInverse` result — upstream doesn't check
- Signing `local_party.go`: nil guard on incoming messages

## 6. Identity Point Checks

- Reject identity-point (0,0) public key shares `BigXj` — means party has zero secret share
- Reject identity-point aggregate public key (`ECDSAPub`/`EDDSAPub`) — means all shares cancel
- Reject identity-point nonce `R` in signing — would make signature verification trivial
- `ecpoint.go`: `IsIdentity()` method added; `ScalarMult`/`ScalarBaseMult` return proper identity instead of panicking on zero scalar

## 7. Secret Zeroing (Memory Hygiene)

- Clear `ui` (partial key share) after last use in keygen
- Clear signing nonces (`ki`, `gammai`, `wi`) after use — nonce leak enables private key recovery
- Unconditionally zero old `Xi` in resharing for parties leaving the committee
- Fix pointer aliasing: upstream `wi = xi` aliases the secret; fork uses explicit copy

## 8. Parameter / Modulus Validation

- Reject invalid `partyCount`/`threshold` combinations (upstream silently accepts)
- Post-sort validation: reject duplicate or empty party keys
- Fix sort comparator: upstream `<= 0` treats equal keys as less-than; fork uses `< 0`
- All proof verifiers reject moduli < 2048 bits (NTilde, Paillier N, N0, NCap)
- Resharing: comprehensive parameter validation battery (NTilde, H1/H2, Paillier) on received data

## 9. ZK Proof Hardening

- **MtA Alice**: s2 upper bound `2*q^3*NTilde` — prevents DoS via oversized exponents
- **MtA Bob**: s2 and t2 upper bounds (same bound) — same motivation
- **MtA both**: reject degenerate Pedersen params (h1=1 or h2=1), verify ciphertext coprimality with N^2
- **Schnorr**: reject proof scalars outside `[0, q)` — prevents malleability (`T + k*q` verifies identically)
- **Schnorr**: check `Add()` error instead of discarding it
- **DLN**: session parameter for SSID domain separation; reject undersized moduli
- **ModProof**: reject undersized N; fail-fast if no quadratic residue found during generation
- **FacProof**: sign-magnitude encoding for V (can be negative) — upstream silently drops sign, causing ~50% honest proof failure; reject undersized N0 and NCap

## 10. Wire Format / Serialization

- Deterministic protobuf marshaling (`proto.MarshalOptions{Deterministic: true}`) — upstream uses non-deterministic default
- Propagate `anypb.New` errors — upstream silently discards
- Length-prefixed `big.Int` encoding to prevent ambiguous concatenation
- O(n) zero-padding instead of upstream's O(n^2) prepend loop
- EC point deserialization: bound coordinate length to prevent crafted oversized inputs

## 11. VSS Hardening

- Return polynomial coefficients as third return value (for SNARK witness extraction)
- Reject shares that are zero or outside `[1, q-1]`
- Reject share ID that is nil or zero mod q — evaluation at x=0 leaks the secret
- Detect duplicate share IDs (reduced mod q) — prevents silently wrong interpolation
- Nil-check on `ModInverse` during Lagrange interpolation

## 12. Lagrange Interpolation (PrepareForSigning)

- Explicit pointer copy instead of aliasing (`wi = new(big.Int).Set(xi)` instead of `wi = xi`)
- Nil-check on `ModInverse` — returns nil if two party keys collide (modular inverse doesn't exist)
- `wi == 0` check — zero Lagrange coefficient means party contributes nothing to the signature
- Same nil-check pattern in `BigXj` Lagrange interpolation loop

## 13. SNARK Integration

- `NoProofDLN()`, `NoProofMod()`, `NoProofFac()` flags — skip classical ZK proofs when replaced by SNARK coverage
- `GetPoly()` method to extract VSS polynomial coefficients for SNARK witness
- `GetNewVs()` method to extract Feldman VSS commitments for SNARK witness
- Store VSS polynomial and commitments in temp data during keygen/resharing Round 1

## 14. Save Data Validation

- **`ValidateWithProof()`** (ECDSA pre-params): verifies P!=Q, NTilde=(2P+1)(2Q+1), H2=H1^Alpha mod NTilde — catches corrupted/tampered pre-params before they silently produce invalid proofs
- **`ValidateSaveData()`** (ECDSA + EdDSA): nil checks, array consistency, on-curve verification, ShareID lookup, Feldman VSS invariant (Xi*G == BigXj[ownIndex]) — catches storage corruption before signing

## 15. Signing Protocol Hardening

- Message range check: verify `m >= 0` (upstream only checks `m < N`)
- Theta zero-check in round 4 — zero theta causes division-by-zero
- Zero-r check in round 5 — ECDSA requires `r = R.x mod N != 0`
- Range check on each party's `s_j` share in finalize — reject values outside `[0, q)`
- Zero-S rejection — final signature `S = 0` is invalid
- Ceiling division for byte-length: `(BitSize + 7) / 8` instead of `BitSize / 8` (correct for non-8-aligned curves like P-521)
- EdDSA: reject values exceeding 32 bytes (upstream silently truncates)
- EdDSA: R identity check in round 3

## 16. Commitment Scheme

- Reject decommitments with `len(D) < 2` — a single-element decommitment has no payload after the blinding factor

## 17. Miscellaneous

- Append-aliasing fix in `ReSharingParameters`: upstream's `append(old, new...)` can mutate the old slice's backing array
- `GenerateNTildei`: reject equal primes (P==Q makes NTilde a perfect square, breaking DLN)
- `EightInvEight()` call ordering fix in EdDSA signing round 3
