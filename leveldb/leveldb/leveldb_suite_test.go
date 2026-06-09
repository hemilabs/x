package leveldb

import (
	"testing"

	"github.com/hemilabs/x/leveldb/leveldb/testutil"
)

func TestLevelDB(t *testing.T) {
	testutil.RunSuite(t, "LevelDB Suite")
}
