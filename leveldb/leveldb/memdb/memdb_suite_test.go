package memdb

import (
	"testing"

	"github.com/hemilabs/x/leveldb/leveldb/testutil"
)

func TestMemDB(t *testing.T) {
	testutil.RunSuite(t, "MemDB Suite")
}
