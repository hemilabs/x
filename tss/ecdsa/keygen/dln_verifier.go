// Copyright (c) 2019 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package keygen

import (
	"context"
	"errors"
	"math/big"

	"github.com/hemilabs/x/tss/v3/crypto/dlnproof"
)

// DlnProofVerifier runs DLN proof verification concurrently with
// bounded parallelism.
type DlnProofVerifier struct {
	semaphore chan interface{}
}

// NewDlnProofVerifier creates a verifier that runs at most concurrency
// verifications at the same time.  It returns an error if concurrency < 1.
func NewDlnProofVerifier(concurrency int) (*DlnProofVerifier, error) {
	if concurrency < 1 {
		return nil, errors.New("dln proof verifier: concurrency must be at least 1")
	}
	return &DlnProofVerifier{
		semaphore: make(chan interface{}, concurrency),
	}, nil
}

// VerifyDLNProof verifies a DLN proof with bounded concurrency and calls
// onDone once with the result.  It calls onDone(false) for a nil proof,
// and also when ctx is done before the proof is verified.  A caller that
// cancels ctx must not count that false as a bad proof.
func (dpv *DlnProofVerifier) VerifyDLNProof(ctx context.Context, proof *dlnproof.Proof, Session []byte, h1, h2, n *big.Int, onDone func(bool)) {
	select {
	case dpv.semaphore <- struct{}{}:
	case <-ctx.Done():
		onDone(false)
		return
	}
	go func() {
		defer func() { <-dpv.semaphore }()
		if proof == nil || ctx.Err() != nil {
			onDone(false)
			return
		}
		onDone(proof.Verify(Session, h1, h2, n))
	}()
}
