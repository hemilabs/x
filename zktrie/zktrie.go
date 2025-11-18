// Copyright (c) 2025 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package zktrie

import (
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
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
	"github.com/holiman/uint256"
)

var (
	MetadataAddress common.Address

	ErrAddressNotFound  = errors.New("address not found")
	ErrBlockNotFound    = errors.New("block information not found")
	ErrOutpointNotFound = errors.New("key not found")
)

func init() {
	const reserved string = "0xffffffffffffffffffffffffffffffffffffffff"
	MetadataAddress = common.BytesToAddress([]byte(reserved))
}

// txId + index
type Outpoint [32 + 4]byte

func NewOutpoint(txid [32]byte, index uint32) (op Outpoint) {
	copy(op[:32], txid[:])
	binary.BigEndian.PutUint32(op[32:], index)
	return op
}

// blockhash + txId + txInIdx
type SpentOutput [32 + 32 + 4]byte

func NewSpentOutput(blockHash, txId [32]byte, index uint32) (so SpentOutput) {
	copy(so[:32], blockHash[:])
	copy(so[32:64], txId[:])
	binary.BigEndian.PutUint32(so[64:], index)
	return so
}

// blockhash + txId + txOutIdx + value
type SpendableOutput [32 + 32 + 4 + 8]byte

func (so SpendableOutput) Value() uint64 {
	return binary.BigEndian.Uint64(so[68:])
}

func NewSpendableOutput(blockHash, txId [32]byte, index uint32, value uint64) (so SpendableOutput) {
	copy(so[:32], blockHash[:])
	copy(so[32:64], txId[:])
	binary.BigEndian.PutUint32(so[64:68], index)
	binary.BigEndian.PutUint64(so[68:], value)
	return so
}

// prev_stateroot + prev_blockhash + height
type BlockInfo [32 + 32 + 8]byte

func (bi BlockInfo) PrevStateRoot() common.Hash {
	return common.BytesToHash(bi[:32])
}

func (bi BlockInfo) PrevBlockHash() chainhash.Hash {
	ch, err := chainhash.NewHash(bi[32:64])
	if err != nil {
		panic("stored value is not blockhash")
	}
	return *ch
}

func (bi BlockInfo) Height() uint64 {
	return binary.BigEndian.Uint64(bi[64:])
}

func NewBlockInfo(prevStateRoot common.Hash, prevBlockHash chainhash.Hash, height uint64) (bi BlockInfo) {
	copy(bi[:32], prevStateRoot[:])
	copy(bi[32:64], prevBlockHash[:])
	binary.BigEndian.PutUint64(bi[64:], height)
	return bi
}

// ZKBlock holds information on a block used for a ZK Trie state transition.
type ZKBlock struct {
	blockHash common.Hash

	// Storage information. Automatically managed using the utility methods.
	storage map[common.Address]map[common.Hash][]byte
}

func NewZKBlock(blockHash, prevBlockHash chainhash.Hash, prevStateRoot common.Hash, height uint64) *ZKBlock {
	bi := NewBlockInfo(prevStateRoot, prevBlockHash, height)
	bh := common.Hash(blockHash)
	return &ZKBlock{
		blockHash: bh,
		storage: map[common.Address]map[common.Hash][]byte{
			MetadataAddress: {
				bh: bi[:],
			},
		},
	}
}

func (b *ZKBlock) NewOut(pkScript []byte, out Outpoint, so SpendableOutput) {
	if b.storage == nil {
		b.storage = make(map[common.Address]map[common.Hash][]byte)
	}

	addr := common.BytesToAddress(pkScript)
	if _, ok := b.storage[addr]; !ok {
		b.storage[addr] = make(map[common.Hash][]byte)
	}

	key := crypto.Keccak256Hash(out[:])
	b.storage[addr][key] = so[:]
}

func (b *ZKBlock) NewIn(pkScript []byte, out Outpoint, so SpentOutput) {
	if b.storage == nil {
		b.storage = make(map[common.Address]map[common.Hash][]byte)
	}

	addr := common.BytesToAddress(pkScript)
	if _, ok := b.storage[addr]; !ok {
		b.storage[addr] = make(map[common.Hash][]byte)
	}

	key := crypto.Keccak256Hash(out[:])
	b.storage[addr][key] = so[:]
}

func (b *ZKBlock) GetOutpoint(pkScript []byte, out Outpoint) []byte {
	addr := common.BytesToAddress(pkScript)
	if b.storage == nil || b.storage[addr] == nil {
		return nil
	}
	key := crypto.Keccak256Hash(out[:])
	return b.storage[addr][key]
}

func (b *ZKBlock) GetMetadata() BlockInfo {
	return BlockInfo(b.storage[MetadataAddress][b.blockHash])
}

// ZKTrie is used to perform operation on a ZK trie and its database.
type ZKTrie struct {
	mtx             sync.RWMutex
	stateRoot       common.Hash
	uncommitedRoots map[common.Hash]struct{}
	tdb             *triedb.Database
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
	disk, err := rawdb.Open(kv, rawdb.OpenOptions{
		Ancient: filepath.Join(home, "ancients"),
	})
	if err != nil {
		return nil, fmt.Errorf("open rawdb: %w", err)
	}
	tdb := triedb.NewDatabase(disk, &triedb.Config{
		PathDB: &pathdb.Config{
			//	NoAsyncFlush:        true,
			EnableStateIndexing: true,
		},
	})

	t := &ZKTrie{
		tdb:             tdb,
		stateRoot:       types.EmptyRootHash,
		uncommitedRoots: make(map[common.Hash]struct{}),
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
	if err := t.tdb.Recover(stateRoot); err != nil {
		return err
	}
	t.stateRoot = stateRoot
	t.uncommitedRoots = make(map[common.Hash]struct{})
	return nil
}

// Put performs an insert into the underlying ZKTrie database.
func (t *ZKTrie) Put(key, value []byte) error {
	return t.tdb.Disk().Put(key, value)
}

// Get performs a lookup on the underlying ZKTrie database.
func (t *ZKTrie) Get(key []byte) ([]byte, error) {
	return t.tdb.Disk().Get(key)
}

// Del performs a delete on the underlying ZKTrie database.
func (t *ZKTrie) Del(key []byte) error {
	return t.tdb.Disk().Delete(key)
}

func (t *ZKTrie) inMemory(state common.Hash) bool {
	t.mtx.RLock()
	defer t.mtx.RUnlock()
	_, ok := t.uncommitedRoots[state]
	return ok || state.Cmp(t.stateRoot) == 0
}

// GetBlockInfo retrieves Block information from the reserved
// metadata state account.
func (t *ZKTrie) GetBlockInfo(blockHash chainhash.Hash, state *common.Hash) (BlockInfo, error) {
	var (
		addrHash  = crypto.Keccak256Hash(MetadataAddress[:])
		stateRoot = t.currentState()
	)
	if state != nil {
		stateRoot = *state
	}
	var b []byte
	if t.inMemory(stateRoot) {
		r, err := t.tdb.StateReader(stateRoot)
		if err != nil {
			return BlockInfo{}, err
		}
		b, err = r.Storage(addrHash, common.BytesToHash(blockHash[:]))
		if err != nil {
			return BlockInfo{}, err
		}
	} else {
		r, err := t.tdb.HistoricReader(stateRoot)
		if err != nil {
			return BlockInfo{}, err
		}
		b, err = r.Storage(MetadataAddress, common.BytesToHash(blockHash[:]))
		if err != nil {
			return BlockInfo{}, err
		}
	}
	if b == nil {
		return BlockInfo{}, ErrBlockNotFound
	}
	if len(b) != len(BlockInfo{}) {
		return BlockInfo{}, fmt.Errorf("unexpected stored value size: %d", len(b))
	}
	return BlockInfo(b), nil
}

func (t *ZKTrie) Sync() error {
	return t.tdb.Disk().SyncAncient()
}

func (t *ZKTrie) SyncProgress() (uint64, error) {
	return t.tdb.IndexProgress()
}

func (t *ZKTrie) GetOutpoint(pkScript []byte, out Outpoint, state *common.Hash) ([]byte, error) {
	var (
		addr      = common.BytesToAddress(pkScript)
		stateRoot = t.currentState()
		keyHash   = crypto.Keccak256Hash(out[:])
	)
	if state != nil {
		stateRoot = *state
	}
	if t.inMemory(stateRoot) {
		r, err := t.tdb.StateReader(stateRoot)
		if err != nil {
			return nil, err
		}
		return r.Storage(crypto.Keccak256Hash(addr[:]), keyHash)
	}
	r, err := t.tdb.HistoricReader(stateRoot)
	if err != nil {
		return nil, err
	}
	return r.Storage(addr, keyHash)
}

func (t *ZKTrie) GetAccount(addr common.Address, state *common.Hash) (*types.SlimAccount, error) {
	var stateRoot = t.currentState()
	if state != nil {
		stateRoot = *state
	}
	if t.inMemory(stateRoot) {
		r, err := t.tdb.StateReader(stateRoot)
		if err != nil {
			return nil, err
		}
		return r.Account(crypto.Keccak256Hash(addr[:]))
	}
	r, err := t.tdb.HistoricReader(stateRoot)
	if err != nil {
		return nil, err
	}
	return r.Account(addr)
}

// currentState gets the most recent State Root.
func (t *ZKTrie) currentState() common.Hash {
	t.mtx.RLock()
	defer t.mtx.RUnlock()
	return t.stateRoot
}

// InsertBlock performs a state transition for a given block.
func (t *ZKTrie) InsertBlock(block *ZKBlock) (common.Hash, error) {
	var (
		stateRoot    = block.GetMetadata().PrevStateRoot()
		mergeSet     = trienode.NewMergedNodeSet()
		stateID      = trie.StateTrieID(stateRoot)
		mutatedAcc   = make(map[common.Hash][]byte, len(block.storage))
		originAcc    = make(map[common.Address][]byte, len(block.storage))
		mutatedStore = make(map[common.Hash]map[common.Hash][]byte, len(block.storage))
		originStore  = make(map[common.Address]map[common.Hash][]byte, len(block.storage))
	)
	stateTrie, err := trie.New(stateID, t.tdb)
	if err != nil {
		return types.EmptyRootHash, fmt.Errorf("failed to load state trie, err: %w", err)
	}

	for addr, storage := range block.storage {
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

		mutatedStore[addrHash] = make(map[common.Hash][]byte, len(block.storage[addr]))
		originStore[addr] = make(map[common.Hash][]byte, len(block.storage[addr]))
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
			switch len(v) {
			case len(SpendableOutput{}):
				p := uint256.NewInt(binary.BigEndian.Uint64(v[68:76]))
				na.Balance.Add(na.Balance, p)
			case len(SpentOutput{}):
				val := originStore[addr][k]
				if val == nil {
					// If out was created and spent in the same block,
					// then don't update the balance.
					continue
				}
				pr := originStore[addr][k][68:76]
				p := uint256.NewInt(binary.BigEndian.Uint64(pr))
				na.Balance.Sub(na.Balance, p)
			case len(BlockInfo{}):
			default:
				panic("unknown storage value passed")
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
	if err := t.tdb.Update(newStateRoot, stateRoot, block.GetMetadata().Height(), mergeSet, &s); err != nil {
		return types.EmptyRootHash, fmt.Errorf("update db: %w", err)
	}

	t.stateRoot = newStateRoot
	t.uncommitedRoots[newStateRoot] = struct{}{}
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
	t.uncommitedRoots = make(map[common.Hash]struct{})
	return nil
}
