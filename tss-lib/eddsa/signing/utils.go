// Copyright © 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package signing

import (
	"math/big"
)

func encodedBytesToBigInt(s *[32]byte) *big.Int {
	sCopy := new([32]byte)
	copy(sCopy[:], s[:])
	reverse(sCopy)
	return new(big.Int).SetBytes(sCopy[:])
}

func bigIntToEncodedBytes(a *big.Int) *[32]byte {
	s := new([32]byte)
	if a == nil {
		return s
	}
	s = copyBytes(a.Bytes())
	reverse(s)
	return s
}

func copyBytes(aB []byte) *[32]byte {
	if aB == nil {
		return nil
	}
	s := new([32]byte)
	if len(aB) > 32 {
		panic("copyBytes: input exceeds 32 bytes, would silently truncate")
	}
	aBLen := len(aB)
	if aBLen < 32 {
		diff := 32 - aBLen
		padded := make([]byte, 32)
		copy(padded[diff:], aB)
		aB = padded
	}
	copy(s[:], aB)
	return s
}

// ecPointToEncodedBytes produces the Ed25519-format compressed encoding:
// y coordinate in little-endian with the sign bit of x in the top bit
// of byte 31.  The "sign" of x in Ed25519 is x mod 2 (the low bit).
func ecPointToEncodedBytes(x *big.Int, y *big.Int) *[32]byte {
	s := bigIntToEncodedBytes(y)
	if x.Bit(0) == 1 {
		s[31] |= (1 << 7)
	} else {
		s[31] &^= (1 << 7)
	}
	return s
}

func reverse(s *[32]byte) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// scReduce reduces a 64-byte little-endian scalar mod the curve order.
func scReduce(in *[64]byte, N *big.Int) *big.Int {
	// Convert 64-byte LE to big.Int.
	buf := make([]byte, 64)
	copy(buf, in[:])
	for i, j := 0, 63; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return new(big.Int).Mod(new(big.Int).SetBytes(buf), N)
}

// scMulAdd computes (a*b + c) mod N.
func scMulAdd(a, b, c, N *big.Int) *big.Int {
	ab := new(big.Int).Mul(a, b)
	ab.Add(ab, c)
	return ab.Mod(ab, N)
}
