//go:build !linux && !darwin && !freebsd && !netbsd

package archive

import "os"

func lgetxattr(path string, attr string) ([]byte, error) {
	return nil, nil
}

func lsetxattr(path string, attr string, data []byte, flags int) error {
	return nil
}

func lsetxattrInRoot(root *os.Root, path string, file *os.File, attr string, data []byte, flags int) error {
	return nil
}
