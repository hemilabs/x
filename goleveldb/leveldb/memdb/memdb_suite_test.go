package memdb

import (
	"testing"

	"github.com/hemilabs/x/goleveldb/leveldb/testutil"
)

func TestMemDB(t *testing.T) {
	testutil.RunSuite(t, "MemDB Suite")
}
