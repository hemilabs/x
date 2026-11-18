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
	rootVectorS  = "b0b3e798e388f85158a9eb6c5053b81e76aa77e7a780d21cebb8e127517227dc"
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
	// than the digest of left.
	b := concat(right, left)

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
	merkleRoot2 := Root(Sorted(hashes))
	if !bytes.Equal(merkleRoot, merkleRoot2[:]) {
		t.Fatalf("unexpected merkle root got %x expected %x",
			merkleRoot2[:], merkleRoot)
	}

	// Flip input hashes and make sure we get the same answer.
	hashes = makeHashes(right, left)
	merkleRoot3 := Root(Sorted(hashes))
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
	mt := Sorted(hashes)
	mt2 := Sorted(hashes2)

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
		mt := Sorted(hashes)

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
	mt := Sorted(hashes)

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
	mt := Sorted(hashes)
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
