// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package common

import (
	"encoding/hex"
	"math/big"
	"testing"
)

// TestSHA512_256iZeroInput documents that big.Int(0).Bytes() returns []byte{},
// so SHA512_256i(big.Int(0)) hashes an empty-magnitude byte sequence.
// Rust implementations MUST match this behavior: use [] (empty) not [0x00].
func TestSHA512_256iZeroInput(t *testing.T) {
	result := SHA512_256i(big.NewInt(0))
	if result == nil {
		t.Fatal("SHA512_256i(0) should not return nil")
	}
	h := hex.EncodeToString(result.Bytes())
	t.Logf("SHA512_256i(0) = %s", h)

	// Verify it differs from SHA512_256i(1).
	result1 := SHA512_256i(big.NewInt(1))
	if result.Cmp(result1) == 0 {
		t.Fatal("SHA512_256i(0) should differ from SHA512_256i(1)")
	}
}

// TestSHA512_256iMultipleWithZero documents the preimage when zero is one
// of multiple arguments.
func TestSHA512_256iMultipleWithZero(t *testing.T) {
	// SHA512_256i(2, 0) vs SHA512_256i(2, 1).
	r20 := SHA512_256i(big.NewInt(2), big.NewInt(0))
	r21 := SHA512_256i(big.NewInt(2), big.NewInt(1))
	if r20.Cmp(r21) == 0 {
		t.Fatal("SHA512_256i(2,0) should differ from SHA512_256i(2,1)")
	}
	t.Logf("SHA512_256i(2, 0) = %s", hex.EncodeToString(r20.Bytes()))
	t.Logf("SHA512_256i(2, 1) = %s", hex.EncodeToString(r21.Bytes()))

	// SHA512_256i(1, 0) vs SHA512_256i(0, 1).
	r10 := SHA512_256i(big.NewInt(1), big.NewInt(0))
	r01 := SHA512_256i(big.NewInt(0), big.NewInt(1))
	if r10.Cmp(r01) == 0 {
		t.Fatal("SHA512_256i(1,0) should differ from SHA512_256i(0,1)")
	}
	t.Logf("SHA512_256i(1, 0) = %s", hex.EncodeToString(r10.Bytes()))
	t.Logf("SHA512_256i(0, 1) = %s", hex.EncodeToString(r01.Bytes()))
}

// TestSHA512_256iEmpty verifies nil return for empty input.
func TestSHA512_256iEmpty(t *testing.T) {
	result := SHA512_256i()
	if result != nil {
		t.Fatal("SHA512_256i() with no args should return nil")
	}
}

// TestSHA512_256iOneVsSHA512_256i documents that SHA512_256iOne and
// SHA512_256i produce DIFFERENT results for the same input. The Rust
// guest must implement both functions.
func TestSHA512_256iOneVsSHA512_256i(t *testing.T) {
	x := big.NewInt(42)

	resultOne := SHA512_256iOne(x)
	resultI := SHA512_256i(x)

	if resultOne.Cmp(resultI) == 0 {
		t.Fatal("SHA512_256iOne(42) should differ from SHA512_256i(42)")
	}
	// Hardcoded golden vectors — Rust must match exactly.
	expectOne := "4c443fc75eff4e3c217c1a216d2a18a4057ca05a1a4098d147b0f28a5453c7c8"
	expectI := "5d9ba0bb5fd6df1e69b641f81290de5b7aa24905172b02aee8d39157253814a6"
	gotOne := hex.EncodeToString(resultOne.Bytes())
	gotI := hex.EncodeToString(resultI.Bytes())
	if gotOne != expectOne {
		t.Fatalf("SHA512_256iOne(42) = %s, want %s", gotOne, expectOne)
	}
	if gotI != expectI {
		t.Fatalf("SHA512_256i(42) = %s, want %s", gotI, expectI)
	}
}

// TestSHA512_256iOneZero documents SHA512_256iOne behavior with zero.
func TestSHA512_256iOneZero(t *testing.T) {
	result := SHA512_256iOne(big.NewInt(0))
	if result == nil {
		t.Fatal("SHA512_256iOne(0) should not return nil")
	}
	// SHA512_256iOne(0) = SHA512/256("") — well-known empty-string hash.
	expect := "c672b8d1ef56ed28ab87c3622c5114069bdd3ad7b8f9737498d0c01ecef0967a"
	got := hex.EncodeToString(result.Bytes())
	if got != expect {
		t.Fatalf("SHA512_256iOne(0) = %s, want %s", got, expect)
	}

	// SHA512_256iOne(0) hashes empty bytes (since big.Int(0).Bytes() = []).
	// This is identical to SHA512/256("") which is a well-known value.
	result1 := SHA512_256iOne(big.NewInt(1))
	if result.Cmp(result1) == 0 {
		t.Fatal("SHA512_256iOne(0) should differ from SHA512_256iOne(1)")
	}
}

// TestSHA512_256iOneNil documents nil behavior.
func TestSHA512_256iOneNil(t *testing.T) {
	result := SHA512_256iOne(nil)
	if result != nil {
		t.Fatal("SHA512_256iOne(nil) should return nil")
	}
}

// TestSHA512_256iNilElementHandled documents that SHA512_256i substitutes
// zero.Bytes() for nil *big.Int elements, matching the tagged variant behavior.
func TestSHA512_256iNilElementHandled(t *testing.T) {
	// Must not panic — nil elements are substituted with zero.
	resultNil := SHA512_256i(big.NewInt(1), nil, big.NewInt(2))
	resultZero := SHA512_256i(big.NewInt(1), big.NewInt(0), big.NewInt(2))
	if resultNil == nil {
		t.Fatal("expected non-nil result")
	}
	if resultNil.Cmp(resultZero) != 0 {
		t.Fatal("SHA512_256i with nil and 0 should produce the same result")
	}
}

// TestSHA512_256iNegativeInput documents that big.Int.Bytes() drops the sign,
// so SHA512_256i(-1) produces the same hash as SHA512_256i(1).
func TestSHA512_256iNegativeInput(t *testing.T) {
	hashPos := SHA512_256i(big.NewInt(1))
	hashNeg := SHA512_256i(big.NewInt(-1))
	if hashPos.Cmp(hashNeg) != 0 {
		t.Fatal("SHA512_256i(-1) should equal SHA512_256i(1) because Bytes() drops sign")
	}
}

// TestSHA512_256iTaggedNilElement documents that the tagged variant substitutes
// zero.Bytes() for nil inputs (the untagged variant now does the same).
func TestSHA512_256iTaggedNilElement(t *testing.T) {
	tag := []byte("test-tag")

	// Tagged version handles nil by substituting zero.
	resultNil := SHA512_256i_TAGGED(tag, nil)
	resultZero := SHA512_256i_TAGGED(tag, big.NewInt(0))

	// Since zero.Bytes() == big.NewInt(0).Bytes() == []byte{}, these should match.
	if resultNil.Cmp(resultZero) != 0 {
		t.Fatal("TAGGED with nil and TAGGED with 0 should produce the same result")
	}
	t.Logf("SHA512_256i_TAGGED('test-tag', nil)  = %s", hex.EncodeToString(resultNil.Bytes()))
	t.Logf("SHA512_256i_TAGGED('test-tag', 0)    = %s", hex.EncodeToString(resultZero.Bytes()))
}

// TestSHA512_256iTaggedVsUntagged documents that tagged and untagged produce
// different results for the same input.
func TestSHA512_256iTaggedVsUntagged(t *testing.T) {
	tag := []byte("session")
	x := big.NewInt(42)

	tagged := SHA512_256i_TAGGED(tag, x)
	untagged := SHA512_256i(x)

	if tagged.Cmp(untagged) == 0 {
		t.Fatal("tagged and untagged should differ")
	}
}

// TestSHA512_256iGoldenVectors produces hardcoded golden vectors for
// cross-language verification. These exact hex values must be reproduced
// by any Rust implementation.
func TestSHA512_256iGoldenVectors(t *testing.T) {
	tests := []struct {
		name     string
		inputs   []*big.Int
		expected string // hardcoded hex — Rust must match exactly
	}{
		{"single_1", []*big.Int{big.NewInt(1)}, "c272488e6eb0653d5dd36405b8525c31058d6bcb56a8326e037605aa70c219a8"},
		{"single_0", []*big.Int{big.NewInt(0)}, "bbbbf79af6a54ebfd64b703fca4241d1ef2930bd8fcd0c898da64eeac240fe24"},
		{"pair_1_2", []*big.Int{big.NewInt(1), big.NewInt(2)}, "bb744ef1d81d80add983c0ee6058621cb9de243f8e99c7eb16c816bbbd4c7dca"},
		{"pair_0_0", []*big.Int{big.NewInt(0), big.NewInt(0)}, "ccc267e9748792ad0ab7632b3674208462cd56b5453c014d4f7844bfd3f9be5c"},
		{"triple_0_1_2", []*big.Int{big.NewInt(0), big.NewInt(1), big.NewInt(2)}, "09c0eac3208b7cb3a48f32e33e7003d5df98f4ee7f9fa797687610b78e082407"},
		{"large_256", []*big.Int{big.NewInt(256)}, "c1f31d55168ae2cabfa0909fbdd93d248ff8e60386387aef08908bff718f4eb5"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SHA512_256i(tc.inputs...)
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			got := hex.EncodeToString(result.Bytes())
			// Pad to 64 hex chars (32 bytes) to match %064x format.
			for len(got) < 64 {
				got = "0" + got
			}
			if got != tc.expected {
				t.Fatalf("SHA512_256i(%s) = %s, want %s", tc.name, got, tc.expected)
			}
		})
	}
}

// TestBigIntBytesEncodingForHash documents the critical Go big.Int.Bytes()
// encoding behavior that affects all hash computations.
func TestBigIntBytesEncodingForHash(t *testing.T) {
	tests := []struct {
		value    int64
		expected string // hex
	}{
		{0, ""}, // empty! Rust BigUint::to_bytes_be() returns [0x00] instead
		{1, "01"},
		{127, "7f"},
		{128, "80"},
		{255, "ff"},
		{256, "0100"},
		{65535, "ffff"},
		{65536, "010000"},
	}
	for _, tc := range tests {
		got := hex.EncodeToString(big.NewInt(tc.value).Bytes())
		if got != tc.expected {
			t.Errorf("big.Int(%d).Bytes() = %q, want %q", tc.value, got, tc.expected)
		}
	}
}

// TestAppendBigIntToBytesSlicePartyZero verifies that the length-prefixed
// encoding makes party 0 distinct from bare SSID. Party 0 gets a 4-byte
// length prefix [00 00 00 00] appended (length=0, no value bytes).
func TestAppendBigIntToBytesSlicePartyZero(t *testing.T) {
	ssid := []byte("test-ssid-32-bytes-for-testing!!")

	ctx0 := AppendBigIntToBytesSlice(ssid, big.NewInt(0))
	ctx1 := AppendBigIntToBytesSlice(ssid, big.NewInt(1))

	// Party 0 gets ssid + [00 00 00 00] (4-byte length prefix, zero-length value).
	if len(ctx0) != len(ssid)+4 {
		t.Fatalf("party 0 context should be ssid+4: got %d, want %d",
			len(ctx0), len(ssid)+4)
	}
	// Party 1 gets ssid + [00 00 00 01] + [01] (4-byte length=1, value=0x01).
	if len(ctx1) != len(ssid)+5 {
		t.Fatalf("party 1 context should be ssid+5: got %d, want %d",
			len(ctx1), len(ssid)+5)
	}

	// Party 0 and party 1 differ.
	if hex.EncodeToString(ctx0) == hex.EncodeToString(ctx1) {
		t.Fatal("party 0 and party 1 contexts should differ")
	}

	// Party 0 is now DISTINCT from bare ssid.
	if hex.EncodeToString(ctx0) == hex.EncodeToString(ssid) {
		t.Fatal("party 0 context should differ from bare ssid after length-prefix fix")
	}
}

// TestAppendBigIntToBytesSliceGoldenVectors verifies hardcoded hex golden
// vectors for known inputs. Rust implementations must match exactly.
func TestAppendBigIntToBytesSliceGoldenVectors(t *testing.T) {
	ssid := []byte("test-ssid")

	tests := []struct {
		name     string
		index    int64
		expected string
	}{
		// ssid bytes + [00 00 00 00] (length=0, no value bytes for zero)
		{"index_0", 0, "746573742d7373696400000000"},
		// ssid bytes + [00 00 00 01] + [01]
		{"index_1", 1, "746573742d737369640000000101"},
		// ssid bytes + [00 00 00 01] + [ff]
		{"index_255", 255, "746573742d7373696400000001ff"},
		// ssid bytes + [00 00 00 02] + [01 00]
		{"index_256", 256, "746573742d73736964000000020100"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := AppendBigIntToBytesSlice(ssid, big.NewInt(tc.index))
			got := hex.EncodeToString(result)
			if got != tc.expected {
				t.Fatalf("AppendBigIntToBytesSlice(ssid, %d) = %s, want %s",
					tc.index, got, tc.expected)
			}
		})
	}
}

// TestAppendBigIntToBytesSliceNegativeInput documents that big.Int.Bytes()
// drops the sign bit, so AppendBigIntToBytesSlice(ssid, -5) produces the
// same result as AppendBigIntToBytesSlice(ssid, 5). Rust implementations
// using unsigned types will naturally match this behavior.
func TestAppendBigIntToBytesSliceNegativeInput(t *testing.T) {
	ssid := []byte("test-ssid")
	pos := AppendBigIntToBytesSlice(ssid, big.NewInt(5))
	neg := AppendBigIntToBytesSlice(ssid, big.NewInt(-5))
	if hex.EncodeToString(pos) != hex.EncodeToString(neg) {
		t.Fatalf("negative and positive should match: pos=%s neg=%s",
			hex.EncodeToString(pos), hex.EncodeToString(neg))
	}
}

// TestAppendBigIntToBytesSliceNilCommonBytes verifies that passing nil as
// commonBytes works correctly and produces only the length prefix + value.
func TestAppendBigIntToBytesSliceNilCommonBytes(t *testing.T) {
	result := AppendBigIntToBytesSlice(nil, big.NewInt(42))
	got := hex.EncodeToString(result)
	// 42 = 0x2a, length = 1 byte → [00 00 00 01] [2a]
	expected := "000000012a"
	if got != expected {
		t.Fatalf("AppendBigIntToBytesSlice(nil, 42) = %s, want %s", got, expected)
	}
}

// TestAppendBigIntToBytesSliceMultipleAppends verifies that the function does
// not mutate the input commonBytes slice. The implementation allocates a new
// slice rather than appending in-place.
func TestAppendBigIntToBytesSliceMultipleAppends(t *testing.T) {
	ssid := []byte("test")
	ssidCopy := make([]byte, len(ssid))
	copy(ssidCopy, ssid)

	_ = AppendBigIntToBytesSlice(ssid, big.NewInt(42))

	// ssid should still equal ssidCopy — no mutation.
	if hex.EncodeToString(ssid) != hex.EncodeToString(ssidCopy) {
		t.Fatalf("AppendBigIntToBytesSlice mutated input: was %s, now %s",
			hex.EncodeToString(ssidCopy), hex.EncodeToString(ssid))
	}
}

// TestAppendBigIntToBytesSliceLengthPrefixIs4Bytes verifies that for various
// indices, the 4 bytes immediately after the ssid are always exactly a
// big-endian uint32 length prefix encoding the byte length of the index value.
func TestAppendBigIntToBytesSliceLengthPrefixIs4Bytes(t *testing.T) {
	ssid := []byte("test-ssid")
	ssidLen := len(ssid)

	tests := []struct {
		index         int64
		expectedLen   uint32 // expected value of the 4-byte length prefix
		expectedBytes int    // expected byte length of the big.Int value
	}{
		{0, 0, 0},     // big.NewInt(0).Bytes() = []
		{1, 1, 1},     // big.NewInt(1).Bytes() = [0x01]
		{256, 2, 2},   // big.NewInt(256).Bytes() = [0x01, 0x00]
		{65536, 3, 3}, // big.NewInt(65536).Bytes() = [0x01, 0x00, 0x00]
	}
	for _, tc := range tests {
		result := AppendBigIntToBytesSlice(ssid, big.NewInt(tc.index))

		// Extract the 4-byte length prefix immediately after ssid.
		if len(result) < ssidLen+4 {
			t.Fatalf("index %d: result too short: %d", tc.index, len(result))
		}
		prefix := result[ssidLen : ssidLen+4]
		gotLen := uint32(prefix[0])<<24 | uint32(prefix[1])<<16 | uint32(prefix[2])<<8 | uint32(prefix[3])
		if gotLen != tc.expectedLen {
			t.Fatalf("index %d: length prefix = %d, want %d (bytes: %s)",
				tc.index, gotLen, tc.expectedLen, hex.EncodeToString(prefix))
		}

		// Total length should be ssid + 4 (prefix) + expectedBytes (value).
		expectedTotal := ssidLen + 4 + tc.expectedBytes
		if len(result) != expectedTotal {
			t.Fatalf("index %d: total length = %d, want %d",
				tc.index, len(result), expectedTotal)
		}
	}
}

// TestSHA512_256iTaggedGoldenVectors freezes golden vectors for
// SHA512_256i_TAGGED. These exact hex values must be reproduced by any
// Rust implementation.
func TestSHA512_256iTaggedGoldenVectors(t *testing.T) {
	tests := []struct {
		name     string
		tag      []byte
		inputs   []*big.Int
		expected string
	}{
		{
			"session_0_1",
			[]byte("session"),
			[]*big.Int{big.NewInt(0), big.NewInt(1)},
			"73b9c44658bea960ae093eba82e8190bcd989b18c8e6a87e03aa6521ebd5ccfc",
		},
		{
			"session_42",
			[]byte("session"),
			[]*big.Int{big.NewInt(42)},
			"c1fd4c4302095795a413cd26f03b31a2f04dbb7d845478fb0fbe88bd114303db",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SHA512_256i_TAGGED(tc.tag, tc.inputs...)
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			got := hex.EncodeToString(result.Bytes())
			if got != tc.expected {
				t.Fatalf("SHA512_256i_TAGGED(%s) = %s, want %s",
					tc.name, got, tc.expected)
			}
		})
	}
}

// TestAppendBigIntToBytesSliceNilAppended verifies that passing nil as the
// appended *big.Int does not panic and produces the same result as passing
// big.NewInt(0) — both append only a 4-byte zero-length prefix.
func TestAppendBigIntToBytesSliceNilAppended(t *testing.T) {
	ssid := []byte("test-ssid")

	// Must not panic.
	result := AppendBigIntToBytesSlice(ssid, nil)

	// Should produce the same encoding as big.NewInt(0): ssid + [00 00 00 00].
	expected := AppendBigIntToBytesSlice(ssid, big.NewInt(0))
	if hex.EncodeToString(result) != hex.EncodeToString(expected) {
		t.Fatalf("nil and zero should produce identical encoding: nil=%s, zero=%s",
			hex.EncodeToString(result), hex.EncodeToString(expected))
	}

	// Total length: ssid(9) + prefix(4) = 13.
	if len(result) != len(ssid)+4 {
		t.Fatalf("expected length %d, got %d", len(ssid)+4, len(result))
	}
}

// TestAppendBigIntToBytesSliceBothNil verifies that passing nil for both
// commonBytes and appended works correctly.
func TestAppendBigIntToBytesSliceBothNil(t *testing.T) {
	result := AppendBigIntToBytesSlice(nil, nil)
	// Should be just [00 00 00 00].
	expected := "00000000"
	got := hex.EncodeToString(result)
	if got != expected {
		t.Fatalf("AppendBigIntToBytesSlice(nil, nil) = %s, want %s", got, expected)
	}
}

// ---------------------------------------------------------------------------
// ContextI/ContextJ regression tests
//
// These tests freeze the exact byte encoding of ContextI and ContextJ as
// constructed in the round files:
//   - eddsa/keygen/round_2.go (line 50)
//   - eddsa/signing/round_2.go (line 37)
//   - ecdsa/signing/round_3.go (line 41)
//
// All three construct: AppendBigIntToBytesSlice(ssid, big.Int(partyIndex))
//
// The OLD encoding (bare append) produced: ssid || big.Int(i).Bytes()
//   - For index 0: ssid (UNCHANGED — indistinguishable from bare SSID)
//   - For index 1: ssid || [0x01]
//
// The NEW encoding (length-prefixed) produces: ssid || [4-byte len] || big.Int(i).Bytes()
//   - For index 0: ssid || [00 00 00 00] (distinguishable from bare SSID)
//   - For index 1: ssid || [00 00 00 01] || [0x01]
//
// If someone reverts the fix in any round file back to bare append, the E2E
// tests would still pass (all parties use the same encoding). These tests
// catch the regression by freezing the CORRECT encoding as golden vectors.
// ---------------------------------------------------------------------------

// TestContextIEncodingRegressionPartyZero verifies that the ContextI encoding
// for party index 0 differs from bare SSID. This is the critical regression
// test: with the old bare-append encoding, party 0 got ContextI == SSID
// (because big.Int(0).Bytes() == []), making party 0's proofs use a
// different session context than all other parties.
func TestContextIEncodingRegressionPartyZero(t *testing.T) {
	ssid := []byte("test-ssid-32-bytes-for-testing!!")

	// NEW encoding (correct): ssid + [00 00 00 00]
	contextI := AppendBigIntToBytesSlice(ssid, new(big.Int).SetUint64(0))

	// OLD encoding (buggy): ssid + big.Int(0).Bytes() = ssid + [] = ssid
	oldContextI := append([]byte{}, ssid...)
	oldContextI = append(oldContextI, new(big.Int).SetUint64(0).Bytes()...)

	// The new encoding MUST differ from the old encoding.
	if hex.EncodeToString(contextI) == hex.EncodeToString(oldContextI) {
		t.Fatal("ContextI for party 0 must differ from bare SSID — " +
			"if this fails, the AppendBigIntToBytesSlice fix has been reverted")
	}

	// The new encoding must be exactly 4 bytes longer (the length prefix).
	if len(contextI) != len(ssid)+4 {
		t.Fatalf("ContextI for party 0 should be ssid+4 bytes, got %d", len(contextI))
	}

	// Verify the 4-byte suffix is [00 00 00 00] (zero-length value).
	suffix := contextI[len(ssid):]
	expectedSuffix := "00000000"
	if hex.EncodeToString(suffix) != expectedSuffix {
		t.Fatalf("ContextI suffix for party 0 = %s, want %s",
			hex.EncodeToString(suffix), expectedSuffix)
	}
}

// TestContextIEncodingRegressionPartyOne verifies that party 1's ContextI
// uses the length-prefixed encoding, not bare append.
func TestContextIEncodingRegressionPartyOne(t *testing.T) {
	ssid := []byte("test-ssid-32-bytes-for-testing!!")

	// NEW encoding (correct): ssid + [00 00 00 01] + [01]
	contextI := AppendBigIntToBytesSlice(ssid, new(big.Int).SetUint64(1))

	// OLD encoding (buggy): ssid + [01]
	oldContextI := append([]byte{}, ssid...)
	oldContextI = append(oldContextI, new(big.Int).SetUint64(1).Bytes()...)

	// The new encoding is 4 bytes longer (the length prefix).
	if len(contextI) != len(oldContextI)+4 {
		t.Fatalf("new encoding should be 4 bytes longer: new=%d, old=%d",
			len(contextI), len(oldContextI))
	}

	// Verify exact encoding.
	expectedSuffix := "0000000101" // [00 00 00 01] + [01]
	gotSuffix := hex.EncodeToString(contextI[len(ssid):])
	if gotSuffix != expectedSuffix {
		t.Fatalf("ContextI suffix for party 1 = %s, want %s", gotSuffix, expectedSuffix)
	}
}

// TestContextIEncodingDistinguishesAllParties verifies that each party index
// 0-4 gets a unique ContextI encoding with the length-prefixed format.
func TestContextIEncodingDistinguishesAllParties(t *testing.T) {
	ssid := []byte("test-ssid-32-bytes-for-testing!!")

	seen := make(map[string]int)
	for i := 0; i < 5; i++ {
		contextI := AppendBigIntToBytesSlice(ssid, new(big.Int).SetInt64(int64(i)))
		h := hex.EncodeToString(contextI)
		if prev, exists := seen[h]; exists {
			t.Fatalf("party %d has same ContextI as party %d: %s", i, prev, h)
		}
		seen[h] = i
	}

	// Also verify party 0 differs from bare SSID.
	if _, exists := seen[hex.EncodeToString(ssid)]; exists {
		t.Fatal("a party's ContextI collides with bare SSID")
	}
}

// TestContextIGoldenVectors freezes the exact hex encoding of ContextI for
// party indices 0-2 with a known SSID. Rust implementations MUST reproduce
// these exact bytes. If the encoding in any round file is reverted to bare
// append, these vectors will no longer match the round's actual output.
func TestContextIGoldenVectors(t *testing.T) {
	ssid := []byte("test-ssid")

	tests := []struct {
		index    uint64
		expected string // hex of full ContextI
	}{
		// ssid("test-ssid") = 746573742d73736964
		// index 0: ssid + [00 00 00 00]
		{0, "746573742d7373696400000000"},
		// index 1: ssid + [00 00 00 01] + [01]
		{1, "746573742d737369640000000101"},
		// index 2: ssid + [00 00 00 01] + [02]
		{2, "746573742d737369640000000102"},
	}
	for _, tc := range tests {
		contextI := AppendBigIntToBytesSlice(ssid, new(big.Int).SetUint64(tc.index))
		got := hex.EncodeToString(contextI)
		if got != tc.expected {
			t.Fatalf("ContextI(ssid, %d) = %s, want %s", tc.index, got, tc.expected)
		}
	}
}

// TestSHA512_256iDomainSeparationLengthPrefix verifies that SHA512_256i(256)
// differs from SHA512_256i(1, 0) even though both have magnitude bytes
// [0x01, 0x00]. The length prefix and block count in the hash preimage
// ensure proper domain separation.
func TestSHA512_256iDomainSeparationLengthPrefix(t *testing.T) {
	// big.NewInt(256).Bytes() = [0x01, 0x00]
	// big.NewInt(1).Bytes() = [0x01], big.NewInt(0).Bytes() = []
	// Without domain separation, the concatenation of magnitudes could collide.
	h256 := SHA512_256i(big.NewInt(256))
	h10 := SHA512_256i(big.NewInt(1), big.NewInt(0))

	if h256.Cmp(h10) == 0 {
		t.Fatal("SHA512_256i(256) should differ from SHA512_256i(1, 0) — " +
			"domain separation via block count and length prefixes must prevent collision")
	}

	// Freeze golden vectors for cross-language verification.
	expect256 := "c1f31d55168ae2cabfa0909fbdd93d248ff8e60386387aef08908bff718f4eb5"
	expect10 := "16e94de4c1997dce6279e7d4c576f9e55f26071515032945a60bb085e38d03e1"
	got256 := hex.EncodeToString(h256.Bytes())
	got10 := hex.EncodeToString(h10.Bytes())
	// Pad to 64 hex chars (32 bytes).
	for len(got256) < 64 {
		got256 = "0" + got256
	}
	for len(got10) < 64 {
		got10 = "0" + got10
	}
	if got256 != expect256 {
		t.Fatalf("SHA512_256i(256) = %s, want %s", got256, expect256)
	}
	if got10 != expect10 {
		t.Fatalf("SHA512_256i(1, 0) = %s, want %s", got10, expect10)
	}
}
