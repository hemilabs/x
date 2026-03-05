package commitments

import (
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVerifyRejectsEmptyD(t *testing.T) {
	cmt := NewHashCommitment(rand.Reader, big.NewInt(42))
	cmt.D = []*big.Int{}
	assert.False(t, cmt.Verify(), "empty D should be rejected")
}

func TestVerifyRejectsSingletonD(t *testing.T) {
	cmt := NewHashCommitment(rand.Reader, big.NewInt(42))
	cmt.D = []*big.Int{big.NewInt(42)}
	assert.False(t, cmt.Verify(), "singleton D (missing randomness or secret) should be rejected")
}

func TestVerifyRejectsNilD(t *testing.T) {
	cmt := NewHashCommitment(rand.Reader, big.NewInt(42))
	cmt.D = nil
	assert.False(t, cmt.Verify(), "nil D should be rejected")
}

func TestDeCommitRejectsSingletonD(t *testing.T) {
	cmt := NewHashCommitment(rand.Reader, big.NewInt(42))
	cmt.D = []*big.Int{big.NewInt(42)}
	ok, _ := cmt.DeCommit()
	assert.False(t, ok, "DeCommit should fail when D has only one element")
}
