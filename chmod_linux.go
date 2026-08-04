package archive

import (
	"fmt"
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

// chmodNoSymlinkFallback applies mode without following the final path
// component on systems without fchmodat2 support.
//
// Callers must have already excluded symlink entries.
func chmodNoSymlinkFallback(parentFD int, base, name string, perm uint32) error {
	fd, err := unix.Openat(parentFD, base, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return &os.PathError{Op: "openat", Path: name, Err: err}
	}
	defer unix.Close(fd)

	procPath := "/proc/self/fd/" + strconv.Itoa(fd)
	if err := unix.Chmod(procPath, perm); err != nil {
		return &os.PathError{
			Op:   "chmod",
			Path: name,
			Err:  fmt.Errorf("via %s: %w", procPath, err),
		}
	}
	return nil
}
