package leveldb

import (
	"testing"

	"github.com/hemilabs/x/goleveldb/leveldb/testutil"
)

func TestLevelDB(t *testing.T) {
	testutil.RunSuite(t, "LevelDB Suite")
}
