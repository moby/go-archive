package archive

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"golang.org/x/sys/unix"
)

// lsetxattrInRoot does not need a descriptor for name because it sets the
// attribute by pathname through the open parent directory.
func lsetxattrInRoot(root *os.Root, name string, _ *os.File, attr string, data []byte, flags int) error {
	parent, err := root.Open(filepath.Dir(name))
	if err != nil {
		return err
	}
	defer parent.Close()

	// Anchor the pathname to the open parent, not root.Name(), which may no
	// longer identify the directory. Lsetxattr does not follow the final link.
	path := "/proc/self/fd/" + strconv.FormatUint(uint64(parent.Fd()), 10) + "/" + filepath.Base(name)
	if err := unix.Lsetxattr(path, attr, data, flags); err != nil {
		return wrapPathError("lsetxattr", name, attr, fmt.Errorf("via %s: %w", path, err))
	}
	return nil
}
