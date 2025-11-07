// Copyright (c) 2025 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package zktrie

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
)

func TestZKTrie(t *testing.T) {
	const (
		blockCount  uint64 = 11
		storageKeys uint64 = 5
	)
	var (
		home       = t.TempDir()
		cacheField = []byte("current block")
		accounts   = make(map[common.Address][][]byte, blockCount+1)
		manualAcc  = common.BytesToAddress(random(20))
	)
	accounts[manualAcc] = nil

	zkt, err := NewZKTrie(t.Context(), home)
	if err != nil {
		t.Fatal(err)
	}
	if err := zkt.Put([]byte("hello"), []byte("world")); err != nil {
		t.Fatal(err)
	}
	v, err := zkt.Get([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(v, []byte("world")) {
		t.Fatalf("got %s, wanted %s", v, []byte("world"))
	}

	for i := range blockCount {
		blk := ZKBlock{
			Height:   i,
			Storage:  make(map[common.Address]map[common.Hash][]byte, 1),
			Accounts: make(map[common.Address]types.StateAccount, 1),
		}

		var md []byte
		if i%2 == 0 {
			// delete and create every other block
			md = fmt.Appendf(nil, "%d", blk.Height)
		}
		blk.Storage[MetadataAddress] = map[common.Hash][]byte{
			crypto.Keccak256Hash(cacheField): md,
		}

		randAcc := common.BytesToAddress(random(20))
		blk.Storage[randAcc] = make(map[common.Hash][]byte, storageKeys)
		accounts[randAcc] = make([][]byte, storageKeys)
		for i := range storageKeys {
			newKey := random(10)
			hash := crypto.Keccak256Hash(newKey)
			accounts[randAcc][i] = newKey
			blk.Storage[randAcc][hash] = random(32)
		}

		manualState := types.NewEmptyStateAccount()
		manualState.Balance = uint256.NewInt(10 * (i + 1))
		blk.Accounts[manualAcc] = *manualState

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

	md, err := zkt.MetadataGet(cacheField)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("stored metadata: %s", md)

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
