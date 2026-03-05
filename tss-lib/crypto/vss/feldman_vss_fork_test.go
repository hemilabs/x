package vss

import (
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

func makeValidShares(t *testing.T) (Vs, Shares) {
	t.Helper()
	ec := tss.S256()
	q := ec.Params().N
	secret := common.GetRandomPositiveInt(rand.Reader, q)
	indexes := []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)}
	vs, shares, _, err := Create(ec, 1, secret, indexes, rand.Reader)
	assert.NoError(t, err)
	return vs, shares
}

func TestVerifyRejectsZeroShare(t *testing.T) {
	vs, shares := makeValidShares(t)
	share := *shares[0]
	share.Share = big.NewInt(0)
	assert.False(t, share.Verify(tss.S256(), 1, vs), "zero share should be rejected")
}

func TestVerifyRejectsNegativeShare(t *testing.T) {
	vs, shares := makeValidShares(t)
	share := *shares[0]
	share.Share = big.NewInt(-1)
	assert.False(t, share.Verify(tss.S256(), 1, vs), "negative share should be rejected")
}

func TestVerifyRejectsOutOfRangeShare(t *testing.T) {
	vs, shares := makeValidShares(t)
	q := tss.S256().Params().N
	share := *shares[0]
	share.Share = new(big.Int).Set(q)
	assert.False(t, share.Verify(tss.S256(), 1, vs), "share >= q should be rejected")
}

func TestVerifyRejectsNilShare(t *testing.T) {
	vs, shares := makeValidShares(t)
	share := *shares[0]
	share.Share = nil
	assert.False(t, share.Verify(tss.S256(), 1, vs), "nil share should be rejected")
}

func TestVerifyRejectsNilShareID(t *testing.T) {
	vs, shares := makeValidShares(t)
	share := *shares[0]
	share.ID = nil
	assert.False(t, share.Verify(tss.S256(), 1, vs), "nil share ID should be rejected")
}

func TestVerifyRejectsZeroShareID(t *testing.T) {
	vs, shares := makeValidShares(t)
	share := *shares[0]
	share.ID = big.NewInt(0)
	assert.False(t, share.Verify(tss.S256(), 1, vs), "zero share ID should be rejected")
}

func TestVerifyRejectsShareIDEqualToQ(t *testing.T) {
	vs, shares := makeValidShares(t)
	q := tss.S256().Params().N
	share := *shares[0]
	share.ID = new(big.Int).Set(q)
	assert.False(t, share.Verify(tss.S256(), 1, vs), "share ID == q should be rejected (q mod q == 0)")
}

func TestReconstructRejectsDuplicateIDs(t *testing.T) {
	_, shares := makeValidShares(t)
	shares[1].ID = new(big.Int).Set(shares[0].ID)
	_, err := shares.ReConstruct(tss.S256())
	assert.Error(t, err, "duplicate share IDs should cause ReConstruct to fail")
}

func TestReconstructRejectsDuplicateModQ(t *testing.T) {
	_, shares := makeValidShares(t)
	q := tss.S256().Params().N
	shares[1].ID = new(big.Int).Add(shares[0].ID, q)
	_, err := shares.ReConstruct(tss.S256())
	assert.Error(t, err, "share IDs congruent mod q should cause ReConstruct to fail")
}
