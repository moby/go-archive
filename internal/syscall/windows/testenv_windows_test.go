package windows_test

import (
	"os"
	"testing"
)

// Alternative for [internal/testenv.Executable].
func testenvExecutable(t *testing.T) string {
	t.Helper()

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return exe
}
