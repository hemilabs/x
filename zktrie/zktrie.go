// Copyright (c) 2025 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package zktrie

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/ethdb/leveldb"
	"github.com/hemilabs/x/eth-trie/trie"
	"github.com/hemilabs/x/eth-trie/trie/trienode"
	"github.com/hemilabs/x/eth-trie/triedb"
	"github.com/hemilabs/x/eth-trie/triedb/pathdb"
)

var metadataAddress common.Address

func init() {
	const reserved string = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"
	ab, err := hex.DecodeString(reserved)
	if err != nil {
		panic(fmt.Errorf("error parsing address %v: %w", reserved, err))
	}
	metadataAddress.SetBytes(ab)
}

// ZKBlock holds information on a block used for a ZK Trie state transition.
type ZKBlock struct {
	Height uint64

	// Storage information. Automatically updates the AccountState
	// that each storage is associated with.
	Storage map[common.Address]map[common.Hash][]byte

	// Accounts information. Overrides any values passed for the
	// same address in storage.
	Accounts map[common.Address]types.StateAccount
}

// ZKTrie is used to perform operation on a ZK trie
// and its underlying database. It is not concurrency safe.
type ZKTrie struct {
	stateRoots []common.Hash
	tdb        *triedb.Database
}

// TODO: set database cache and handles
func NewZKTrie(_ context.Context, home string) (*ZKTrie, error) {
	db, err := leveldb.New(home, 0, 0, "", false)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// You can now use db as a KeyValueStore
	var kv ethdb.KeyValueStore = db

	// high-level database wrapper for the given key-value store
	disk, err := rawdb.Open(kv, rawdb.OpenOptions{})
	if err != nil {
		return nil, fmt.Errorf("open rawdb: %w", err)
	}
	tdb := triedb.NewDatabase(disk, &triedb.Config{
		PathDB: &pathdb.Config{
			NoAsyncFlush: true,
		},
	})

	t := &ZKTrie{
		tdb:        tdb,
		stateRoots: make([]common.Hash, 0),
	}
	return t, nil
}

// Close closes the underlying database for ZKTrie.
func (t *ZKTrie) Close() error {
	return t.tdb.Close()
}

// Recover rollbacks the ZKTrie database to a specified historical point.
func (t *ZKTrie) Recover(stateRoot common.Hash) error {
	return t.tdb.Recover(stateRoot)
}

// Put performs an insert into the underlying ZKTrie database.
func (t *ZKTrie) Put(key, value []byte) error {
	return t.tdb.Disk().Put(key, value)
}

// Get performs a lookup on the underlying ZKTrie database.
func (t *ZKTrie) Get(key []byte) ([]byte, error) {
	return t.tdb.Disk().Get(key)
}

func (t *ZKTrie) GetStorage(addr common.Address, key common.Hash) ([]byte, error) {
	var (
		stateRoot = t.currentState()
		addrHash  = crypto.Keccak256Hash(addr.Bytes())
	)
	sa, err := t.GetAccount(addr)
	if err != nil {
		return nil, fmt.Errorf("get state account: %w", err)
	}
	if sa == nil {
		return nil, nil
	}

	storeID := trie.StorageTrieID(stateRoot, addrHash, sa.Root)
	storeTrie, err := trie.New(storeID, t.tdb)
	if err != nil {
		return nil, fmt.Errorf("failed to load trie, err: %w", err)
	}
	return storeTrie.Get(key[:])
}

func (t *ZKTrie) GetAccount(addr common.Address) (*types.StateAccount, error) {
	var (
		stateRoot = t.currentState()
		stateID   = trie.StateTrieID(stateRoot)
		addrHash  = crypto.Keccak256Hash(addr.Bytes())
	)
	stateTrie, err := trie.New(stateID, t.tdb)
	if err != nil {
		return nil, fmt.Errorf("failed to load state trie, err: %w", err)
	}
	stateVal, err := stateTrie.Get(addrHash[:])
	if err != nil {
		return nil, fmt.Errorf("get account state: %w", err)
	}
	if stateVal == nil {
		return nil, nil
	}
	sa, err := types.FullAccount(stateVal)
	if err != nil {
		return nil, fmt.Errorf("error restoring account: %w", err)
	}
	return sa, nil
}

// currentState gets the most recent State Root.
func (t *ZKTrie) currentState() common.Hash {
	if len(t.stateRoots) == 0 {
		return types.EmptyRootHash
	}
	return t.stateRoots[len(t.stateRoots)-1]
}

// TODO: update StateAccount balance
// InsertBlock performs a state transition for a given block.
func (t *ZKTrie) InsertBlock(block *ZKBlock) (common.Hash, error) {
	var (
		stateRoot    = t.currentState()
		mergeSet     = trienode.NewMergedNodeSet()
		stateID      = trie.StateTrieID(stateRoot)
		mutatedAcc   = make(map[common.Hash][]byte, len(block.Storage)+len(block.Accounts))
		originAcc    = make(map[common.Address][]byte, len(block.Storage)+len(block.Accounts))
		mutatedStore = make(map[common.Hash]map[common.Hash][]byte, len(block.Storage))
		originStore  = make(map[common.Address]map[common.Hash][]byte, len(block.Storage))
	)
	stateTrie, err := trie.New(stateID, t.tdb)
	if err != nil {
		return types.EmptyRootHash, fmt.Errorf("failed to load state trie, err: %w", err)
	}

	for addr, storage := range block.Storage {
		if _, ok := block.Accounts[addr]; ok {
			continue
		}

		addrHash := crypto.Keccak256Hash(addr.Bytes())
		stateVal, err := stateTrie.Get(addrHash[:])
		if err != nil {
			return types.EmptyRootHash, fmt.Errorf("get account state: %w", err)
		}

		sa := types.NewEmptyStateAccount()
		if stateVal != nil {
			sa, err = types.FullAccount(stateVal)
			if err != nil {
				return types.EmptyRootHash, fmt.Errorf("stored value for %v != stateAccount: %w", addr, err)
			}
		}
		na := types.StateAccount{
			Balance:  sa.Balance,
			CodeHash: sa.CodeHash,
			Nonce:    sa.Nonce,
		}

		storeID := trie.StorageTrieID(stateRoot, addrHash, sa.Root)
		storeTrie, err := trie.New(storeID, t.tdb)
		if err != nil {
			return types.EmptyRootHash, fmt.Errorf("failed to load trie, err: %w", err)
		}

		mutatedStore[addrHash] = make(map[common.Hash][]byte, len(block.Storage[addr]))
		originStore[addr] = make(map[common.Hash][]byte, len(block.Storage[addr]))
		for key, value := range storage {
			prev, err := storeTrie.Get(key[:])
			if err != nil {
				return types.EmptyRootHash, fmt.Errorf("get storage trie value: %w", err)
			}
			originStore[addr][key] = prev
			mutatedStore[addrHash][key] = value
		}

		for k, v := range mutatedStore[addrHash] {
			if err := storeTrie.Update(k[:], v); err != nil {
				return types.EmptyRootHash, fmt.Errorf("update storage trie: %w", err)
			}
		}

		// commit the trie, get storage trie root and node set
		newStorageRoot, set := storeTrie.Commit(false)
		if err := mergeSet.Merge(set); err != nil {
			return types.EmptyRootHash, fmt.Errorf("merge storage nodes: %w", err)
		}
		na.Root = newStorageRoot

		mutatedAcc[addrHash] = types.SlimAccountRLP(na)
		originAcc[addr] = stateVal
	}

	for addr, sa := range block.Accounts {
		addrHash := crypto.Keccak256Hash(addr.Bytes())
		stateVal, err := stateTrie.Get(addrHash[:])
		if err != nil {
			return types.EmptyRootHash, fmt.Errorf("get account state: %w", err)
		}
		originAcc[addr] = stateVal
		mutatedAcc[addrHash] = types.SlimAccountRLP(sa)
	}

	for key, val := range mutatedAcc {
		if err := stateTrie.Update(key.Bytes(), val); err != nil {
			return types.EmptyRootHash, fmt.Errorf("update accounts trie: %w", err)
		}
	}

	// commit the trie, get state trie root and node set
	newStateRoot, set := stateTrie.Commit(false)
	if err := mergeSet.Merge(set); err != nil {
		return types.EmptyRootHash, fmt.Errorf("merge account nodes: %w", err)
	}

	// StateSet represents a collection of mutated states during a state transition.
	s := triedb.StateSet{
		Accounts:       mutatedAcc,
		AccountsOrigin: originAcc,
		Storages:       mutatedStore,
		StoragesOrigin: originStore,
		RawStorageKey:  false,
	}

	// performs a state transition
	if err := t.tdb.Update(newStateRoot, stateRoot, block.Height, mergeSet, &s); err != nil {
		return types.EmptyRootHash, fmt.Errorf("update db: %w", err)
	}

	t.stateRoots = append(t.stateRoots, newStateRoot)
	return newStateRoot, nil
}

// Commit commits performed state trasitions from memory to disk.
func (t *ZKTrie) Commit() error {
	currState := t.currentState()
	if err := t.tdb.Commit(currState, true); err != nil {
		return fmt.Errorf("commit db: %w", err)
	}
	t.stateRoots = []common.Hash{currState}
	return nil
}
