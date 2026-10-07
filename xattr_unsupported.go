//go:build !linux && !darwin && !freebsd && !netbsd

package archive

import "github.com/tonistiigi/fsutil"

func lgetxattr(path string, attr string) ([]byte, error) {
	return nil, nil
}

func setRootEntryXattr(entry *fsutil.RootEntry, attr string, data []byte, flags int) error {
	return nil
}
