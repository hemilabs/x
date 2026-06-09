// Copyright (c) 2019-2023 Binance
// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package modproof

import (
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/hemilabs/x/tss/v3/common"
)

const (
	Iterations         = 80
	ProofModBytesParts = Iterations*2 + 3
)

var one = big.NewInt(1)

type (
	ProofMod struct {
		W *big.Int
		X [Iterations]*big.Int
		A *big.Int
		B *big.Int
		Z [Iterations]*big.Int
	}
)

// isQuadraticResidue checks Euler criterion
func isQuadraticResidue(X, N *big.Int) bool {
	return big.Jacobi(X, N) == 1
}

// NewProof generates a mod proof. Session provides SSID domain separation.
func NewProof(Session []byte, N, P, Q *big.Int, rand io.Reader) (*ProofMod, error) {
	Phi := new(big.Int).Mul(new(big.Int).Sub(P, one), new(big.Int).Sub(Q, one))
	// Fig 16.1
	W := common.GetRandomQuadraticNonResidue(rand, N)

	// Fig 16.2
	Y := [Iterations]*big.Int{}
	for i := range Y {
		ei := common.SHA512_256i_TAGGED(Session, append([]*big.Int{W, N}, Y[:i]...)...)
		Y[i] = common.RejectionSample(N, ei)
	}

	// Fig 16.3
	modN, modPhi := common.ModInt(N), common.ModInt(Phi)
	invN := new(big.Int).ModInverse(N, Phi)
	if invN == nil {
		return nil, errors.New("n not coprime with phi")
	}
	X := [Iterations]*big.Int{}
	// Fix bitLen of A and B
	A := new(big.Int).Lsh(one, Iterations)
	B := new(big.Int).Lsh(one, Iterations)
	Z := [Iterations]*big.Int{}

	// for fourth-root
	expo := new(big.Int).Add(Phi, big.NewInt(4))
	expo = new(big.Int).Rsh(expo, 3)
	expo = modPhi.Mul(expo, expo)

	for i := range Y {
		for j := 0; j < 4; j++ {
			a, b := j&1, j&2>>1
			Yi := new(big.Int).SetBytes(Y[i].Bytes())
			if a > 0 {
				Yi = modN.Mul(big.NewInt(-1), Yi)
			}
			if b > 0 {
				Yi = modN.Mul(W, Yi)
			}
			if isQuadraticResidue(Yi, P) && isQuadraticResidue(Yi, Q) {
				Xi := modN.Exp(Yi, expo)
				Zi := modN.Exp(Y[i], invN)
				X[i], Z[i] = Xi, Zi
				A.SetBit(A, i, uint(a))
				B.SetBit(B, i, uint(b))
				break
			}
		}
		// [FORK] Defense-in-depth: fail fast if no quadratic residue was found.
		//
		// This condition is unreachable when P and Q are safe primes because:
		//   - Safe primes satisfy P ≡ Q ≡ 3 (mod 4), so Jacobi(-1, P) = Jacobi(-1, Q) = -1
		//     (negation flips both Legendre symbols).
		//   - W has Jacobi(W, N) = -1, meaning it flips exactly one of the two
		//     Legendre symbols (QNR mod one prime, QR mod the other).
		//   - Together, the four candidates {Y, -Y, W·Y, -W·Y} cycle through all four
		//     quadratic residuosity classes (QR/QR, QR/NR, NR/QR, NR/NR), so exactly
		//     one candidate is always a QR mod both P and Q.
		//
		// If this error fires, it indicates P or Q are not safe primes (not ≡ 3 mod 4),
		// or N is otherwise malformed. Without this check, NewProof would return a proof
		// with nil X[i]/Z[i] entries that silently fails verification at a remote party,
		// obscuring the root cause.
		if X[i] == nil {
			return nil, fmt.Errorf("NewProof: no quadratic residue found for Y[%d]; P and Q must be safe primes (≡ 3 mod 4)", i)
		}
	}

	pf := &ProofMod{W: W, X: X, A: A, B: B, Z: Z}
	return pf, nil
}

func NewProofFromBytes(bzs [][]byte) (*ProofMod, error) {
	if !common.NonEmptyMultiBytes(bzs, ProofModBytesParts) {
		return nil, fmt.Errorf("expected %d byte parts to construct ProofMod", ProofModBytesParts)
	}
	bis := make([]*big.Int, len(bzs))
	for i := range bis {
		bis[i] = new(big.Int).SetBytes(bzs[i]) //nolint:gosec // i bounded by len(bzs)
	}

	X := [Iterations]*big.Int{}
	copy(X[:], bis[1:(Iterations+1)])

	Z := [Iterations]*big.Int{}
	copy(Z[:], bis[(Iterations+3):])

	return &ProofMod{
		W: bis[0],
		X: X,
		A: bis[Iterations+1],
		B: bis[Iterations+2],
		Z: Z,
	}, nil
}

func (pf *ProofMod) Verify(Session []byte, N *big.Int) bool {
	if pf == nil || !pf.ValidateBasic() {
		return false
	}
	// [FORK] Reject N that is too small to be secure (must be at least 2048 bits).
	// Upstream does not check N's size.
	if N == nil || N.BitLen() < 2048 {
		return false
	}
	if isQuadraticResidue(pf.W, N) {
		return false
	}
	// Validate W is in the correct range and coprime with N.
	if pf.W.Sign() != 1 || pf.W.Cmp(N) != -1 {
		return false
	}
	gcd := new(big.Int).GCD(nil, nil, pf.W, N)
	if gcd.Cmp(one) != 0 {
		return false
	}
	// Range checks on proof elements: Z[i] and X[i] must be in (1, N),
	// and A/B must have the correct bit length.
	for i := range pf.Z {
		if pf.Z[i].Sign() != 1 || pf.Z[i].Cmp(N) != -1 {
			return false
		}
	}
	for i := range pf.X {
		if pf.X[i].Sign() != 1 || pf.X[i].Cmp(N) != -1 {
			return false
		}
	}
	if pf.A.BitLen() != Iterations+1 {
		return false
	}
	if pf.B.BitLen() != Iterations+1 {
		return false
	}

	modN := common.ModInt(N)
	Y := [Iterations]*big.Int{}
	for i := range Y {
		ei := common.SHA512_256i_TAGGED(Session, append([]*big.Int{pf.W, N}, Y[:i]...)...)
		Y[i] = common.RejectionSample(N, ei)
	}

	// Fig 16. Verification
	{
		if N.Bit(0) == 0 || N.ProbablyPrime(30) {
			return false
		}
	}

	chs := make(chan bool, Iterations*2)
	for i := 0; i < Iterations; i++ {
		go func(i int) {
			defer func() {
				if r := recover(); r != nil {
					chs <- false
				}
			}()
			left := modN.Exp(pf.Z[i], N)
			if left.Cmp(Y[i]) != 0 {
				chs <- false
				return
			}
			chs <- true
		}(i)

		go func(i int) {
			defer func() {
				if r := recover(); r != nil {
					chs <- false
				}
			}()
			a := pf.A.Bit(i)
			b := pf.B.Bit(i)
			// Defense-in-depth: Bit() always returns 0 or 1 per Go stdlib (math/big),
			// so these conditions are unreachable. Retained as a safeguard against
			// hypothetical stdlib behavior changes.
			if a != 0 && a != 1 {
				chs <- false
				return
			}
			if b != 0 && b != 1 {
				chs <- false
				return
			}
			left := modN.Exp(pf.X[i], big.NewInt(4))
			right := Y[i]
			if a > 0 {
				right = modN.Mul(big.NewInt(-1), right)
			}
			if b > 0 {
				right = modN.Mul(pf.W, right)
			}
			if left.Cmp(right) != 0 {
				chs <- false
				return
			}
			chs <- true
		}(i)
	}

	for i := 0; i < Iterations*2; i++ {
		if !<-chs {
			return false
		}
	}

	return true
}

func (pf *ProofMod) ValidateBasic() bool {
	if pf.W == nil {
		return false
	}
	for i := range pf.X {
		if pf.X[i] == nil {
			return false
		}
	}
	if pf.A == nil {
		return false
	}
	if pf.B == nil {
		return false
	}
	for i := range pf.Z {
		if pf.Z[i] == nil {
			return false
		}
	}
	return true
}

func (pf *ProofMod) Bytes() [ProofModBytesParts][]byte {
	bzs := [ProofModBytesParts][]byte{}
	bzs[0] = pf.W.Bytes()
	for i := range pf.X {
		if pf.X[i] != nil {
			bzs[1+i] = pf.X[i].Bytes()
		}
	}
	bzs[Iterations+1] = pf.A.Bytes()
	bzs[Iterations+2] = pf.B.Bytes()
	for i := range pf.Z {
		if pf.Z[i] != nil {
			bzs[Iterations+3+i] = pf.Z[i].Bytes()
		}
	}
	return bzs
}
