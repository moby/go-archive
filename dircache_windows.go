//go:build windows

package archive

import (
	"os"
	"time"
)

func newDirCache(root *os.Root) *dirCache {
	return &dirCache{root: root}
}

// dirCache on Windows delegates all operations to os.Root methods. There is
// no fd caching because the equivalent openat(2)-style optimisation is not
// available through the current os.Root API on Windows.
type dirCache struct {
	root *os.Root
}

// close is a no-op on Windows.
func (dc *dirCache) close() {}

// openFile opens or creates path within root.
func (dc *dirCache) openFile(path string, flag int, perm os.FileMode) (*os.File, error) {
	return dc.root.OpenFile(path, flag, perm)
}

// isExistingDir reports whether path within root exists and is a directory.
func (dc *dirCache) isExistingDir(path string) (bool, error) {
	fi, err := dc.root.Lstat(path)
	if err != nil {
		return false, nil
	}
	return fi.IsDir(), nil
}

// mkdir creates a directory at path within root.
func (dc *dirCache) mkdir(path string, perm os.FileMode) error {
	return dc.root.Mkdir(path, perm)
}

// lchown sets ownership of path without following symlinks.
func (dc *dirCache) lchown(path string, uid, gid int) error {
	return dc.root.Lchown(path, uid, gid)
}

// chtimes sets access and modification times of path.
func (dc *dirCache) chtimes(path string, atime, mtime time.Time) error {
	return dc.root.Chtimes(path, atime, mtime)
}

// lchtimes is a no-op on Windows; symlink timestamps are not supported.
func (dc *dirCache) lchtimes(path string, atime, mtime time.Time) error {
	return nil
}
