// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package tss

import (
	"crypto/elliptic"
	"testing"
)

func TestEdwardsSingleton(t *testing.T) {
	a := Edwards()
	b := Edwards()
	if a != b {
		t.Fatal("Edwards() should return the same pointer")
	}
}

func TestS256(t *testing.T) {
	c := S256()
	if c == nil {
		t.Fatal("S256 returned nil")
	}
	if c.Params().BitSize != 256 {
		t.Fatalf("S256 BitSize: want 256, got %d", c.Params().BitSize)
	}
}

func TestECDefault(t *testing.T) {
	c := EC()
	if c == nil {
		t.Fatal("EC returned nil")
	}
}

func TestSetCurve(t *testing.T) {
	orig := EC()
	defer SetCurve(orig)

	SetCurve(Edwards())
	if EC() != Edwards() {
		t.Fatal("SetCurve did not change EC()")
	}
}

func TestSetCurveNilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("SetCurve(nil) should panic")
		}
	}()
	SetCurve(nil)
}

func TestRegisterCurveAndGet(t *testing.T) {
	c := elliptic.P256()
	RegisterCurve("test-p256", c)

	got, ok := GetCurveByName("test-p256")
	if !ok || got != c {
		t.Fatal("GetCurveByName failed for registered curve")
	}

	_, ok = GetCurveByName("nonexistent")
	if ok {
		t.Fatal("GetCurveByName should return false for unknown curve")
	}
}

func TestGetCurveName(t *testing.T) {
	name, ok := GetCurveName(S256())
	if !ok {
		t.Fatal("GetCurveName failed for S256")
	}
	if name != Secp256k1 {
		t.Fatalf("GetCurveName: want %s, got %s", Secp256k1, name)
	}

	name, ok = GetCurveName(Edwards())
	if !ok {
		t.Fatal("GetCurveName failed for Edwards")
	}
	if name != Ed25519 {
		t.Fatalf("GetCurveName: want %s, got %s", Ed25519, name)
	}

	_, ok = GetCurveName(elliptic.P384())
	if ok {
		t.Fatal("GetCurveName should return false for unregistered curve")
	}
}

func TestSameCurve(t *testing.T) {
	if !SameCurve(S256(), S256()) {
		t.Fatal("SameCurve(S256, S256) should be true")
	}
	if !SameCurve(Edwards(), Edwards()) {
		t.Fatal("SameCurve(Edwards, Edwards) should be true")
	}
	if SameCurve(S256(), Edwards()) {
		t.Fatal("SameCurve(S256, Edwards) should be false")
	}
	if SameCurve(elliptic.P384(), S256()) {
		t.Fatal("SameCurve with unregistered curve should be false")
	}
}
