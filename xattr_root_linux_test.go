package archive

import (
	"archive/tar"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

// TestUntarDestinationRootXattrsRetainIdentityAfterRename catches metadata writes
// through the original pathname after a caller-owned root has been renamed.
func TestUntarDestinationRootXattrsRetainIdentityAfterRename(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	dest, moved := filepath.Join(base, "dest"), filepath.Join(base, "moved")
	assert.NilError(t, os.Mkdir(dest, 0o755))
	root, err := os.OpenRoot(dest)
	assert.NilError(t, err)
	defer root.Close()
	assert.NilError(t, os.Rename(dest, moved))
	assert.NilError(t, os.Mkdir(dest, 0o755))

	victim := filepath.Join(dest, "file")
	assert.NilError(t, os.WriteFile(victim, []byte("untouched"), 0o644))
	const attr = "user.go_archive_root"
	err = lsetxattr(victim, attr, []byte("original"), 0)
	skip.If(t, errors.Is(err, unix.ENOTSUP), "filesystem does not support user xattrs")
	assert.NilError(t, err)

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	assert.NilError(t, tw.WriteHeader(&tar.Header{
		Name:       "file",
		Typeflag:   tar.TypeReg,
		Mode:       0o644,
		PAXRecords: map[string]string{paxSchilyXattr + attr: "archive"},
	}))
	assert.NilError(t, tw.Close())
	assert.NilError(t, Untar(&buf, "", &TarOptions{NoLchown: true, DestinationRoot: root}))

	actual, err := lgetxattr(filepath.Join(moved, "file"), attr)
	assert.NilError(t, err)
	assert.Equal(t, string(actual), "archive")
	actual, err = lgetxattr(victim, attr)
	assert.NilError(t, err)
	assert.Equal(t, string(actual), "original")
}
