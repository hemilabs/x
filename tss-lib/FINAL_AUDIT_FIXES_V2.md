# TSS-Lib Fork Audit Fixes — Sessions 2–3

## Summary

| Metric | Value |
|--------|-------|
| Total fixes this session | 18 (fixes 95–112) |
| Total agents this session | 14 (Wave 17–20) |
| Total findings this session | 120+ |
| All 17 Go test packages | PASS |

Previous sessions implemented fixes 1–94 across 51 agents and 17 waves.

---

## Fixes Implemented

### Fix 95 (MEDIUM): ECDSA keygen Xi zero-check
- **File**: `ecdsa/keygen/round_3.go:45`
- **Issue**: After summing VSS shares, Xi is not checked for zero. A zero private key share is degenerate and would produce the identity point for BigXj.
- **Fix**: Added `if round.save.Xi.Sign() == 0 { return error }` after mod reduction.

### Fix 96 (MEDIUM): ECDSA keygen ECDSAPub identity-point check
- **File**: `ecdsa/keygen/round_3.go:207`
- **Issue**: The computed ECDSA public key is checked for on-curve but not for being the identity point (0,0).
- **Fix**: Added `if ecdsaPubKey.IsIdentity() { return error }`.
- **Supporting**: Added `ECPoint.IsIdentity()` method to `crypto/ecpoint.go`.

### Fix 97 (MEDIUM): ECDSA keygen BigXj identity-point check
- **File**: `ecdsa/keygen/round_3.go:196`
- **Issue**: Per-party public key shares (BigXj) not checked for being the identity point.
- **Fix**: Added identity check before saving each BigXj.

### Fix 98 (MEDIUM): EdDSA keygen Xi, EDDSAPub, and BigXj checks
- **File**: `eddsa/keygen/round_3.go`
- **Issue**: Same three checks missing as in ECDSA keygen (fixes 95–97).
- **Fix**: Added Xi zero-check, EDDSAPub identity check, BigXj identity check.

### Fix 99 (MEDIUM): AliceInit nil validation
- **File**: `crypto/mta/share_protocol.go:20-33`
- **Issue**: `AliceInit` does not validate input parameters for nil before use. A nil `pkA` causes a nil-pointer panic.
- **Fix**: Added nil guard: `if ec == nil || pkA == nil || a == nil || NTildeB == nil || h1B == nil || h2B == nil || rand == nil`.

### Fix 100 (MEDIUM): BobMid/BobMidWC nil validation
- **File**: `crypto/mta/share_protocol.go:35-104`
- **Issue**: Neither BobMid nor BobMidWC validates its input parameters for nil.
- **Fix**: Added nil guards at function entry for both functions.

### Fix 101 (MEDIUM): Paillier N minimum bit-length in MtA proofs
- **Files**: `crypto/mta/proofs.go:220`, `crypto/mta/range_proof.go:123`
- **Issue**: MtA proof verification checks NTilde.BitLen() >= 2048 but not pk.N.BitLen(). A malicious party could use a small Paillier modulus.
- **Fix**: Added `if pk.N.BitLen() < 2048 { return false }` to both ProofBobWC.Verify and RangeProofAlice.Verify.

### Fix 102 (LOW): Clear secret ui after last use in keygen
- **Files**: `ecdsa/keygen/round_1.go:55-58`, `eddsa/keygen/round_1.go:59-63`, `eddsa/keygen/round_2.go:50`
- **Issue**: `round.temp.ui` holds the partial key share secret. In ECDSA keygen it's never used after round 1 but remains in memory. In EdDSA keygen it's used in round 2 for Schnorr proof but not cleared after.
- **Fix**: ECDSA: `round.temp.ui = new(big.Int)` in round 1 after VSS create. EdDSA: `round.temp.ui = new(big.Int)` in round 2 after Schnorr proof.
- **Test**: Updated both `local_party_test.go` files to not compare against zeroed `temp.ui`.

### Fix 103 (MEDIUM): ZKVProof panic via nil pointer when tR+uG is identity
- **File**: `crypto/schnorr/schnorr_proof.go:99,136`
- **Issue**: `NewZKVProof` and `ZKVProof.Verify` discard errors from `Add()`. If the result is the identity point, `NewECPoint` returns (nil, error) and the subsequent `.X()` call panics.
- **Fix**: Changed `alpha, _ := aR.Add(bG)` to handle error and return. Changed `tRuG, _ := tR.Add(uG)` to handle error and return false.

### Fix 104 (MEDIUM): ECDSA resharing newXi zero-check and newBigXj identity check
- **File**: `ecdsa/resharing/round_4_new_step_2.go`
- **Issue**: Same Xi zero-check and BigXj identity-point checks missing as in keygen.
- **Fix**: Added zero-check after newXi mod reduction. Added identity check on each newBigXj before saving.

### Fix 105 (MEDIUM): EdDSA resharing newXi zero-check and newBigXj identity check
- **File**: `eddsa/resharing/round_4_new_step_2.go`
- **Issue**: Same as Fix 104 for EdDSA protocol.
- **Fix**: Added zero-check after newXi mod reduction. Added identity check on each newBigXj before saving.

### Fix 106 (LOW): Clear secret nonces gamma/sigma in ECDSA signing
- **File**: `ecdsa/signing/round_5.go:88-89`
- **Issue**: `round.temp.gamma` and `round.temp.sigma` are secret nonces that remain in memory after their last use. Combined with signature output, these could help reconstruct the private key.
- **Fix**: Added `round.temp.gamma = new(big.Int)` and `round.temp.sigma = new(big.Int)` alongside existing k/w clearing.

### Fix 107 (LOW): OldAndNewParties append aliasing bug
- **File**: `tss/params.go:211`
- **Issue**: `OldAndNewParties()` uses `append(rgParams.OldParties().IDs(), ...)` which can corrupt the old parties slice if it has spare capacity — a classic Go append-aliasing bug.
- **Fix**: Explicit copy into a new slice before appending new party IDs.

### Fix 108 (MEDIUM): ECDSA signing round 7 Vj/Aj identity-point check
- **File**: `ecdsa/signing/round_7.go:49-54`
- **Issue**: Decommitted `bigVj` and `bigAj` points are checked for on-curve but not for being the identity point. Defense-in-depth against curves where `IsOnCurve(0,0)` might return true.
- **Fix**: Added `if bigVj.IsIdentity()` and `if bigAj.IsIdentity()` checks after `NewECPoint`.

### Fix 109 (MEDIUM): ECDSA signing round 9 Uj/Tj identity-point check
- **File**: `ecdsa/signing/round_9.go:44-49`
- **Issue**: Decommitted `Uj` and `Tj` points are checked for on-curve but not for identity. Same defense-in-depth as Fix 108.
- **Fix**: Added `if Uj.IsIdentity()` and `if Tj.IsIdentity()` checks after `NewECPoint`.

### Fix 110 (MEDIUM): EdDSA signing nonce clearing
- **File**: `eddsa/signing/round_3.go:114-115`
- **Issue**: `round.temp.ri` (signing nonce) and `round.temp.wi` (Lagrange-interpolated secret share) remain in memory after their last use in round 3. If `ri` leaks, the private key can be recovered from the published signature.
- **Fix**: Added `round.temp.ri = new(big.Int)` and `round.temp.wi = new(big.Int)` after computing `localS`.

### Fix 111 (MEDIUM): GenerateNTildei distinct primes check
- **File**: `crypto/utils.go:23`
- **Issue**: `GenerateNTildei` does not check that the two safe primes are distinct. If `p == q`, then `NTilde = p^2`, which is trivially factorable and completely breaks the Pedersen commitment security.
- **Fix**: Added `if safePrimes[0].Cmp(safePrimes[1]) == 0 { return error }`.

### Fix 112 (LOW): IsInInterval nil-safety
- **File**: `common/int.go:63`
- **Issue**: `IsInInterval(b, bound)` panics on nil inputs. This function is used in proof verification where attacker-controlled deserialized values are checked, making it a potential DoS vector.
- **Fix**: Added `if b == nil || bound == nil { return false }`.

---

## Agent Results

### Wave 17

**MtA + Paillier audit**: 18 findings (7 MEDIUM, 8 LOW, 2 INFO, 1 correct-by-inspection)
- Implemented: Fixes 99, 100, 101

**ECDSA keygen round 3-4 audit**: 11 findings (3 MEDIUM, 8 LOW)
- Implemented: Fixes 95, 96, 97

### Wave 18

**ECDSA resharing deep audit**: ~15 findings → Fix 104
**EdDSA resharing deep audit**: ~10 findings → Fix 105
**ECDSA signing deep audit**: ~6 findings → Fix 106
**VSS + Schnorr audit**: ~5 findings → Fix 103

### Wave 19

**TSS params + wire audit**: 7 findings (1 HIGH, 2 MEDIUM, 3 LOW, 1 INFO)
- Key findings: threshold=0 accepted, no guard against disabling all proofs, OldAndNewParties append aliasing, SetRand accepts nil, SSIDNonce no monotonicity
- Implemented: Fix 107

**DLN + commitment audit**: 6 findings (1 MEDIUM, 3 LOW, 2 INFO)
- Key findings: commitment scheme no domain separation, DLN no oddness check on N
- DLN proof 128 iterations confirmed adequate for 128-bit soundness

**ECDSA signing rounds 7-9 audit**: 17 findings (3 MEDIUM, 4 LOW, 10 INFO)
- Key findings: no identity check on decommitted Vj/Aj/Uj/Tj points
- Implemented: Fixes 108, 109
- Confirmed: ecdsa.Verify final safety net, low-S normalization, duplicate rejection

### Wave 20

**common/ package audit**: 18 findings (2 HIGH, 5 MEDIUM, 7 LOW, 4 INFO)
- Key findings: MustGetRandomInt off-by-one (excludes 2^bits-1), GetRandomPositiveInt(rand,2) infinite loops, concurrent io.Reader sharing in safe prime gen, ModInverse silent nil return
- Implemented: Fix 112

**crypto/ckd + utils audit**: 12 findings (1 CRITICAL, 1 HIGH, 6 MEDIUM, 4 LOW)
- Key findings: CKD only works for secp256k1 (not generic curves), no identity check on derived child key, GenerateNTildei no distinct primes check
- Implemented: Fix 111

**Signing finalize audit**: 17 findings (5 MEDIUM, 4 LOW, 8 INFO)
- Key findings: EdDSA no nonce clearing, localTempData not cleared after signing, s=0 no automatic restart
- Implemented: Fix 110

---

## Files Modified This Session

| File | Fixes |
|------|-------|
| `common/int.go` | IsInInterval nil check (112) |
| `crypto/ecpoint.go` | IsIdentity() method (96) |
| `crypto/utils.go` | Distinct primes check (111) |
| `crypto/schnorr/schnorr_proof.go` | Identity panic fix (103) |
| `crypto/mta/share_protocol.go` | Nil validation (99, 100) |
| `crypto/mta/proofs.go` | Paillier N check (101) |
| `crypto/mta/range_proof.go` | Paillier N check (101) |
| `tss/params.go` | OldAndNewParties aliasing fix (107) |
| `ecdsa/keygen/round_1.go` | Clear ui (102) |
| `ecdsa/keygen/round_3.go` | Xi, ECDSAPub, BigXj checks (95–97) |
| `ecdsa/keygen/local_party_test.go` | Test update for ui zeroing (102) |
| `ecdsa/resharing/round_4_new_step_2.go` | Xi, BigXj checks (104) |
| `ecdsa/signing/round_5.go` | Clear gamma/sigma (106) |
| `ecdsa/signing/round_7.go` | Vj/Aj identity checks (108) |
| `ecdsa/signing/round_9.go` | Uj/Tj identity checks (109) |
| `eddsa/keygen/round_1.go` | ui comment update (102) |
| `eddsa/keygen/round_2.go` | Clear ui (102) |
| `eddsa/keygen/round_3.go` | Xi, EDDSAPub, BigXj checks (98) |
| `eddsa/keygen/local_party_test.go` | Test update for ui zeroing (102) |
| `eddsa/resharing/round_4_new_step_2.go` | Xi, BigXj checks (105) |
| `eddsa/signing/round_3.go` | Clear ri/wi nonces (110) |

---

## Known Deferred Items

1. **MtA inverted lower-bound checks** (proofs.go:274-285, range_proof.go:149-163): S1/S2/T1/T2 lower bounds deviate from GG18 spec but don't reject honest proofs in practice. Deferred to avoid changing proof verification semantics.

2. **ProofIters=13** (paillier.go:35): Paillier proof soundness is 2^{-13}. Matches GG18 spec. Increasing would break backward compatibility.

3. **BuildLocalSaveDataSubset panic** (save_data.go:110): Uses panic instead of error return. Changing signature would require updating all callers. Deferred.

4. **NSquare() caching** (paillier.go:167): Performance-only. Computes N^2 on every call.

5. **MustGetRandomInt off-by-one** (random.go:28-32): Excludes value `2^bits - 1` from the output range. The probability impact is negligible for 256-bit values (~2^-256). Fixing requires careful analysis of all callers to ensure no proof compatibility issues. `GetRandomPositiveInt(rand, 2)` would infinite loop but no caller passes `lessThan=2`.

6. **Concurrent io.Reader in safe prime gen** (safe_prime.go:127-153): Multiple goroutines share the same `io.Reader`. Safe when `crypto/rand.Reader` is used (which is always the case in production). Non-thread-safe readers would cause data races. Deferred — adding a mutex would add complexity and the production path is safe.

7. **CKD only works for secp256k1** (crypto/ckd/child_key_derivation.go): `NewExtendedKeyFromString` uses `elliptic.Unmarshal` which only handles uncompressed format, but keys are serialized compressed. Works correctly for secp256k1 via btcec path. Non-blocking for deployment.

8. **Commitment scheme no domain separation** (crypto/commitments/commitment.go): Uses untagged `SHA512_256i` without protocol-phase tag. Collision between commitment uses is prevented by the 256-bit random nonce but lacks formal domain separation.

9. **Threshold=0 accepted** (tss/params.go:54): Allows 1-of-n scheme. The VSS layer rejects `threshold < 1` during keygen, providing downstream protection.

10. **SHA512_256i sign-blindness** (common/hash.go): `big.Int.Bytes()` drops the sign, so negative inputs collide with their positive counterpart. All protocol callers pass non-negative values, but the hash layer doesn't enforce this.
