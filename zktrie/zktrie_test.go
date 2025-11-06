// Copyright (c) 2025 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package zktrie

import (
	"crypto/rand"
	"io"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestZKTrie(t *testing.T) {
	const (
		blockCount  uint64 = 10
		storageKeys uint64 = 5
	)

	home := t.TempDir()
	zkt, err := NewZKTrie(t.Context(), home)
	if err != nil {
		t.Fatal(err)
	}

	if err := zkt.Put([]byte("hello"), []byte("world")); err != nil {
		t.Fatal(err)
	}

	_, err = zkt.Get([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}

	accounts := make(map[common.Address][]common.Hash, blockCount)
	for i := range blockCount {
		blk := ZKBlock{
			Height:  i,
			Storage: make(map[common.Address]map[common.Hash][]byte),
		}

		var randAcc common.Address
		randAcc.SetBytes(random(20))

		blk.Storage[randAcc] = make(map[common.Hash][]byte, storageKeys)
		accounts[randAcc] = make([]common.Hash, storageKeys)
		for i := range storageKeys {
			var hash common.Hash
			hash.SetBytes(random(32))
			accounts[randAcc][i] = hash
			blk.Storage[randAcc][hash] = random(32)
		}

		sr, err := zkt.InsertBlock(&blk)
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("inserted block %d, new state root: %v", blk.Height, sr)
	}

	if err := zkt.Commit(); err != nil {
		t.Fatal(err)
	}

	for ac, keys := range accounts {
		sa, err := zkt.GetAccount(ac)
		if err != nil {
			t.Fatal(err)
		}

		spew.Dump(sa)

		for _, k := range keys {
			v, err := zkt.GetStorage(ac, k)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("address %x, key %x, value %x", ac, k, v)
		}
	}

	if err := zkt.Recover(types.EmptyRootHash); err != nil {
		t.Fatal(err)
	}
}

// Random returns a variable number of random bytes.
func random(n int) []byte {
	buffer := make([]byte, n)
	_, err := io.ReadFull(rand.Reader, buffer)
	if err != nil {
		panic(err)
	}
	return buffer
}
