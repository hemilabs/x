# Hemi TSS Library (v3)

[![MIT licensed](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Threshold signature scheme library for ECDSA (secp256k1) and EdDSA
(Ed25519), forked from [binance/tss-lib](https://github.com/bnb-chain/tss-lib)
with security audit fixes and a channel-free round function API.

## Attribution

This library is built on the excellent work of the
[Binance tss-lib](https://github.com/bnb-chain/tss-lib) team, which
provided a solid, well-structured implementation of GG18/GG20
threshold ECDSA and EdDSA.  The original Feldman VSS, Paillier
encryption, Schnorr proofs, and MtA protocol implementations form the
cryptographic foundation of this library.  We are grateful for their
contribution to the open-source TSS ecosystem.

Our fork (v3) adds security audit fixes, removes protobuf in favor of
plain Go structs, replaces the channel-based state machine with pure
round functions, and adds overlapping-committee resharing support.
See [SECURITY_FIXES.md](SECURITY_FIXES.md) for details on the audit
fixes.

## v3 API

v3 replaces the channel-based `Party`/`Round`/`BaseUpdate` state
machine with pure round functions.  Each round takes explicit state +
inbound messages and returns outbound messages.  The caller owns the
event loop.

**No protobuf.**  All message types are plain Go structs with typed
fields.  Serialization is the caller's responsibility — the library
does not prescribe a wire format.

## ECDSA — Keygen / Sign / Reshare

Complete runnable example: [`ecdsa/example_test.go`](ecdsa/example_test.go)

```
go test -tags tssexamples -v -run TestECDSAKeygenSignReshare ./ecdsa/ -timeout 15m
```

### Keygen (4 rounds)

```go
import "github.com/hemilabs/x/tss-lib/v3/ecdsa/keygen"

// Generate Paillier pre-params (CPU-intensive, do out-of-band).
preParams, _ := keygen.GeneratePreParams(5 * time.Minute)

params := tss.NewParameters(tss.S256(), peerCtx, pIDs[i], n, threshold)
state, r1out, _ := keygen.Round1(ctx, params, *preParams)
r2out, _ := keygen.Round2(ctx, state, r1Msgs)
r3out, _ := keygen.Round3(ctx, state, r2p2p, r2bcast)
r4out, _ := keygen.Round4(ctx, state, r3Msgs)
// r4out.Save = LocalPartySaveData (key share)
```

### Signing (9 rounds + finalize)

```go
import "github.com/hemilabs/x/tss-lib/v3/ecdsa/signing"

m := new(big.Int).SetBytes(msgHash[:])
sigState, r1out, _ := signing.SignRound1(params, keyShare, m, nil, 0)
r2out, _ := signing.SignRound2(ctx, sigState, r1p2p, r1bcast)
r3out, _ := signing.SignRound3(ctx, sigState, r2p2p)
// Rounds 4-9 are all broadcast:
r4out, _ := signing.SignRound4(sigState, r3)
r5out, _ := signing.SignRound5(sigState, r4)
r6out, _ := signing.SignRound6(sigState)
r7out, _ := signing.SignRound7(sigState, r5, r6)
r8out, _ := signing.SignRound8(sigState)
r9out, _ := signing.SignRound9(sigState, r7, r8)
finalOut, _ := signing.SignFinalize(sigState, r9)
// finalOut.Signature.R, .S = standard ECDSA sig
```

### Resharing (5 rounds, overlapping committees)

```go
import "github.com/hemilabs/x/tss-lib/v3/ecdsa/resharing"

// Supports overlapping committees: [P0,P1,P2] → [P1,P2,P3]
// Each committee needs its own *PartyID copies (SortPartyIDs
// mutates Index).
params := tss.NewReSharingParameters(
    tss.S256(), oldCtx, newCtx, myPID,
    oldN, oldT, newN, newT)

state, r1out, _ := resharing.ReshareRound1(params, keyShare, newPreParams)
r2out, _ := resharing.ReshareRound2(state, r1Msgs)
r3out, _ := resharing.ReshareRound3(state, r2AckMsgs)
r4out, _ := resharing.ReshareRound4(ctx, state, r2Msgs, r3p2p, r3bcast)
r5out, _ := resharing.ReshareRound5(state, r4p2p, r4bcast)
// r5out.Save = new key share (new committee), old Xi zeroed
```

## EdDSA — Keygen / Sign / Reshare

Complete runnable example: [`eddsa/example_test.go`](eddsa/example_test.go)

```
go test -tags tssexamples -v -run TestEdDSAKeygenSignReshare ./eddsa/ -timeout 5m
```

EdDSA is simpler than ECDSA: no Paillier pre-parameters, no MtA.
Keygen is 3 rounds (vs 4), signing is 3+finalize (vs 9+finalize).

### Keygen (3 rounds)

```go
import "github.com/hemilabs/x/tss-lib/v3/eddsa/keygen"

// No pre-parameters needed for EdDSA.
params := tss.NewParameters(tss.Edwards(), peerCtx, pIDs[i], n, threshold)
state, r1out, _ := keygen.Round1(params)
r2out, _ := keygen.Round2(state, r1Msgs)
r3out, _ := keygen.Round3(state, r2p2p, r2bcast)
// r3out.Save = LocalPartySaveData (key share)
```

### Signing (3 rounds + finalize)

```go
import "github.com/hemilabs/x/tss-lib/v3/eddsa/signing"

m := new(big.Int).SetBytes(msgHash[:])
sigState, r1out, _ := signing.SignRound1(params, keyShare, m, 0)
r2out, _ := signing.SignRound2(sigState, r1)
r3out, _ := signing.SignRound3(sigState, r2)
finalOut, _ := signing.SignFinalize(sigState, r3)
// finalOut.Signature.R, .S = standard EdDSA sig
// finalOut.Signature.Signature = 64-byte R||S encoding
```

### Resharing (5 rounds, overlapping committees)

```go
import "github.com/hemilabs/x/tss-lib/v3/eddsa/resharing"

// Same dual-committee pattern as ECDSA, but no Paillier.
params := tss.NewReSharingParameters(
    tss.Edwards(), oldCtx, newCtx, myPID,
    oldN, oldT, newN, newT)

state, r1out, _ := resharing.ReshareRound1(params, &keyShare)
r2out, _ := resharing.ReshareRound2(state, r1Msgs)
r3out, _ := resharing.ReshareRound3(state, r2AckMsgs)
r4out, _ := resharing.ReshareRound4(state, r1Msgs, r3p2p, r3bcast)
r5out, _ := resharing.ReshareRound5(state, r4AckMsgs)
// r5out.Save = new key share (new committee), old Xi zeroed
```

## Message Routing

Round functions return `[]*tss.Message`.  Each message has:

- `msg.From` — sender's `*tss.PartyID`
- `msg.To` — `[]*tss.PartyID` (nil = broadcast to all)
- `msg.IsBroadcast` — redundant flag for convenience
- `msg.Content` — `interface{}` holding the round-specific struct

The caller is responsible for serializing `Content` and delivering
it to the correct parties.  See the heminetwork continuum service
for a JSON-based wire format example.

## Overlapping Committees

When resharing between committees that share members (e.g.,
`[P0,P1,P2] → [P1,P2,P3]`), each committee **must** have its own
`*PartyID` copies:

```go
copyPID := func(src *tss.PartyID) *tss.PartyID {
    return tss.NewPartyID(src.Id, src.Moniker,
        new(big.Int).SetBytes(src.Key))
}
```

`SortPartyIDs` assigns `Index` by sort position within the committee.
Shared `*PartyID` objects would have their Index overwritten by the
second sort.

## Security Audit

See [SECURITY_FIXES.md](SECURITY_FIXES.md) for the complete list of
security fixes from the v2 audit (76 in v3 production code, annotated
with `[FORK]` comments).

## Packages

| Package | Description |
|---------|-------------|
| `ecdsa/keygen` | ECDSA distributed key generation (4 rounds) |
| `ecdsa/signing` | ECDSA threshold signing (9 rounds + finalize) |
| `ecdsa/resharing` | ECDSA key resharing (5 rounds, dual committee) |
| `eddsa/keygen` | EdDSA distributed key generation (3 rounds) |
| `eddsa/signing` | EdDSA threshold signing (3 rounds + finalize) |
| `eddsa/resharing` | EdDSA key resharing (5 rounds, dual committee) |
| `tss` | Core types: Parameters, PartyID, Message |
| `common` | Hash utilities, safe prime generation, random |
| `crypto` | EC points, Paillier, VSS, ZK proofs, commitments |

## Running the Demos

Standalone CLI programs that run the full lifecycle:
keygen → sign → reshare (overlapping committees) → sign.

```
# EdDSA (fast — no Paillier, ~1 second):
go run ./cmd/tss-eddsa-demo

# ECDSA (slow — Paillier safe-prime generation, ~30 seconds):
go run ./cmd/tss-ecdsa-demo
```

## Testing

```
make test         # all tests
make lint         # golangci-lint
make race         # race detector
make vulncheck    # govulncheck
```

## License

MIT — see [LICENSE](LICENSE).
