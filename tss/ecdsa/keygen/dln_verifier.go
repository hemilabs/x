// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"math/big"

	"github.com/hemilabs/x/tss/v3/crypto/dlnproof"
)

// DlnProofVerifier runs DLN proof verification concurrently with
// bounded parallelism.
type DlnProofVerifier struct {
	semaphore chan interface{}
}

// NewDlnProofVerifier creates a verifier with the given concurrency.
// A value below 1 is used as 1.
func NewDlnProofVerifier(concurrency int) *DlnProofVerifier {
	return &DlnProofVerifier{
		semaphore: make(chan interface{}, max(concurrency, 1)),
	}
}

// VerifyDLNProof verifies a DLN proof with bounded concurrency.
// The proof may be nil (SNARK mode), in which case onDone(false).
func (dpv *DlnProofVerifier) VerifyDLNProof(proof *dlnproof.Proof, Session []byte, h1, h2, n *big.Int, onDone func(bool)) {
	dpv.semaphore <- struct{}{}
	go func() {
		defer func() { <-dpv.semaphore }()
		if proof == nil {
			onDone(false)
			return
		}
		onDone(proof.Verify(Session, h1, h2, n))
	}()
}
