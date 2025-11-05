// Copyright (c) 2025 Hemi Labs, Inc.
// Use of this source code is governed by a GNU LGPL License,
// which can be found in the LICENSE file.
package testutil

import (
	"crypto/rand"
	"io"

	"github.com/ethereum/go-ethereum/common"
)

// Random returns a variable number of random bytes.
func Random(n int) []byte {
	buffer := make([]byte, n)
	_, err := io.ReadFull(rand.Reader, buffer)
	if err != nil {
		panic(err)
	}
	return buffer
}

// Hash generates a random hash.
func Hash() common.Hash {
	return common.BytesToHash(Random(common.HashLength))
}

// Address generates a random address.
func Address() common.Address {
	return common.BytesToAddress(Random(common.AddressLength))
}
