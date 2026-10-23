// Copyright (c) 2017-2020 The Decred developers
// Copyright (c) 2025 Hemi Labs, Inc.
//
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package merkle

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"testing"
)

const (
	leftVectorS  = "360f84035942243c6a36537ae2f8673485e6c04455a0a85a0db19690f2541480"
	rightVectorS = "27042f4e6eca7d0b2a7ee4026df2ecfa51d3339e6d122aa099118ecd8563bad9"
	rootVectorS  = "a6834b8bdc1d09db298a42e9552271362d83dd0b0d1a00cde031a9df3d48f127"
)

var leftVector, rightVector, rootVector []byte

func init() {
	var err error

	leftVector, err = hex.DecodeString(leftVectorS)
	if err != nil {
		panic(err)
	}

	rightVector, err = hex.DecodeString(rightVectorS)
	if err != nil {
		panic(err)
	}

	rootVector, err = hex.DecodeString(rootVectorS)
	if err != nil {
		panic(err)
	}
}

// mustSorted returns Sorted(hashes) and fails the test on an error.
func mustSorted(t *testing.T, hashes []*[sha256.Size]byte) []*[sha256.Size]byte {
	t.Helper()
	mt, err := Sorted(hashes)
	if err != nil {
		t.Fatal(err)
	}
	return mt
}

// concat concatenates two byte slices.
func concat(l, r []byte) []byte {
	b := make([]byte, len(l)+len(r))
	copy(b, l)
	copy(b[len(l):], r)
	return b
}

// digest takes the dihest of src and returns it in dst.
func digest(src []byte, dst *[]byte) {
	h := sha256.New()
	h.Write(src)
	copy(*dst, h.Sum(nil))
}

// makeHashes takes an array of []byte and converts it to sha256 digest
// pointers.
func makeHashes(b ...[]byte) []*[sha256.Size]byte {
	hashes := make([]*[sha256.Size]byte, 0, len(b))
	for _, v := range b {
		var hash [sha256.Size]byte
		copy(hash[:], v)
		hashes = append(hashes, &hash)
	}
	return hashes
}

func TestMerkle(t *testing.T) {
	// hand roll a merkle tree to validate actual implementation.  This is
	// slow but, deliberately so.
	left := make([]byte, sha256.Size)
	digest([]byte("left"), &left)

	right := make([]byte, sha256.Size)
	digest([]byte("right"), &right)

	// Note that we flip left and right because the digest of right is less
	// than the digest of left.  Leaves hash as sha256(0x00 || leaf) and
	// nodes as sha256(0x01 || left || right).
	lh := make([]byte, sha256.Size)
	digest(concat([]byte{0x00}, left), &lh)
	rh := make([]byte, sha256.Size)
	digest(concat([]byte{0x00}, right), &rh)
	b := concat([]byte{0x01}, concat(rh, lh))

	merkleRoot := make([]byte, sha256.Size)
	digest(b, &merkleRoot)

	// Make sure vectors are right.
	if !bytes.Equal(left, leftVector) {
		t.Fatalf("invalid left got %x want %x", left, leftVector)
	}
	if !bytes.Equal(right, rightVector) {
		t.Fatalf("invalid right got %x want %x", right, rightVector)
	}
	if !bytes.Equal(merkleRoot, rootVector) {
		t.Fatalf("invalid root got %x want %x", merkleRoot, rootVector)
	}

	// Calculate merkle tree and root.
	hashes := makeHashes(left, right)
	merkleRoot2 := Root(mustSorted(t, hashes))
	if !bytes.Equal(merkleRoot, merkleRoot2[:]) {
		t.Fatalf("unexpected merkle root got %x expected %x",
			merkleRoot2[:], merkleRoot)
	}

	// Flip input hashes and make sure we get the same answer.
	hashes = makeHashes(right, left)
	merkleRoot3 := Root(mustSorted(t, hashes))
	if !bytes.Equal(merkleRoot, merkleRoot3[:]) {
		t.Fatalf("unexpected merkle root got %x expected %x",
			merkleRoot3[:], merkleRoot)
	}
}

func TestAuthPathSort(t *testing.T) {
	count := uint64(32769)
	hashes := make([]*[sha256.Size]byte, 0, count)
	hashes2 := make([]*[sha256.Size]byte, 0, count)
	for i := uint64(0); i < count; i++ {
		hash := &[sha256.Size]byte{}
		binary.LittleEndian.PutUint64(hash[:], count-i-1)
		hashes = append(hashes, hash)

		hash2 := &[sha256.Size]byte{byte(i)}
		binary.LittleEndian.PutUint64(hash2[:], i)
		hashes2 = append(hashes2, hash2)
	}
	mt := mustSorted(t, hashes)
	mt2 := mustSorted(t, hashes2)

	for k, v := range mt[:count] {
		if *v != *mt2[k] {
			t.Fatalf("expected %x, got %x", *v, *mt2[k])
		}
	}
}

func TestAuthPath(t *testing.T) {
	// Create 256 merkle trees and find every value in it.
	for count := 0; count < 255; count++ {
		hashes := make([]*[sha256.Size]byte, 0, count)
		for i := 0; i < count; i++ {
			hash := &[sha256.Size]byte{byte(i)}
			hashes = append(hashes, hash)
		}
		mt := mustSorted(t, hashes)

		for find := 0; find < count; find++ {
			root := mt[len(mt)-1]
			mb := AuthPath(hashes, hashes[find])
			if err := VerifyAuthPath(mb, root, hashes[find]); err != nil {
				t.Fatal(err)
			}

			// The branch must not prove another leaf.
			if count > 1 {
				other := hashes[(find+1)%count]
				err := VerifyAuthPath(mb, root, other)
				if !errors.Is(err, ErrLeafNotFound) {
					t.Fatalf("other leaf: got %v, want %v",
						err, ErrLeafNotFound)
				}
			}

			// The branch must not verify against another root.
			bad := *root
			bad[0] ^= 0xff
			err := VerifyAuthPath(mb, &bad, hashes[find])
			if !errors.Is(err, ErrRootMismatch) {
				t.Fatalf("bad root: got %v, want %v",
					err, ErrRootMismatch)
			}
		}
	}
}

func TestAuthPathInvalid(t *testing.T) {
	count := uint64(198123)
	hashes := make([]*[sha256.Size]byte, 0, count)
	for i := uint64(0); i < count; i++ {
		hash := &[sha256.Size]byte{}
		binary.LittleEndian.PutUint64(hash[:], count-i-1)
		hashes = append(hashes, hash)
	}
	mt := mustSorted(t, hashes)

	findHash := &[sha256.Size]byte{}
	binary.LittleEndian.PutUint64(findHash[:], count)
	root := mt[len(mt)-1]

	// No branch exists for a value that is not a leaf.
	if mb := AuthPath(hashes, findHash); mb != nil {
		t.Fatalf("Should have gotten nil, got %v hashes",
			len(mb.Hashes))
	}

	// A branch that holds only the root matches no leaf.
	mb := &Branch{
		NumLeaves: uint32(count),
		Hashes:    [][sha256.Size]byte{*root},
		Flags:     []byte{0},
	}
	if err := VerifyAuthPath(mb, root, findHash); !errors.Is(err, ErrNoMatch) {
		t.Fatalf("got %v, want %v", err, ErrNoMatch)
	}
}

func TestAuthPathEmpty(t *testing.T) {
	hashes := make([]*[sha256.Size]byte, 0)
	mt := mustSorted(t, hashes)
	if mt != nil {
		t.Fatalf("Should have gotten nil")
	}

	findHash := &[sha256.Size]byte{}
	binary.LittleEndian.PutUint64(findHash[:], 1)
	mb := AuthPath(hashes, findHash)

	if mb != nil {
		t.Fatalf("Should have gotten nil")
	}
}

// TestVerifyAuthPathMalformed checks that malformed input returns an
// error and does not panic.
func TestVerifyAuthPathMalformed(t *testing.T) {
	leaves := make([]*[sha256.Size]byte, 0, 10)
	for i := range 10 {
		leaves = append(leaves, &[sha256.Size]byte{byte(i)})
	}
	mt := mustSorted(t, leaves)
	root := mt[len(mt)-1]
	good := AuthPath(leaves, leaves[3])
	if err := VerifyAuthPath(good, root, leaves[3]); err != nil {
		t.Fatal(err)
	}

	h := [sha256.Size]byte{1}
	tests := []struct {
		name string
		mb   *Branch
		want error
	}{
		{"nil branch", nil, ErrEmpty},
		{"no flags", &Branch{NumLeaves: 4, Hashes: good.Hashes[:1]}, ErrMalformed},
		{"short flags", &Branch{NumLeaves: 1 << 20, Hashes: good.Hashes, Flags: []byte{0xff}}, ErrMalformed},
		{"short hashes", &Branch{NumLeaves: 10, Hashes: good.Hashes[:1], Flags: good.Flags}, ErrMalformed},
		{"too many hashes", &Branch{NumLeaves: 1, Hashes: good.Hashes, Flags: good.Flags}, ErrMalformed},
		{"too many flags", &Branch{NumLeaves: 10, Hashes: good.Hashes, Flags: make([]byte, 1<<20)}, ErrMalformed},
	}
	for _, tt := range tests {
		if err := VerifyAuthPath(tt.mb, root, leaves[3]); !errors.Is(err, tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, err, tt.want)
		}
	}
	if err := VerifyAuthPath(good, nil, leaves[3]); !errors.Is(err, ErrMalformed) {
		t.Errorf("nil root: got %v, want %v", err, ErrMalformed)
	}
	if mb := AuthPath(nil, &h); mb != nil {
		t.Errorf("empty leaves: got a branch, want nil")
	}
	if mb := AuthPath(leaves, nil); mb != nil {
		t.Errorf("nil hash: got a branch, want nil")
	}
}

// FuzzVerifyAuthPath checks that VerifyAuthPath does not panic.
func FuzzVerifyAuthPath(f *testing.F) {
	f.Add(uint32(10), []byte{0xff, 0x01}, []byte{1, 2, 3})
	f.Add(uint32(1<<20), []byte{0xff}, []byte{})
	f.Fuzz(func(t *testing.T, n uint32, flags, hashes []byte) {
		mb := &Branch{NumLeaves: n, Flags: flags}
		for i := 0; i < len(hashes); i++ {
			mb.Hashes = append(mb.Hashes, [sha256.Size]byte{hashes[i]})
		}
		var root, leaf [sha256.Size]byte
		_ = VerifyAuthPath(mb, &root, &leaf)
	})
}

// TestVerifyAuthPathFlagEncoding checks that a branch with an extra flag
// byte or a set padding bit fails.
func TestVerifyAuthPathFlagEncoding(t *testing.T) {
	for count := 1; count < 70; count++ {
		leaves := make([]*[sha256.Size]byte, 0, count)
		for i := range count {
			leaves = append(leaves, &[sha256.Size]byte{byte(i)})
		}
		mt := mustSorted(t, leaves)
		root := mt[len(mt)-1]
		for find := range count {
			mb := AuthPath(leaves, leaves[find])

			extra := *mb
			extra.Flags = append(append([]byte{}, mb.Flags...), 0)
			if err := VerifyAuthPath(&extra, root, leaves[find]); !errors.Is(err, ErrMalformed) {
				t.Fatalf("%d/%d extra byte: got %v, want %v", count, find, err, ErrMalformed)
			}

			// Count the flag bits that the branch uses.
			m := &merkleBranch{flags: mb.Flags, inHashes: mb.Hashes, numLeaves: mb.NumLeaves}
			height := treeHeight(mb.NumLeaves)
			if _, err := m.extract(height, 0); err != nil {
				t.Fatal(err)
			}
			rem := m.bitsUsed % 8
			if rem == 0 {
				continue // the last byte has no padding
			}
			padded := *mb
			padded.Flags = append([]byte{}, mb.Flags...)
			padded.Flags[len(padded.Flags)-1] |= 1 << rem
			err := VerifyAuthPath(&padded, root, leaves[find])
			if !errors.Is(err, ErrMalformed) {
				t.Fatalf("%d/%d padding bit: got %v, want %v", count, find, err, ErrMalformed)
			}
		}
	}
}

// TestAuthPathNodeAsLeaf presents an internal node as a leaf of a smaller
// tree.  The branch must fail.
func TestAuthPathNodeAsLeaf(t *testing.T) {
	leaves := make([]*[sha256.Size]byte, 0, 4)
	for i := range 4 {
		leaves = append(leaves, &[sha256.Size]byte{byte(i + 1)})
	}
	mt := mustSorted(t, leaves)
	root := mt[len(mt)-1]

	// The root is the hash of the two internal nodes mt[4] and mt[5].
	// Claim that mt[4] is a leaf of a tree of two leaves.  Flags: the
	// root is a parent of a match, mt[4] is a match, mt[5] is not.
	mb := &Branch{
		NumLeaves: 2,
		Hashes:    [][sha256.Size]byte{*mt[4], *mt[5]},
		Flags:     []byte{0x03},
	}
	if err := VerifyAuthPath(mb, root, mt[4]); !errors.Is(err, ErrRootMismatch) {
		t.Fatalf("got %v, want %v", err, ErrRootMismatch)
	}
}

// TestTreeLeafChecks checks that nil and duplicate leaves fail, that
// Sorted does not change its input, and that a proof from SortLeaves
// verifies.
func TestTreeLeafChecks(t *testing.T) {
	a, b, c := &[sha256.Size]byte{3}, &[sha256.Size]byte{2}, &[sha256.Size]byte{1}

	// Without the check, [a b c] and [a b c c] share a root.
	if _, err := Ordered([]*[sha256.Size]byte{a, b, c, c}); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate: got %v, want %v", err, ErrDuplicate)
	}
	if _, err := Sorted([]*[sha256.Size]byte{a, b, a}); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate: got %v, want %v", err, ErrDuplicate)
	}
	// Without the check, leaves after a nil fall out of the root.
	if _, err := Ordered([]*[sha256.Size]byte{a, b, nil, c}); !errors.Is(err, ErrNilLeaf) {
		t.Fatalf("nil: got %v, want %v", err, ErrNilLeaf)
	}
	if _, err := Sorted([]*[sha256.Size]byte{a, nil}); !errors.Is(err, ErrNilLeaf) {
		t.Fatalf("nil: got %v, want %v", err, ErrNilLeaf)
	}
	if mb := AuthPath([]*[sha256.Size]byte{a, b, a}, a); mb != nil {
		t.Fatal("duplicate: got a branch, want nil")
	}

	leaves := []*[sha256.Size]byte{a, b, c}
	mt := mustSorted(t, leaves)
	if leaves[0] != a || leaves[1] != b || leaves[2] != c {
		t.Fatal("Sorted changed the order of its input")
	}
	mb := AuthPath(SortLeaves(leaves), b)
	if err := VerifyAuthPath(mb, mt[len(mt)-1], b); err != nil {
		t.Fatal(err)
	}
}

// TestTreeWidthLarge checks the width and height for leaf counts above
// 2^31, where uint32 math wraps.
func TestTreeWidthLarge(t *testing.T) {
	n := uint32(1<<31 + 1)
	if w := calcTreeWidth(n, 31); w != 2 {
		t.Fatalf("width: got %d, want 2", w)
	}
	if h := treeHeight(n); h != 32 {
		t.Fatalf("height: got %d, want 32", h)
	}
	if h := treeHeight(1); h != 0 {
		t.Fatalf("height of one leaf: got %d, want 0", h)
	}
}
