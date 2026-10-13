//go:build darwin || freebsd || netbsd

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

// TestLsetxattrInRootRejectsReopeningNonDirectories guards against opening
// entries by name to set attributes: a regular file can be swapped for a
// device node after Lstat, and opening a device can trigger driver side
// effects. A FIFO stands in for devices because creating it needs no
// privileges.
func TestLsetxattrInRootRejectsReopeningNonDirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	assert.NilError(t, unix.Mkfifo(filepath.Join(dir, "fifo"), 0o600))
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "file"), nil, 0o600))
	root, err := os.OpenRoot(dir)
	assert.NilError(t, err)
	defer root.Close()

	for _, name := range []string{"fifo", "file"} {
		err = lsetxattrInRoot(root, name, nil, "user.go_archive_root", []byte("archive"), 0)
		assert.Check(t, errors.Is(err, unix.ENOTSUP), "%s: got %v", name, err)
	}
}

// TestUntarDestinationRootXattrsOnUnreadableEntries catches xattr failures on
// entries the owner cannot read, which pathname-based setxattr allows.
// Files use their creation descriptor; directories are reopened.
func TestUntarDestinationRootXattrsOnUnreadableEntries(t *testing.T) {
	t.Parallel()
	skip.If(t, os.Getuid() == 0, "root bypasses read permission checks")
	const attr = "user.go_archive_root"
	for _, tc := range []struct {
		name     string
		typeflag byte
		mode     os.FileMode
	}{
		{name: "write-only file", typeflag: tar.TypeReg, mode: 0o200},
		{name: "unlistable directory", typeflag: tar.TypeDir, mode: 0o300},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			root, err := os.OpenRoot(dir)
			assert.NilError(t, err)
			defer root.Close()

			var buf bytes.Buffer
			tw := tar.NewWriter(&buf)
			assert.NilError(t, tw.WriteHeader(&tar.Header{
				Name:       "entry",
				Typeflag:   tc.typeflag,
				Mode:       int64(tc.mode),
				PAXRecords: map[string]string{paxSchilyXattr + attr: "archive"},
			}))
			assert.NilError(t, tw.Close())
			err = Untar(&buf, "", &TarOptions{NoLchown: true, DestinationRoot: root})
			skip.If(t, errors.Is(err, unix.ENOTSUP), "filesystem does not support xattrs")
			assert.NilError(t, err)

			path := filepath.Join(dir, "entry")
			fi, err := os.Lstat(path)
			assert.NilError(t, err)
			assert.Equal(t, fi.Mode().Perm(), tc.mode)
			// Reading attributes needs read access.
			assert.NilError(t, os.Chmod(path, 0o600))
			actual, err := lgetxattr(path, attr)
			assert.NilError(t, err)
			assert.Equal(t, string(actual), "archive")
		})
	}
}
