// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package tss

import (
	"crypto/elliptic"
	"errors"
	"fmt"
	"reflect"
	"sync"

	s256k1 "github.com/btcsuite/btcd/btcec/v2"
	"github.com/decred/dcrd/dcrec/edwards/v2"
)

// CurveName is a registered name for an elliptic curve.
type CurveName string

const (
	Secp256k1 CurveName = "secp256k1"
	Ed25519   CurveName = "ed25519"
)

// [FORK] mu guards ec and registry.  Upstream has no lock, so a
// RegisterCurve or SetCurve during a ceremony races with the readers.
var (
	mu       sync.RWMutex
	ec       elliptic.Curve
	registry map[CurveName]elliptic.Curve
)

// Init default curve (secp256k1)
func init() {
	ec = s256k1.S256()

	registry = make(map[CurveName]elliptic.Curve)
	registry[Secp256k1] = s256k1.S256()
	registry[Ed25519] = edwardsCurve
}

// RegisterCurve adds a named curve to the global registry.  It panics
// if the name is already registered, so that a later call cannot
// replace a curve that is in use.
func RegisterCurve(name CurveName, curve elliptic.Curve) {
	mu.Lock()
	defer mu.Unlock()
	if _, exist := registry[name]; exist {
		panic(fmt.Errorf("RegisterCurve: curve %q is already registered", name))
	}
	registry[name] = curve
}

// return curve, exist(bool)
func GetCurveByName(name CurveName) (elliptic.Curve, bool) {
	mu.RLock()
	defer mu.RUnlock()
	if val, exist := registry[name]; exist {
		return val, true
	}

	return nil, false
}

// return name, exist(bool)
func GetCurveName(curve elliptic.Curve) (CurveName, bool) {
	mu.RLock()
	defer mu.RUnlock()
	for name, e := range registry {
		if reflect.TypeOf(curve) == reflect.TypeOf(e) {
			return name, true
		}
	}

	return "", false
}

// SameCurve returns true if both lhs and rhs are the same known curve
func SameCurve(lhs, rhs elliptic.Curve) bool {
	lName, lOk := GetCurveName(lhs)
	rName, rOk := GetCurveName(rhs)
	if lOk && rOk {
		return lName == rName
	}
	// if lhs/rhs not exist, return false
	return false
}

// EC returns the current elliptic curve in use. The default is secp256k1
func EC() elliptic.Curve {
	mu.RLock()
	defer mu.RUnlock()
	return ec
}

// SetCurve sets the curve used by TSS. Must be called before Start. The default is secp256k1
// Deprecated
func SetCurve(curve elliptic.Curve) {
	if curve == nil {
		panic(errors.New("SetCurve received a nil curve"))
	}
	mu.Lock()
	defer mu.Unlock()
	ec = curve
}

// secp256k1
func S256() elliptic.Curve {
	return s256k1.S256()
}

// edwardsCurve is cached because edwards.Edwards() allocates a new
// instance each call.  ECPoint.Add uses pointer identity to check
// curve compatibility, so all Edwards points must share one instance.
var edwardsCurve = edwards.Edwards()

// Edwards returns the Edwards25519 curve for EdDSA.
func Edwards() elliptic.Curve {
	return edwardsCurve
}
