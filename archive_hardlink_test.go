package archive

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestHardlinkInfoSingleLink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file")
	err := os.WriteFile(path, []byte("hello"), 0o644)
	assert.NilError(t, err)

	fi, err := os.Lstat(path)
	assert.NilError(t, err)

	multiLink, _, err := hardlinkInfo(path, fi)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(multiLink, false))
}

func TestHardlinkInfoMultiLink(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")

	err := os.WriteFile(a, []byte("hello"), 0o644)
	assert.NilError(t, err)

	if err := os.Link(a, b); err != nil {
		t.Skipf("skipping; hardlinks unsupported on this filesystem: %v", err)
	}

	fiA, err := os.Lstat(a)
	assert.NilError(t, err)
	fiB, err := os.Lstat(b)
	assert.NilError(t, err)

	multiA, idA, err := hardlinkInfo(a, fiA)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(multiA, true))

	multiB, idB, err := hardlinkInfo(b, fiB)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(multiB, true))

	// both paths refer to the same underlying file, so the SeenFiles key
	// must match
	assert.Check(t, is.Equal(idA, idB))
}

func TestHardlinkInfoDistinctFiles(t *testing.T) {
	dir := t.TempDir()
	a1 := filepath.Join(dir, "a1")
	a2 := filepath.Join(dir, "a2")
	b1 := filepath.Join(dir, "b1")
	b2 := filepath.Join(dir, "b2")

	for _, p := range []string{a1, b1} {
		err := os.WriteFile(p, []byte("x"), 0o644)
		assert.NilError(t, err)
	}
	if err := os.Link(a1, a2); err != nil {
		t.Skipf("skipping; hardlinks unsupported on this filesystem: %v", err)
	}
	err := os.Link(b1, b2)
	assert.NilError(t, err)

	fiA, err := os.Lstat(a1)
	assert.NilError(t, err)
	fiB, err := os.Lstat(b1)
	assert.NilError(t, err)

	_, idA, err := hardlinkInfo(a1, fiA)
	assert.NilError(t, err)
	_, idB, err := hardlinkInfo(b1, fiB)
	assert.NilError(t, err)

	// two independent hardlink groups must produce different keys
	assert.Check(t, idA != idB, "expected distinct SeenFiles keys, got %d for both", idA)
}
