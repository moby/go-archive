package archive

import (
	"archive/tar"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// longPathPrefix is the longpath prefix for Windows file paths.
const longPathPrefix = `\\?\`

// addLongPathPrefix adds the Windows long path prefix to the path provided if
// it does not already have it. It is a no-op on platforms other than Windows.
//
// addLongPathPrefix is a copy of [github.com/docker/docker/pkg/longpath.AddPrefix].
func addLongPathPrefix(srcPath string) string {
	if strings.HasPrefix(srcPath, longPathPrefix) {
		return srcPath
	}
	if strings.HasPrefix(srcPath, `\\`) {
		// This is a UNC path, so we need to add 'UNC' to the path as well.
		return longPathPrefix + `UNC` + srcPath[1:]
	}
	return longPathPrefix + srcPath
}

// getWalkRoot calculates the root path when performing a TarWithOptions.
// We use a separate function as this is platform specific.
func getWalkRoot(srcPath string, include string) string {
	return filepath.Join(srcPath, include)
}

// chmodTarEntry is used to adjust the file permissions used in tar header based
// on the platform the archival is done.
func chmodTarEntry(perm os.FileMode) os.FileMode {
	// Remove group- and world-writable bits.
	perm &= 0o755

	// Add the x bit: make everything +x on Windows
	return perm | 0o111
}

func getInodeFromStat(stat interface{}) (uint64, error) {
	// do nothing. no notion of Inode in stat on Windows
	return 0, nil
}

// handleTarTypeBlockCharFifo is an OS-specific helper function used by
// createTarFile to handle the following types of header: Block; Char; Fifo
func handleTarTypeBlockCharFifo(hdr *tar.Header, path string) error {
	return nil
}

func handleLChmod(hdr *tar.Header, path string, hdrInfo os.FileInfo) error {
	return nil
}

func getFileUIDGID(stat interface{}) (int, int, error) {
	// no notion of file ownership mapping yet on Windows
	return 0, 0, nil
}

// hardlinkInfo reports whether path refers to a file with more than one hard
// link, and returns an identifier suitable as a SeenFiles key. The data
// exposed via os.FileInfo.Sys() does not include NumberOfLinks or the file
// ID, so the file is opened and GetFileInformationByHandle is called
// directly.
func hardlinkInfo(path string, _ os.FileInfo) (multiLink bool, id uint64, err error) {
	p, err := windows.UTF16PtrFromString(addLongPathPrefix(path))
	if err != nil {
		return false, 0, err
	}
	h, err := windows.CreateFile(
		p,
		0, // no access required to call GetFileInformationByHandle
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return false, 0, err
	}
	defer windows.CloseHandle(h)

	var bhfi windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &bhfi); err != nil {
		return false, 0, err
	}
	if bhfi.NumberOfLinks <= 1 {
		return false, 0, nil
	}
	id = (uint64(bhfi.FileIndexHigh) << 32) | uint64(bhfi.FileIndexLow)
	id ^= uint64(bhfi.VolumeSerialNumber)
	return true, id, nil
}
