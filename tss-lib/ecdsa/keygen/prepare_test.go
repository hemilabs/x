// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package keygen

import (
	"context"
	"testing"
	"time"

)

func TestGeneratePreParamsTimeout(t *testing.T) {
	start := time.Now()
	preParams, err := GeneratePreParams(5*time.Millisecond, 1)

	if preParams != nil {
		t.Fatalf("expected nil, got %v", preParams)
	}
	if err == nil {
		t.Fatal("expected non-nil")
	}
	if diff := time.Now().Sub(start); diff < 0 || diff > 1*time.Second {
		t.Fatalf("duration %v exceeds %v", diff, 1*time.Second)
	}
}

func TestGeneratePreParamsWithContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	start := time.Now()
	preParams, err := GeneratePreParamsWithContext(ctx, 1)

	if preParams != nil {
		t.Fatalf("expected nil, got %v", preParams)
	}
	if err == nil {
		t.Fatal("expected non-nil")
	}
	if diff := time.Now().Sub(start); diff < 0 || diff > 1*time.Second {
		t.Fatalf("duration %v exceeds %v", diff, 1*time.Second)
	}
}

func TestGenerateWithContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	preParams, err := GeneratePreParamsWithContext(ctx, 1)
	if preParams == nil {
		t.Fatal("expected non-nil")
	}
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if preParams.PaillierSK == nil {
		t.Fatal("expected non-nil")
	}
	if preParams.NTildei == nil {
		t.Fatal("expected non-nil")
	}
	if preParams.H1i == nil {
		t.Fatal("expected non-nil")
	}
	if preParams.H2i == nil {
		t.Fatal("expected non-nil")
	}
	if preParams.Alpha == nil {
		t.Fatal("expected non-nil")
	}
	if preParams.Beta == nil {
		t.Fatal("expected non-nil")
	}
	if preParams.P == nil {
		t.Fatal("expected non-nil")
	}
	if preParams.Q == nil {
		t.Fatal("expected non-nil")
	}
}
