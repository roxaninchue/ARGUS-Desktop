package tools

import (
	"testing"

	"github.com/lacksonkapapahue/argus-desktop/internal/store"
)

// testHome points ARGUS_HOME at dir for this test and closes the shared argus.db
// before the test's temp folders are removed (Windows can't delete an open file).
func testHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("ARGUS_HOME", dir)
	t.Cleanup(store.CloseDefault)
}
