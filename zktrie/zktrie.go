// Copyright (c) 2025 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package zktrie

import (
	"errors"
	"fmt"
	"sync"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/ethdb/leveldb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/hemilabs/x/eth-trie/trie"
	"github.com/hemilabs/x/eth-trie/trie/trienode"
	"github.com/hemilabs/x/eth-trie/triedb"
	"github.com/hemilabs/x/eth-trie/triedb/pathdb"
)

// XXX probably need a way to be able to differentiate between account
// types so when we insert a block, they update fields in different ways.
// Maybe we can use the CodeHash field of StateAccount?
var (
	MetadataAddress common.Address
	ErrNotFound     = errors.New("key not found")
)

func init() {
	const reserved string = "0xffffffffffffffffffffffffffffffffffffffff"
	MetadataAddress = common.BytesToAddress([]byte(reserved))
	spew.Dump(MetadataAddress)
}

// ZKBlock holds informati∂on on a block used for a ZK Trie state transition.
type ZKBlock struct {
	Height uint64

	// Storage information. Automatically updates the AccountState
	// that each storage is associated with. The inner map's keys
	// are Keccak256 hashes of []byte values.
	//
	// When the value passed is nil, the associated key is deleted
	// from the Trie. Nil should only be passed if the key exists
	// in the previous state.
	Storage map[common.Address]map[common.Hash][]byte

	// Accounts information. Overrides any values passed for the
	// same address in storage.
	//
	// When the StateAccount passed is nil, the associated address
	// is deleted from the Trie. Nil should only be passed if the
	// address exists in the previous state.
	Accounts map[common.Address]types.StateAccount
}

func NewZKBlock(height uint64) *ZKBlock {
	return &ZKBlock{
		Height: height,
	}
}

func (b *ZKBlock) AddStorage(addr common.Address, key common.Hash, value []byte) {
	if b.Storage == nil {
		b.Storage = make(map[common.Address]map[common.Hash][]byte)
	}
	if _, ok := b.Storage[addr]; !ok {
		b.Storage[addr] = make(map[common.Hash][]byte)
	}
	b.Storage[addr][key] = value
}

func (b *ZKBlock) GetStorage(addr common.Address, key common.Hash) ([]byte, error) {
	if b.Storage == nil || b.Storage[addr] == nil {
		return nil, ErrNotFound
	}
	val, ok := b.Storage[addr][key]
	if !ok {
		return nil, ErrNotFound
	}
	return val, nil
}

func (b *ZKBlock) AddAccount(addr common.Address, state types.StateAccount) {
	if b.Accounts == nil {
		b.Accounts = make(map[common.Address]types.StateAccount)
	}
	b.Accounts[addr] = state
}

func (b *ZKBlock) GetAccount(addr common.Address) (types.StateAccount, error) {
	if b.Accounts == nil {
		return *types.NewEmptyStateAccount(), ErrNotFound
	}
	val, ok := b.Accounts[addr]
	if !ok {
		return val, ErrNotFound
	}
	return val, nil
}

func (b *ZKBlock) AddMetadata(key common.Hash, value []byte) {
	b.AddStorage(MetadataAddress, key, value)
}

func (b *ZKBlock) GetMetadata(key common.Hash) ([]byte, error) {
	return b.GetStorage(MetadataAddress, key)
}

// ZKTrie is used to perform operation on a ZK trie and its database.
type ZKTrie struct {
	mtx        sync.RWMutex
	stateRoots []common.Hash
	tdb        *triedb.Database
}

// TODO: set database cache and handles
func NewZKTrie(home string) (*ZKTrie, error) {
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
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return t.tdb.Close()
}

// Recover rollbacks the ZKTrie database to a specified historical point.
func (t *ZKTrie) Recover(stateRoot common.Hash) error {
	t.mtx.Lock()
	defer t.mtx.Unlock()
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

// MetadataGet retrieves values from the reserved metadata state account.
func (t *ZKTrie) MetadataGet(key []byte) ([]byte, error) {
	return t.GetStorage(MetadataAddress, key)
}

func (t *ZKTrie) GetStorage(addr common.Address, key []byte) ([]byte, error) {
	var (
		stateRoot = t.currentState()
		addrHash  = crypto.Keccak256Hash(addr.Bytes())
		keyHash   = crypto.Keccak256Hash(key)
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
	return storeTrie.Get(keyHash[:])
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
	t.mtx.RLock()
	defer t.mtx.RUnlock()

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
				panic(fmt.Errorf("decode stored state value: %w", err))
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

		full, err := rlp.EncodeToBytes(&na)
		if err != nil {
			return types.EmptyRootHash, err
		}
		mutatedAcc[addrHash] = full
		originAcc[addr] = stateVal
	}

	for addr, sa := range block.Accounts {
		addrHash := crypto.Keccak256Hash(addr.Bytes())
		stateVal, err := stateTrie.Get(addrHash[:])
		if err != nil {
			return types.EmptyRootHash, fmt.Errorf("get account state: %w", err)
		}
		full, err := rlp.EncodeToBytes(&sa)
		if err != nil {
			return types.EmptyRootHash, err
		}
		mutatedAcc[addrHash] = full
		originAcc[addr] = stateVal
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

	t.mtx.Lock()
	defer t.mtx.Unlock()

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
	t.mtx.Lock()
	defer t.mtx.Unlock()

	if err := t.tdb.Commit(currState, true); err != nil {
		return fmt.Errorf("commit db: %w", err)
	}
	t.stateRoots = []common.Hash{currState}
	return nil
}
