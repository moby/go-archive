//go:build windows

package chrootarchive

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// TestUntarSymlinkScopeConfine is a regression test for moby/moby#47107
// (TestBuildSymlinkBreakout) on Windows. Windows has no chroot, so chrootarchive
// unpacks inline against the real filesystem. A symlink whose target escapes the
// extraction root must be scope-confined rather than rejected: the symlink is
// created (matching the Linux chroot behaviour that lets the build succeed) and
// a file written *through* it is contained at the extraction root instead of
// breaking out to the host.
func TestUntarSymlinkScopeConfine(t *testing.T) {
	const contained = "contained"

	buf := &bytes.Buffer{}
	tw := tar.NewWriter(buf)
	// An escaping symlink: enough ".." to climb above any extraction root.
	assert.NilError(t, tw.WriteHeader(&tar.Header{
		Name:     "symlink2",
		Typeflag: tar.TypeSymlink,
		Linkname: "/../../../../../../../../../../../../../../",
		Mode:     0o755,
	}))
	// A regular file written through the escaping symlink.
	assert.NilError(t, tw.WriteHeader(&tar.Header{
		Name:     "symlink2/file-in-escape",
		Typeflag: tar.TypeReg,
		Size:     int64(len(contained)),
		Mode:     0o644,
	}))
	_, err := tw.Write([]byte(contained))
	assert.NilError(t, err)
	assert.NilError(t, tw.Close())

	base := t.TempDir()
	dest := filepath.Join(base, "dest")
	assert.NilError(t, os.Mkdir(dest, 0o755))

	// The escaping symlink must not be rejected; extraction succeeds.
	assert.NilError(t, Untar(bytes.NewReader(buf.Bytes()), dest, nil))

	// The symlink itself was created inside dest.
	fi, err := os.Lstat(filepath.Join(dest, "symlink2"))
	assert.NilError(t, err)
	assert.Check(t, fi.Mode()&os.ModeSymlink != 0, "symlink2 should be a symlink")

	// The file written through the escaping symlink is contained at the root of
	// dest (the Windows analogue of the Linux chroot root), not escaped.
	b, err := os.ReadFile(filepath.Join(dest, "file-in-escape"))
	assert.NilError(t, err)
	assert.Equal(t, string(b), contained)

	// Nothing escaped above dest.
	_, err = os.Lstat(filepath.Join(base, "file-in-escape"))
	assert.Check(t, is.ErrorIs(err, os.ErrNotExist))
	_, err = os.Lstat(filepath.Join(filepath.VolumeName(dest)+`\`, "file-in-escape"))
	assert.Check(t, is.ErrorIs(err, os.ErrNotExist))
}
