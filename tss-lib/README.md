# Hemi TSS Library (v3)

[![MIT licensed](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Threshold signature scheme library for ECDSA (secp256k1), forked from
[binance/tss-lib](https://github.com/bnb-chain/tss-lib) with 112
security audit fixes and a channel-free round function API.

## v3 API

v3 replaces the channel-based `Party`/`Round`/`BaseUpdate` state
machine with pure round functions.  Each round takes explicit state +
inbound messages and returns outbound messages.  The caller owns the
event loop.

**No protobuf.**  All message types are plain Go structs with typed
fields.  Serialization is the caller's responsibility — the library
does not prescribe a wire format.

## ECDSA Example — Keygen + Sign

A complete runnable example lives at
[`ecdsa/example_test.go`](ecdsa/example_test.go).  Run it with:

```
go test -tags tssexamples -v -run TestECDSAKeygenAndSign ./ecdsa/ -timeout 10m
```

### Key Generation (4 rounds)

```go
import (
    "github.com/hemilabs/x/tss-lib/v3/ecdsa/keygen"
    "github.com/hemilabs/x/tss-lib/v3/tss"
)

// Step 1: Generate Paillier pre-parameters (CPU-intensive, do once).
preParams, _ := keygen.GeneratePreParams(5 * time.Minute)

// Step 2: Create party IDs and peer context.
pIDs := tss.GenerateTestPartyIDs(n)
peerCtx := tss.NewPeerContext(pIDs)
params := tss.NewParameters(tss.S256(), peerCtx, pIDs[i], n, threshold)

// Step 3: Run 4 rounds.  Each round returns outbound messages
// that must be delivered to all other parties before the next
// round can begin.
state, r1out, _ := keygen.Round1(ctx, params, *preParams)
// deliver r1out.Messages to all parties, collect r1Msgs
r2out, _ := keygen.Round2(ctx, state, r1Msgs)
// r2out.Messages contains both P2P and broadcast messages:
//   msg.To == nil → broadcast to all
//   msg.To != nil → send to each listed party
// Also export self-messages:
//   state.ExportR2P2PSelf(), state.ExportR2BcastSelf()
r3out, _ := keygen.Round3(ctx, state, r2p2p, r2bcast)
r4out, _ := keygen.Round4(ctx, state, r3Msgs)
// r4out.Save contains the key share (LocalPartySaveData).
```

### Threshold Signing (9 rounds + finalize)

```go
import "github.com/hemilabs/x/tss-lib/v3/ecdsa/signing"

msgHash := sha256.Sum256([]byte("hello"))
m := new(big.Int).SetBytes(msgHash[:])

// Round 1 returns P2P (Paillier ciphertext) and broadcast (commitment).
sigState, r1out, _ := signing.SignRound1(params, keyShare, m, nil, 0)
// Rounds 2-3: MtA protocol (P2P), theta/sigma computation.
r2out, _ := signing.SignRound2(ctx, sigState, r1p2p, r1bcast)
r3out, _ := signing.SignRound3(ctx, sigState, r2p2p)
// Rounds 4-9: Schnorr proofs, commitment/decommitment, partial sigs.
r4out, _ := signing.SignRound4(sigState, r3)
r5out, _ := signing.SignRound5(sigState, r4)
r6out, _ := signing.SignRound6(sigState)
r7out, _ := signing.SignRound7(sigState, r5, r6)
r8out, _ := signing.SignRound8(sigState)
r9out, _ := signing.SignRound9(sigState, r7, r8)
// Finalize: sum partial signatures, verify ECDSA.
finalOut, _ := signing.SignFinalize(sigState, r9)
// finalOut.Signature.R, finalOut.Signature.S — standard ECDSA sig.
```

### Key Resharing (5 rounds)

```go
import "github.com/hemilabs/x/tss-lib/v3/ecdsa/resharing"

// Dual-committee protocol: old committee transfers key shares
// to a new committee without reconstructing the private key.
reshareState, r1out, _ := resharing.ReshareRound1(params, keyShare, newPreParams)
r2out, _ := resharing.ReshareRound2(reshareState, r1Msgs)
r3out, _ := resharing.ReshareRound3(reshareState, r2AckMsgs)
r4out, _ := resharing.ReshareRound4(ctx, reshareState, r1Msgs, r2Msgs, r3p2p, r3bcast)
r5out, _ := resharing.ReshareRound5(ctx, reshareState, r2Msgs, r4p2p, r4AckMsgs)
// r5out.Save contains the new key share (new committee members only).
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

## Security Audit

See [FORK_CHANGES.md](FORK_CHANGES.md) for the complete list of 112
security fixes with v3 code locations.

## Packages

| Package | Description |
|---------|-------------|
| `ecdsa/keygen` | ECDSA distributed key generation (4 rounds) |
| `ecdsa/signing` | ECDSA threshold signing (9 rounds + finalize) |
| `ecdsa/resharing` | ECDSA key resharing (5 rounds, dual committee) |
| `tss` | Core types: Parameters, PartyID, Message |
| `common` | Hash utilities, safe prime generation, random |
| `crypto` | EC points, Paillier, VSS, ZK proofs, commitments |

## Testing

```
make test         # all tests
make lint         # golangci-lint
make race         # race detector
make vulncheck    # govulncheck

# Run the ECDSA example (slow — generates Paillier primes):
go test -tags tssexamples -v -run TestECDSAKeygenAndSign ./ecdsa/ -timeout 10m
```

## License

MIT — see [LICENSE](LICENSE).
