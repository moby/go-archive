//go:build linux || darwin || freebsd || netbsd

package archive

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/tonistiigi/fsutil"
	"golang.org/x/sys/unix"
)

// lgetxattr retrieves the value of the extended attribute identified by attr
// and associated with the given path in the file system.
// It returns a nil slice and nil error if the xattr is not set.
func lgetxattr(filePath string, attr string) ([]byte, error) {
	// Start with a 128 length byte array
	dest := make([]byte, 128)
	sz, err := unix.Lgetxattr(filePath, attr, dest)

	for errors.Is(err, unix.ERANGE) {
		// Buffer too small, use zero-sized buffer to get the actual size
		sz, err = unix.Lgetxattr(filePath, attr, []byte{})
		if err != nil {
			return nil, wrapPathError("lgetxattr", filePath, attr, err)
		}
		dest = make([]byte, sz)
		sz, err = unix.Lgetxattr(filePath, attr, dest)
	}

	if err != nil {
		if errors.Is(err, noattr) {
			return nil, nil
		}
		return nil, wrapPathError("lgetxattr", filePath, attr, err)
	}

	return dest[:sz], nil
}

func setRootEntryXattr(entry *fsutil.RootEntry, attr string, data []byte, flags int) error {
	if err := entry.LSetxattr(attr, data, flags); err != nil {
		return wrapPathError("lsetxattr", entry.Path(), attr, err)
	}
	return nil
}

func wrapPathError(op, filePath, attr string, err error) error {
	if err == nil {
		return nil
	}
	return &fs.PathError{Op: op, Path: filePath, Err: fmt.Errorf("xattr %q: %w", attr, err)}
}
