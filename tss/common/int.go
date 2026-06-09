// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package common

import (
	"encoding/binary"
	"math/big"
)

// modInt is a *big.Int that performs all of its arithmetic with modular reduction.
type modInt big.Int

var (
	zero = big.NewInt(0)
	one  = big.NewInt(1)
	two  = big.NewInt(2)
)

func ModInt(mod *big.Int) *modInt {
	return (*modInt)(mod)
}

func (mi *modInt) Add(x, y *big.Int) *big.Int {
	i := new(big.Int)
	i.Add(x, y)
	return i.Mod(i, mi.i())
}

func (mi *modInt) Sub(x, y *big.Int) *big.Int {
	i := new(big.Int)
	i.Sub(x, y)
	return i.Mod(i, mi.i())
}

func (mi *modInt) Mul(x, y *big.Int) *big.Int {
	i := new(big.Int)
	i.Mul(x, y)
	return i.Mod(i, mi.i())
}

func (mi *modInt) Exp(x, y *big.Int) *big.Int {
	return new(big.Int).Exp(x, y, mi.i())
}

func (mi *modInt) ModInverse(g *big.Int) *big.Int {
	return new(big.Int).ModInverse(g, mi.i())
}

func (mi *modInt) i() *big.Int {
	return (*big.Int)(mi)
}

// [FORK] Nil guard: upstream panics on nil b or bound. These inputs can be
// attacker-controlled (e.g., received proof fields in ZK verification), so we
// return false instead of crashing.
func IsInInterval(b *big.Int, bound *big.Int) bool {
	if b == nil || bound == nil {
		return false
	}
	return b.Cmp(bound) == -1 && b.Cmp(zero) >= 0
}

// AppendBigIntToBytesSlice appends a length-prefixed big.Int encoding to a
// byte slice. The encoding is [4-byte big-endian length][big.Int.Bytes()].
// This ensures that big.Int(0) (which has empty Bytes()) is distinguishable
// from "nothing appended" — party index 0 gets [00 00 00 00] appended.
//
// [FORK] Upstream appends raw big.Int.Bytes() without a length prefix, making
// it impossible to distinguish zero-valued fields from absent fields in the
// SSID/CeremonyID byte stream. The length prefix provides unambiguous parsing.
func AppendBigIntToBytesSlice(commonBytes []byte, appended *big.Int) []byte {
	var bz []byte
	// Defense-in-depth: all current callers pass non-nil big.Int, but this is a
	// public utility function. Guards against future misuse.
	if appended != nil {
		bz = appended.Bytes()
	}
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(bz))) //nolint:gosec // big.Int bytes < 1KB
	resultBytes := make([]byte, len(commonBytes), len(commonBytes)+4+len(bz))
	copy(resultBytes, commonBytes)
	resultBytes = append(resultBytes, lenBuf[:]...)
	resultBytes = append(resultBytes, bz...)
	return resultBytes
}
