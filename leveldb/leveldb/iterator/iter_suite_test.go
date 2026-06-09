package iterator_test

import (
	"testing"

	"github.com/hemilabs/x/leveldb/leveldb/testutil"
)

func TestIterator(t *testing.T) {
	testutil.RunSuite(t, "Iterator Suite")
}
