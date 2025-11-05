package trie_test

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/ethdb/leveldb"

	"github.com/hemilabs/x/eth-trie/internal/testutil"
	"github.com/hemilabs/x/eth-trie/triedb"
	"github.com/hemilabs/x/eth-trie/triedb/pathdb"
)

func TestZKTrie(t *testing.T) {
	const (
		blockCount uint64 = 1000000 // num of blocks
		// newOutsCount >= inCount + outCount
		newOutsCount uint64 = 1500 // num of outs with new scripts
		inCount      uint64 = 1000 // num of ins
		outCount     uint64 = 500  // num of outs with previous scripts
	)

	datadir := t.TempDir()

	// Open LevelDB database as the underlying disk DB
	db, err := leveldb.New(datadir, 0, 0, "", false)
	if err != nil {
		panic(err)
	}

	// You can now use db as a KeyValueStore
	var kv ethdb.KeyValueStore = db

	// high-level database wrapper for the given key-value store
	disk, err := rawdb.Open(kv, rawdb.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// Create triedb using pathDB.
	//
	// Basically, consists of one persistent base layer backed by a key-value
	// store (which is rawdb wrapped level), on top of which arbitrarily many
	// in-memory diff layers are stacked.
	//
	// On startup, attempts to load an already existing layer from the rawDB
	// store (with a number of memory layers from a journal). If the journal is not
	// matched with the base persistent layer, all the recorded diff layers are discarded.
	tdb := triedb.NewDatabase(disk, &triedb.Config{
		PathDB: &pathdb.Config{
			NoAsyncFlush: true,
		},
	})
	defer func() {
		if err := tdb.Close(); err != nil {
			t.Logf("ERROR: %v", err)
		}
	}()

	holder := NewTestHolder(tdb)
	for blk := range blockCount {
		realDuration := time.Now()
		for range newOutsCount {
			pkScript := common.BytesToAddress(testutil.Random(common.AddressLength))
			if err := holder.SimulateOut(pkScript, 1000); err != nil {
				t.Fatal(err)
			}
		}
		if blk != 0 {
			for range outCount {
				os := holder.outpoints[0]
				holder.outpoints = holder.outpoints[1:]
				pkScript := os.GetAddress()
				if err := holder.SimulateOut(pkScript, 75); err != nil {
					t.Fatal(err)
				}
			}
			for range inCount {
				os := holder.outpoints[0]
				holder.outpoints = holder.outpoints[1:]
				if err := holder.SimulateIn(os); err != nil {
					t.Fatal(err)
				}
			}
		}
		commitDuration := time.Now()
		if err := holder.CommitBlock(blk); err != nil {
			t.Fatal(err)
		}
		t.Logf("block %d: commited in %v, total time %v",
			blk, time.Since(commitDuration), time.Since(realDuration))
	}

	if err := tdb.Recover(types.EmptyRootHash); err != nil {
		t.Fatal(err)
	}
}
