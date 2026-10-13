//go:build darwin || freebsd || netbsd

package archive

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// lsetxattrInRoot sets an extended attribute through file, the descriptor
// already open for name, or else through a newly opened directory.
func lsetxattrInRoot(root *os.Root, name string, file *os.File, attr string, data []byte, flags int) error {
	if file == nil {
		// Reopen only directories: a device node swapped in after Lstat would
		// be opened, which can trigger driver side effects, and only
		// O_DIRECTORY refuses it atomically.
		fi, err := root.Lstat(name)
		if err != nil {
			return err
		}
		if !fi.IsDir() {
			return wrapPathError("fsetxattr", name, attr, unix.ENOTSUP)
		}
		// Do not fall back to root.Name(): a rename may have replaced that directory.
		dir, err := openDirForXattr(root, name, fi.Mode())
		if err != nil {
			if errors.Is(err, unix.ENOTDIR) || errors.Is(err, unix.ELOOP) {
				return wrapPathError("fsetxattr", name, attr, unix.ENOTSUP)
			}
			return err
		}
		defer dir.Close()
		file = dir
	}
	return wrapPathError("fsetxattr", name, attr, unix.Fsetxattr(int(file.Fd()), attr, data, flags)) // #nosec G115 -- Required descriptor conversion.
}

// openDirForXattr opens the directory name for fsetxattr, which needs a
// descriptor, whereas pathname-based setxattr needs only write access.
// Owner read access is granted only for the open, for example for a
// write-only directory, and the mode is restored before attributes are set so
// their permission checks still apply.
func openDirForXattr(root *os.Root, name string, mode os.FileMode) (*os.File, error) {
	open := func() (*os.File, error) {
		return root.OpenFile(name, os.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	}
	file, err := open()
	if !errors.Is(err, unix.EACCES) || mode&0o400 != 0 {
		return file, err
	}
	if chmodNoSymlink(root, name, mode|0o400, nil) != nil {
		return nil, err
	}
	file, err = open()
	if restoreErr := chmodNoSymlink(root, name, mode, nil); restoreErr != nil {
		if file != nil {
			_ = file.Close()
		}
		return nil, restoreErr
	}
	return file, err
}
