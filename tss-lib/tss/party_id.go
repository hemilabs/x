// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package tss

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sort"

	"github.com/hemilabs/x/tss-lib/v2/common"
)

type (
	// PartyID represents a participant in the TSS protocol rounds.
	// Note: The `id` and `moniker` are provided for convenience to allow you to track participants easier.
	// The `id` is intended to be a unique string representation of `key` and `moniker` can be anything (even left blank).
	PartyID struct {
		*MessageWrapper_PartyID
		Index int `json:"index"`
	}

	UnSortedPartyIDs []*PartyID
	SortedPartyIDs   []*PartyID
)

// [FORK] Upstream checks `pid.Key != nil`, but an empty byte slice (len 0) is
// equally invalid since KeyInt() would return 0, colliding with the VSS secret
// coefficient. Changed to `len(pid.Key) > 0` for defense in depth.
func (pid *PartyID) ValidateBasic() bool {
	return pid != nil && len(pid.Key) > 0 && 0 <= pid.Index
}

// --- ProtoBuf Extensions

func (mpid *MessageWrapper_PartyID) KeyInt() *big.Int {
	return new(big.Int).SetBytes(mpid.Key)
}

// ----- //

// NewPartyID constructs a new PartyID.
// Exported, used in `tss` client. `key` should remain consistent between runs for each party.
//
// [FORK] Note on key range: internally, all polynomial evaluation and Lagrange
// interpolation operates mod q (the curve order). A key >= q is treated as
// equivalent to (key mod q) in all arithmetic. CheckIndexes catches mod-q
// collisions (two distinct keys that are congruent mod q) and mod-q zero.
// Callers that derive keys from 256-bit hashes (e.g., keccak256) may produce
// values >= q on curves with smaller orders (e.g., Ed25519 q ≈ 2^252.8);
// this is handled correctly by the modular arithmetic throughout the library.
func NewPartyID(id, moniker string, key *big.Int) *PartyID {
	return &PartyID{
		MessageWrapper_PartyID: &MessageWrapper_PartyID{
			Id:      id,
			Moniker: moniker,
			Key:     key.Bytes(),
		},
		Index: -1, // not known until sorted
	}
}

func (pid PartyID) String() string {
	return fmt.Sprintf("{%d,%s}", pid.Index, pid.Moniker)
}

// ----- //

// SortPartyIDs sorts a list of []*PartyID by their keys in ascending order
// Exported, used in `tss` client
//
// [FORK] Added post-sort validation that upstream lacks:
//   - Zero-key rejection: VSS polynomial evaluation at x=0 yields the secret coefficient a_0.
//     Allowing Key=0 would let a party trivially extract the group secret.
//   - Duplicate-key rejection: two parties with the same key produce identical Lagrange
//     coefficients, breaking the (t,n) threshold guarantee and causing division-by-zero
//     in interpolation.
func SortPartyIDs(ids UnSortedPartyIDs, startAt ...int) SortedPartyIDs {
	sorted := make(SortedPartyIDs, 0, len(ids))
	for _, id := range ids {
		sorted = append(sorted, id)
	}
	sort.Sort(sorted)
	// Reject zero keys — VSS polynomial evaluation at zero yields the secret coefficient.
	// Also reject duplicate keys — they cause non-deterministic sort and break threshold invariants.
	zero := big.NewInt(0)
	for i := 0; i < len(sorted); i++ {
		if sorted[i].KeyInt().Cmp(zero) == 0 {
			panic(fmt.Sprintf("SortPartyIDs: party at index %d has Key=0; VSS evaluation at zero leaks the secret", i))
		}
		if i > 0 && sorted[i-1].KeyInt().Cmp(sorted[i].KeyInt()) == 0 {
			panic(fmt.Sprintf("SortPartyIDs: duplicate key at indices %d and %d (key=%s)",
				i-1, i, sorted[i].KeyInt()))
		}
	}
	// assign party indexes
	for i, id := range sorted {
		frm := 0
		if len(startAt) > 0 {
			frm = startAt[0]
		}
		id.Index = i + frm
	}
	return sorted
}

// GenerateTestPartyIDs generates a list of mock PartyIDs for tests
func GenerateTestPartyIDs(count int, startAt ...int) SortedPartyIDs {
	ids := make(UnSortedPartyIDs, 0, count)
	key := common.MustGetRandomInt(rand.Reader, 256)
	frm := 0
	i := 0 // default `i`
	if len(startAt) > 0 {
		frm = startAt[0]
		i = startAt[0]
	}
	for ; i < count+frm; i++ {
		ids = append(ids, &PartyID{
			MessageWrapper_PartyID: &MessageWrapper_PartyID{
				Id:      fmt.Sprintf("%d", i+1),
				Moniker: fmt.Sprintf("P[%d]", i+1),
				Key:     new(big.Int).Sub(key, big.NewInt(int64(count)-int64(i))).Bytes(),
			},
			Index: i,
			// this key makes tests more deterministic
		})
	}
	return SortPartyIDs(ids, startAt...)
}

func (spids SortedPartyIDs) Keys() []*big.Int {
	ids := make([]*big.Int, spids.Len())
	for i, pid := range spids {
		ids[i] = pid.KeyInt()
	}
	return ids
}

func (spids SortedPartyIDs) ToUnSorted() UnSortedPartyIDs {
	return UnSortedPartyIDs(spids)
}

func (spids SortedPartyIDs) FindByKey(key *big.Int) *PartyID {
	for _, pid := range spids {
		if pid.KeyInt().Cmp(key) == 0 {
			return pid
		}
	}
	return nil
}

func (spids SortedPartyIDs) Exclude(exclude *PartyID) SortedPartyIDs {
	newSpIDs := make(SortedPartyIDs, 0, len(spids))
	for _, pid := range spids {
		if pid.KeyInt().Cmp(exclude.KeyInt()) == 0 {
			continue // exclude
		}
		newSpIDs = append(newSpIDs, pid)
	}
	return newSpIDs
}

// Sortable

func (spids SortedPartyIDs) Len() int {
	return len(spids)
}

// [FORK] Upstream uses `<= 0` (i.e., less-or-equal), which treats equal keys as
// "less than" each other. This violates the strict weak ordering contract
// required by sort.Interface and can produce non-deterministic sort results.
// Changed to `< 0` for strict less-than comparison.
func (spids SortedPartyIDs) Less(a, b int) bool {
	return spids[a].KeyInt().Cmp(spids[b].KeyInt()) < 0
}

func (spids SortedPartyIDs) Swap(a, b int) {
	spids[a], spids[b] = spids[b], spids[a]
}
