package table

import (
	"testing"

	"github.com/hemilabs/x/leveldb/leveldb/testutil"
)

func TestTable(t *testing.T) {
	testutil.RunSuite(t, "Table Suite")
}
