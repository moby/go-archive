//go:build !linux && !windows

package archive

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
)

// TestChmodNoSymlinkFallback verifies that the chmod fallback applies modes to
// non-symlink entries.
func TestChmodNoSymlinkFallback(t *testing.T) {
	for _, tc := range []struct {
		name   string
		create func(string) error
	}{
		{
			name: "regular-file",
			create: func(p string) error {
				return os.WriteFile(p, nil, 0o600)
			},
		},
		{
			name: "directory",
			create: func(p string) error {
				return os.Mkdir(p, 0o700)
			},
		},
		{
			name: "fifo",
			create: func(p string) error {
				return unix.Mkfifo(p, 0o600)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			entryPath := filepath.Join(tmpDir, tc.name)
			assert.NilError(t, tc.create(entryPath))

			parent, err := os.Open(tmpDir)
			assert.NilError(t, err)
			defer parent.Close()

			// #nosec G115 -- file descriptors fit in int on supported platforms.
			err = chmodNoSymlinkFallback(int(parent.Fd()), tc.name, tc.name, 0o640)
			assert.NilError(t, err, "entryPath: %s", entryPath)

			fi, err := os.Lstat(entryPath)
			assert.NilError(t, err)
			assert.Equal(t, fi.Mode().Perm(), os.FileMode(0o640))
		})
	}
}
