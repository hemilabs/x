package signing

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hemilabs/x/tss-lib/v2/tss"
)

func TestEdDSAPrepareForSigningNoXiMutation(t *testing.T) {
	ec := tss.Edwards()

	ks := []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)}
	xi := new(big.Int).SetInt64(42)
	xiCopy := new(big.Int).Set(xi)

	_ = PrepareForSigning(ec, 0, 3, xi, ks)

	assert.Equal(t, 0, xi.Cmp(xiCopy), "xi must not be mutated by PrepareForSigning")
}

func TestEdDSAPrepareForSigningCollidingKeysPanics(t *testing.T) {
	ec := tss.Edwards()
	// Two identical keys should trigger panic at prepare.go:37-38
	ks := []*big.Int{big.NewInt(42), big.NewInt(42)}
	xi := big.NewInt(7)
	assert.Panics(t, func() {
		PrepareForSigning(ec, 0, 2, xi, ks)
	}, "colliding keys should panic")
}
