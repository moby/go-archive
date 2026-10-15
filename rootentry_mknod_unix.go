//go:build linux || freebsd || netbsd || openbsd || dragonfly

package archive

import (
	"os"

	"github.com/tonistiigi/fsutil"
)

func mknodRootEntry(_ *os.Root, entry *fsutil.RootEntry, _ string, mode uint32, dev uint64) error {
	return entry.Mknod(mode, int(dev)) // #nosec G115 -- Required conversion for fsutil's platform-specific mknod API.
}
