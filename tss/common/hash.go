// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package common

import (
	"crypto"
	"encoding/binary"
	"math/big"

	_ "crypto/sha512"
)

const (
	hashInputDelimiter = byte('$')
	// hashNegativeDelimiter replaces hashInputDelimiter after a negative
	// input.  big.Int.Bytes() drops the sign, so without it x and -x hash
	// the same.  Non-negative inputs keep their encoding.
	hashNegativeDelimiter = byte('-')
)

func hashDelimiter(n *big.Int) byte {
	if n != nil && n.Sign() < 0 {
		return hashNegativeDelimiter
	}
	return hashInputDelimiter
}

// SHA-512/256 is protected against length extension attacks and is more performant than SHA-256 on 64-bit architectures.
// https://en.wikipedia.org/wiki/Template:Comparison_of_SHA_functions
func SHA512_256(in ...[]byte) []byte {
	state := crypto.SHA512_256.New()
	inLen := len(in)
	if inLen == 0 {
		return nil
	}
	bzSize := 0
	// prevent hash collisions with this prefix containing the block count
	inLenBz := make([]byte, 64/8)
	// converting between int and uint64 doesn't change the sign bit, but it may be interpreted as a larger value.
	// this prefix is never read/interpreted, so that doesn't matter.
	binary.LittleEndian.PutUint64(inLenBz, uint64(inLen))
	for _, bz := range in {
		bzSize += len(bz)
	}
	dataCap := len(inLenBz) + bzSize + inLen + (inLen * 8)
	data := make([]byte, 0, dataCap)
	data = append(data, inLenBz...)
	for _, bz := range in {
		data = append(data, bz...)
		data = append(data, hashInputDelimiter) // safety delimiter
		dataLen := make([]byte, 8)              // 64-bits
		binary.LittleEndian.PutUint64(dataLen, uint64(len(bz)))
		data = append(data, dataLen...) // Security audit: length of each byte buffer should be added after
		// each security delimiters in order to enforce proper domain separation
	}
	// n < len(data) or an error will never happen.
	// see: https://golang.org/pkg/hash/#Hash and https://github.com/golang/go/wiki/Hashing#the-hashhash-interface
	if _, err := state.Write(data); err != nil {
		Logger.Errorf("SHA512_256 Write() failed: %v", err)
		return nil
	}
	return state.Sum(nil)
}

// hashBigInts encodes in for SHA512_256i and SHA512_256i_TAGGED.  Each
// value is followed by a delimiter and its 64-bit length.  A nil value
// encodes as zero.
func hashBigInts(in []*big.Int) []byte {
	// prevent hash collisions with this prefix containing the block count
	inLenBz := make([]byte, 64/8)
	// converting between int and uint64 doesn't change the sign bit, but it may be interpreted as a larger value.
	// this prefix is never read/interpreted, so that doesn't matter.
	binary.LittleEndian.PutUint64(inLenBz, uint64(len(in)))
	ptrs := make([][]byte, len(in))
	bzSize := 0
	for i, n := range in {
		if n == nil {
			ptrs[i] = zero.Bytes()
		} else {
			ptrs[i] = n.Bytes()
		}
		bzSize += len(ptrs[i])
	}
	data := make([]byte, 0, len(inLenBz)+bzSize+len(in)*9)
	data = append(data, inLenBz...)
	for i := range in {
		data = append(data, ptrs[i]...)
		data = append(data, hashDelimiter(in[i])) // safety delimiter, binds the sign
		dataLen := make([]byte, 8)                // 64-bits
		binary.LittleEndian.PutUint64(dataLen, uint64(len(ptrs[i])))
		data = append(data, dataLen...) // Security audit: length of each byte buffer should be added after
		// each security delimiters in order to enforce proper domain separation
	}
	return data
}

func SHA512_256i(in ...*big.Int) *big.Int {
	if len(in) == 0 {
		return nil
	}
	state := crypto.SHA512_256.New()
	// n < len(data) or an error will never happen.
	// see: https://golang.org/pkg/hash/#Hash and https://github.com/golang/go/wiki/Hashing#the-hashhash-interface
	if _, err := state.Write(hashBigInts(in)); err != nil {
		Logger.Errorf("SHA512_256i Write() failed: %v", err)
		return nil
	}
	return new(big.Int).SetBytes(state.Sum(nil))
}

// SHA512_256i_TAGGED implements a tagged hash (double-prefix construction per BIP-340)
// for SSID domain separation. All proof constructors/verifiers pass Session tags through
// this function to bind proofs to a specific ceremony session, preventing cross-ceremony
// replay attacks.
func SHA512_256i_TAGGED(tag []byte, in ...*big.Int) *big.Int {
	tagBz := SHA512_256(tag)
	state := crypto.SHA512_256.New()
	state.Write(tagBz)
	state.Write(tagBz)
	if len(in) == 0 {
		return nil
	}
	// n < len(data) or an error will never happen.
	// see: https://golang.org/pkg/hash/#Hash and https://github.com/golang/go/wiki/Hashing#the-hashhash-interface
	if _, err := state.Write(hashBigInts(in)); err != nil {
		Logger.Error(err)
		return nil
	}
	return new(big.Int).SetBytes(state.Sum(nil))
}

func SHA512_256iOne(in *big.Int) *big.Int {
	state := crypto.SHA512_256.New()
	if in == nil {
		return nil
	}
	data := in.Bytes()
	// n < len(data) or an error will never happen.
	// see: https://golang.org/pkg/hash/#Hash and https://github.com/golang/go/wiki/Hashing#the-hashhash-interface
	if _, err := state.Write(data); err != nil {
		Logger.Errorf("SHA512_256iOne Write() failed: %v", err)
		return nil
	}
	return new(big.Int).SetBytes(state.Sum(nil))
}
