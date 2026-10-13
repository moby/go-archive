package archive

import (
	"archive/tar"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"gotest.tools/v3/assert"
)

// TestUntarDestinationRootRejectsPathBasedMknod prevents the macOS pathname
// fallback from creating a FIFO outside a renamed caller-owned root.
func TestUntarDestinationRootRejectsPathBasedMknod(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	dest, moved := filepath.Join(base, "dest"), filepath.Join(base, "moved")
	assert.NilError(t, os.Mkdir(dest, 0o755))
	root, err := os.OpenRoot(dest)
	assert.NilError(t, err)
	defer root.Close()
	assert.NilError(t, os.Rename(dest, moved))
	assert.NilError(t, os.Mkdir(dest, 0o755))

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	assert.NilError(t, tw.WriteHeader(&tar.Header{Name: "fifo", Typeflag: tar.TypeFifo, Mode: 0o600}))
	assert.NilError(t, tw.Close())
	err = Untar(&buf, "", &TarOptions{NoLchown: true, DestinationRoot: root})
	assert.Assert(t, errors.Is(err, syscall.ENOTSUP))
	for _, dir := range []string{dest, moved} {
		_, err := os.Lstat(filepath.Join(dir, "fifo"))
		assert.Assert(t, os.IsNotExist(err))
	}
}
