//go:build !linux && !freebsd && !netbsd && !openbsd && !dragonfly && !windows

package archive

import (
	"os"

	"github.com/tonistiigi/fsutil"
)

func mknodRootEntry(root *os.Root, _ *fsutil.RootEntry, path string, mode uint32, dev uint64) error {
	return mknodInRoot(root, path, mode, dev)
}
