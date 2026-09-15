//go:build !windows

package archive

import (
	"os"
	"time"
)

// chtimes changes the access and modification time of a file at the given
// path relative to root.
//
// Callers must use boundTime to ensure timestamps are within the range
// supported by os.Chtimes.
func chtimes(root *os.Root, name string, atime, mtime time.Time) error {
	return root.Chtimes(name, atime, mtime)
}
