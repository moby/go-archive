//go:build linux || darwin || freebsd || netbsd

package archive

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestApplyXattrsOnDanglingSymlink(t *testing.T) {
	for _, target := range []string{"usr/bin", "/missing", "../../outside"} {
		t.Run(target, func(t *testing.T) {
			dest := t.TempDir()
			assert.NilError(t, os.Mkdir(filepath.Join(dest, "dir"), 0o755))
			assert.NilError(t, os.Symlink(target, filepath.Join(dest, "dir", "link")))
			root, err := os.OpenRoot(dest)
			assert.NilError(t, err)
			defer root.Close()

			// Compare with the native syscall: platforms and filesystems differ
			// in whether they support user xattrs on symlinks.
			directErr := unix.Lsetxattr(filepath.Join(dest, "dir", "link"), "user.archive", []byte("probe"), 0)
			ignored, err := applyXattrs(root, "dir/link", map[string]string{paxSchilyXattr + "user.archive": "value"}, false, nil)
			if errors.Is(directErr, unix.EPERM) {
				assert.NilError(t, err)
				assert.Equal(t, len(ignored), 1)
				assert.Assert(t, is.Contains(ignored[0], `entry "dir/link"`))
			} else if directErr == nil {
				assert.NilError(t, err)
				assert.Equal(t, len(ignored), 0)
				value, err := lgetxattr(filepath.Join(dest, "dir", "link"), "user.archive")
				assert.NilError(t, err)
				assert.Equal(t, string(value), "value")
			} else {
				assert.ErrorIs(t, err, directErr)
			}
			assert.Assert(t, !errors.Is(err, os.ErrNotExist), "must address the symlink, not its missing target")
		})
	}
}
