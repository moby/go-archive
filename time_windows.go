package archive

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"

	internalwindows "github.com/moby/go-archive/internal/syscall/windows"
	"golang.org/x/sys/windows"
)

// chtimes changes the access and modification time of a file at the given
// path relative to root.
//
// Symlink entries are handled separately through lchtimes. The final path
// component is expected not to be a reparse point; if one is encountered,
// chtimes returns an error.
//
// Callers must use boundTime to ensure timestamps are within the range
// supported by os.Chtimes.
func chtimes(root *os.Root, name string, atime, mtime time.Time) error {
	parent, err := root.OpenFile(filepath.Dir(name), os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer parent.Close()

	// Symlink entries are handled by lchtimes. The destination for all
	// chtimes callers is therefore expected not to be a reparse point.
	//
	// Do not follow the final component: if it was concurrently replaced
	// with a reparse point, fail instead of updating its target.
	return chtimesAt(parent, filepath.Base(name), atime, mtime, true)
}

func lchtimes(root *os.Root, name string, atime time.Time, mtime time.Time) error {
	return nil
}

func chtimesAt(parent *os.File, name string, atime, mtime time.Time, noFollow bool) error {
	flags := uint64(internalwindows.O_WRITE_ATTRS)
	if noFollow {
		flags |= internalwindows.O_NOFOLLOW_ANY
	}
	h, err := internalwindows.Openat(syscall.Handle(parent.Fd()), name, flags, 0)
	if err != nil {
		// Openat reports a reparse point encountered with O_NOFOLLOW_ANY as ELOOP.
		if noFollow && errors.Is(err, syscall.ELOOP) {
			return breakoutError(err)
		}
		return err
	}
	defer func() { _ = windows.Close(windows.Handle(h)) }()

	var (
		creationTime     = windows.NsecToFiletime(mtime.UnixNano())
		accessTime       = windows.NsecToFiletime(atime.UnixNano())
		modificationTime = windows.NsecToFiletime(mtime.UnixNano())
	)
	return windows.SetFileTime(windows.Handle(h), &creationTime, &accessTime, &modificationTime)
}
