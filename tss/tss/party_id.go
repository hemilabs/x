// Copyright © 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.
package tss

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sort"

	"github.com/hemilabs/x/tss-lib/v3/common"
)

// PartyIDData holds the wire-format fields for a party identifier.
// Replaces the protobuf MessageWrapper_PartyID.
type PartyIDData struct {
	Id      string `json:"id"`
	Moniker string `json:"moniker"`
	Key     []byte `json:"key"`
}

// KeyInt returns the party key as a *big.Int.
func (d *PartyIDData) KeyInt() *big.Int {
	return new(big.Int).SetBytes(d.Key)
}

type (
	// PartyID represents a participant in the TSS protocol rounds.
	PartyID struct {
		*PartyIDData
		Index int `json:"index"`
	}

	UnSortedPartyIDs []*PartyID
	SortedPartyIDs   []*PartyID
)

// ValidateBasic checks that the PartyID has a non-empty key and
// non-negative index.
func (pid *PartyID) ValidateBasic() bool {
	return pid != nil && len(pid.Key) > 0 && 0 <= pid.Index
}

// NewPartyID constructs a new PartyID.
func NewPartyID(id, moniker string, key *big.Int) *PartyID {
	return &PartyID{
		PartyIDData: &PartyIDData{
			Id:      id,
			Moniker: moniker,
			Key:     key.Bytes(),
		},
		Index: -1,
	}
}

// String returns a human-readable representation of the party ID.
func (pid PartyID) String() string {
	return fmt.Sprintf("{%d,%s}", pid.Index, pid.Moniker)
}

// SortPartyIDs sorts a list of []*PartyID by their keys in ascending
// order and validates no zero or duplicate keys.
func SortPartyIDs(ids UnSortedPartyIDs, startAt ...int) SortedPartyIDs {
	sorted := make(SortedPartyIDs, 0, len(ids))
	for _, id := range ids {
		sorted = append(sorted, id)
	}
	sort.Sort(sorted)
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
	for i, id := range sorted {
		frm := 0
		if len(startAt) > 0 {
			frm = startAt[0]
		}
		id.Index = i + frm
	}
	return sorted
}

// GenerateTestPartyIDs generates a list of mock PartyIDs for tests.
func GenerateTestPartyIDs(count int, startAt ...int) SortedPartyIDs {
	ids := make(UnSortedPartyIDs, 0, count)
	key := common.MustGetRandomInt(rand.Reader, 256)
	frm := 0
	i := 0
	if len(startAt) > 0 {
		frm = startAt[0]
		i = startAt[0]
	}
	for ; i < count+frm; i++ {
		ids = append(ids, &PartyID{
			PartyIDData: &PartyIDData{
				Id:      fmt.Sprintf("%d", i+1),
				Moniker: fmt.Sprintf("P[%d]", i+1),
				Key:     new(big.Int).Sub(key, big.NewInt(int64(count)-int64(i))).Bytes(),
			},
			Index: i,
		})
	}
	return SortPartyIDs(ids, startAt...)
}

// Keys returns the big.Int keys of all party IDs in sorted order.
func (spids SortedPartyIDs) Keys() []*big.Int {
	ids := make([]*big.Int, spids.Len())
	for i, pid := range spids {
		ids[i] = pid.KeyInt()
	}
	return ids
}

// ToUnSorted returns the party IDs as an unsorted slice.
func (spids SortedPartyIDs) ToUnSorted() UnSortedPartyIDs {
	return UnSortedPartyIDs(spids)
}

// FindByKey returns the PartyID with the given key, or nil.
func (spids SortedPartyIDs) FindByKey(key *big.Int) *PartyID {
	for _, pid := range spids {
		if pid.KeyInt().Cmp(key) == 0 {
			return pid
		}
	}
	return nil
}

// Exclude returns a new sorted slice with the given party removed.
func (spids SortedPartyIDs) Exclude(exclude *PartyID) SortedPartyIDs {
	newSpIDs := make(SortedPartyIDs, 0, len(spids))
	for _, pid := range spids {
		if pid.KeyInt().Cmp(exclude.KeyInt()) == 0 {
			continue
		}
		newSpIDs = append(newSpIDs, pid)
	}
	return newSpIDs
}

// Sortable
func (spids SortedPartyIDs) Len() int { return len(spids) }

// Less implements sort.Interface.
func (spids SortedPartyIDs) Less(a, b int) bool {
	return spids[a].KeyInt().Cmp(spids[b].KeyInt()) < 0
}

// Swap implements sort.Interface.
func (spids SortedPartyIDs) Swap(a, b int) {
	spids[a], spids[b] = spids[b], spids[a]
}
