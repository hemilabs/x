// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package keygen

import (
	"errors"
	"math/big"

	"github.com/hemilabs/x/tss-lib/v2/crypto/dlnproof"
)

type DlnProofVerifier struct {
	semaphore chan interface{}
}

type message interface {
	UnmarshalDLNProof1() (*dlnproof.Proof, error)
	UnmarshalDLNProof2() (*dlnproof.Proof, error)
}

func NewDlnProofVerifier(concurrency int) *DlnProofVerifier {
	if concurrency == 0 {
		panic(errors.New("NewDlnProofverifier: concurrency level must not be zero"))
	}

	semaphore := make(chan interface{}, concurrency)

	return &DlnProofVerifier{
		semaphore: semaphore,
	}
}

// [FORK] VerifyDLNProof1: upstream did not pass a Session parameter to DLN proof verification.
// The Session []byte provides SSID-based domain separation so that DLN proofs from one ceremony
// cannot be replayed in a different ceremony (cross-ceremony DLN proof replay prevention).
func (dpv *DlnProofVerifier) VerifyDLNProof1(
	m message,
	Session []byte,
	h1, h2, n *big.Int,
	onDone func(bool),
) {
	dpv.semaphore <- struct{}{}
	go func() {
		defer func() { <-dpv.semaphore }()

		dlnProof, err := m.UnmarshalDLNProof1()
		if err != nil {
			onDone(false)
			return
		}

		onDone(dlnProof.Verify(Session, h1, h2, n))
	}()
}

// [FORK] VerifyDLNProof2: same Session-based domain separation as VerifyDLNProof1 (see above).
func (dpv *DlnProofVerifier) VerifyDLNProof2(
	m message,
	Session []byte,
	h1, h2, n *big.Int,
	onDone func(bool),
) {
	dpv.semaphore <- struct{}{}
	go func() {
		defer func() { <-dpv.semaphore }()

		dlnProof, err := m.UnmarshalDLNProof2()
		if err != nil {
			onDone(false)
			return
		}

		onDone(dlnProof.Verify(Session, h1, h2, n))
	}()
}
