// Copyright (c) 2025 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package zktrie

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"io"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestZKTrie(t *testing.T) {
	const (
		blockCount  uint64 = 11
		storageKeys uint64 = 5
	)
	home := t.TempDir()

	zkt, err := NewZKTrie(home)
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

	prevBlock := *chaincfg.TestNet3Params.GenesisHash
	prevStateRoot := types.EmptyRootHash
	outpoints := make(map[uint64][]Outpoint)
	for i := range blockCount {
		bh := chainhash.Hash(random(32))
		blk := NewZKBlock(bh, prevBlock, prevStateRoot, i)

		// simulate outs
		var pkScript [8]byte
		binary.BigEndian.PutUint64(pkScript[:], i)
		outpoints[i] = make([]Outpoint, 0)
		for range storageKeys {
			o := NewOutpoint([32]byte(random(32)), 1)
			so := NewSpendableOutput(blk.blockHash, [32]byte(random(32)), 1, 100)
			blk.NewOut(pkScript[:], o, so)
			outpoints[i] = append(outpoints[i], o)
		}

		// simulate an in
		for ac, keys := range outpoints {
			if len(keys) <= 1 {
				continue
			}
			var pkScript [8]byte
			binary.BigEndian.PutUint64(pkScript[:], ac)
			o := keys[0]
			outpoints[ac] = keys[1:]
			so := NewSpentOutput(blk.blockHash, [32]byte(random(32)), 1)
			blk.NewIn(pkScript[:], o, so)
			break
		}

		sr, err := zkt.InsertBlock(blk)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("inserted block %d, new state root: %v", blk.GetMetadata().Height(), sr)
		prevBlock = bh
		prevStateRoot = sr
	}

	if err := zkt.Commit(); err != nil {
		t.Fatal(err)
	}

	for ac, keys := range outpoints {
		var pkScript [8]byte
		binary.BigEndian.PutUint64(pkScript[:], ac)
		addr := common.BytesToAddress(pkScript[:])
		sa, err := zkt.GetAccount(addr)
		if err != nil {
			t.Fatal(err)
		}
		spew.Dump(sa)

		for _, k := range keys {
			v, err := zkt.GetOutpoint(pkScript[:], k)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("address %x, key %x, value %x", ac, k, v)
		}
	}

	md, err := zkt.GetBlockInfo(prevBlock)
	if err != nil {
		t.Fatal(err)
	}
	spew.Dump(md)

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
