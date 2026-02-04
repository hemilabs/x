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

	// Create an initial empty block to establish the empty state in the database
	emptyBlock := NewZKBlock(chainhash.Hash([32]byte{}), *chaincfg.TestNet3Params.GenesisHash, types.EmptyRootHash, 0)
	initialStateRoot, err := zkt.InsertBlock(emptyBlock)
	if err != nil {
		t.Fatal(err)
	}
	if err := zkt.Commit(); err != nil {
		t.Fatal(err)
	}

	prevBlock := chainhash.Hash([32]byte{})
	prevStateRoot := initialStateRoot
	outpoints := make(map[uint64][]Outpoint)
	states := make([]common.Hash, blockCount)
	blocks := make([]chainhash.Hash, blockCount)
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
		states[i] = sr
		blocks[i] = bh
	}

	if err := zkt.Commit(); err != nil {
		t.Fatal(err)
	}

	for _, state := range states {
		var found int
		for ac, keys := range outpoints {
			var pkScript [8]byte
			binary.BigEndian.PutUint64(pkScript[:], ac)
			addr := common.BytesToAddress(pkScript[:])
			sa, err := zkt.GetAccount(addr, &state)
			if err != nil {
				t.Fatal(err)
			}
			spew.Dump(sa)
			for _, k := range keys {
				v, err := zkt.GetOutpoint(pkScript[:], k, &state)
				if err != nil {
					t.Fatal(err)
				}
				if v != nil {
					found++
					t.Logf("address %x, key %x, value %x", ac, k, v)
				}
			}
		}
		if found < 1 {
			t.Fatalf("no outpoints retrieved for state %x", state)
		}
	}

	for _, blk := range blocks {
		md, err := zkt.GetBlockInfo(blk, nil)
		if err != nil {
			t.Fatal(err)
		}
		spew.Dump(md)
	}

	for i := len(states) - 2; i >= 0; i-- {
		if err := zkt.Recover(states[i]); err != nil {
			t.Fatal(err)
		}
		t.Logf("Rolled back to state %v", states[i])
	}

	if err := zkt.Recover(initialStateRoot); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkZKTrie(b *testing.B) {
	const (
		newAddressNum         uint64 = 10000
		reuseAddressNum       uint64 = 10000
		outpointPerReusedAddr uint64 = 5
		outpointPerNewAddr    uint64 = 5
	)
	home := b.TempDir()

	zkt, err := NewZKTrie(home)
	if err != nil {
		b.Fatal(err)
	}

	// Pre-insert N outs for reuse
	prevBlock := *chaincfg.TestNet3Params.GenesisHash
	prevStateRoot := types.EmptyRootHash
	outpoints := make(map[uint64][]Outpoint)

	bh := chainhash.Hash(random(32))
	blk := NewZKBlock(bh, prevBlock, prevStateRoot, 0)

	// simulate outs
	for i := range reuseAddressNum {
		var pkScript [8]byte
		binary.BigEndian.PutUint64(pkScript[:], i)
		outpoints[i] = make([]Outpoint, outpointPerReusedAddr)
		for j := range outpointPerReusedAddr {
			o := NewOutpoint([32]byte(random(32)), 1)
			so := NewSpendableOutput(blk.blockHash, [32]byte(random(32)), 1, 100)
			blk.NewOut(pkScript[:], o, so)
			outpoints[i][j] = o
		}
	}

	sr, err := zkt.InsertBlock(blk)
	if err != nil {
		b.Fatal(err)
	}
	b.Logf("inserted block %d, new state root: %v", blk.GetMetadata().Height(), sr)

	if err := zkt.Commit(); err != nil {
		b.Fatal(err)
	}

	bhIn := chainhash.Hash(random(32))
	blkIn := NewZKBlock(bhIn, bh, sr, 1)

	for i := range reuseAddressNum {
		var pkScript [8]byte
		binary.BigEndian.PutUint64(pkScript[:], i)
		for _, o := range outpoints[i] {
			so := NewSpentOutput(blkIn.blockHash, [32]byte(random(32)), 100)
			blkIn.NewIn(pkScript[:], o, so)
		}
	}

	for i := range newAddressNum {
		var pkScript [8]byte
		binary.BigEndian.PutUint64(pkScript[:], i+reuseAddressNum)
		for range outpointPerNewAddr {
			o := NewOutpoint([32]byte(random(32)), 1)
			so := NewSpendableOutput(blkIn.blockHash, [32]byte(random(32)), 1, 100)
			blkIn.NewOut(pkScript[:], o, so)
		}
	}

	b.Run("Block Insert", func(b *testing.B) {
		for b.Loop() {
			_, err := zkt.InsertBlock(blkIn)
			if err != nil {
				b.Fatal(err)
			}
			// if err := zkt.Commit(); err != nil {
			// 	b.Fatal(err)
			// }
			// if err := zkt.Recover(sr); err != nil {
			// 	b.Fatal(err)
			// }
		}
	})

	b.Run("Block Commit And Revert", func(b *testing.B) {
		for b.Loop() {
			_, err := zkt.InsertBlock(blkIn)
			if err != nil {
				b.Fatal(err)
			}
			if err := zkt.Commit(); err != nil {
				b.Fatal(err)
			}
			if err := zkt.Recover(sr); err != nil {
				b.Fatal(err)
			}
		}
	})

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
