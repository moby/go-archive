package archive

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/moby/sys/userns"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestApplyXattrsOnDanglingSymlink(t *testing.T) {
	for _, target := range []string{"usr/bin", "/missing", "../../outside"} {
		t.Run(target, func(t *testing.T) {
			dest := t.TempDir()
			assert.NilError(t, os.Mkdir(filepath.Join(dest, "dir"), 0o755))
			assert.NilError(t, os.Symlink(target, filepath.Join(dest, "dir", "link")))
			root, err := os.OpenRoot(dest)
			assert.NilError(t, err)
			defer root.Close()

			// user xattrs on Linux symlinks normally return EPERM. Compare
			// against the direct syscall so this test needs no SELinux support.
			directErr := unix.Lsetxattr(filepath.Join(dest, "dir", "link"), "user.archive", []byte("value"), 0)
			ignored, err := applyXattrs(root, "dir/link", map[string]string{paxSchilyXattr + "user.archive": "value"}, false, nil)
			if errors.Is(directErr, unix.EPERM) {
				assert.NilError(t, err)
				assert.Equal(t, len(ignored), 1)
				assert.Assert(t, is.Contains(ignored[0], `entry "dir/link"`))
			} else if directErr == nil {
				assert.NilError(t, err)
			} else {
				assert.ErrorIs(t, err, directErr)
			}
			assert.Assert(t, !errors.Is(err, os.ErrNotExist), "must address the symlink, not its missing target")
		})
	}
}

func TestXattrPathPinsParent(t *testing.T) {
	dest := t.TempDir()
	assert.NilError(t, os.Mkdir(filepath.Join(dest, "dir"), 0o755))
	assert.NilError(t, os.WriteFile(filepath.Join(dest, "dir", "file"), nil, 0o600))
	outside := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(outside, "file"), nil, 0o600))
	root, err := os.OpenRoot(dest)
	assert.NilError(t, err)
	defer root.Close()

	err = withXattrPath(root, "dir/file", nil, func(path string) error {
		// Replace the parent after it has been pinned, before the syscall.
		if err := os.Rename(filepath.Join(dest, "dir"), filepath.Join(dest, "moved")); err != nil {
			return err
		}
		if err := os.Symlink(outside, filepath.Join(dest, "dir")); err != nil {
			return err
		}
		return lsetxattr(path, "user.archive", []byte("value"), 0)
	})
	assert.NilError(t, err)
	value, err := lgetxattr(filepath.Join(dest, "moved", "file"), "user.archive")
	assert.NilError(t, err)
	assert.Equal(t, string(value), "value")
	value, err = lgetxattr(filepath.Join(outside, "file"), "user.archive")
	assert.NilError(t, err)
	assert.Assert(t, len(value) == 0)
}

func TestApplyXattrsKeepsErrors(t *testing.T) {
	dest := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(dest, "file"), nil, 0o600))
	root, err := os.OpenRoot(dest)
	assert.NilError(t, err)
	defer root.Close()
	for _, bestEffort := range []bool{false, true} {
		_, err := applyXattrs(root, "file", map[string]string{paxSchilyXattr + "user.bad\x00name": "value"}, bestEffort, nil)
		assert.ErrorIs(t, err, unix.EINVAL)
		assert.Assert(t, is.Contains(err.Error(), `entry "file"`))
		_, err = applyXattrs(root, "missing/file", map[string]string{paxSchilyXattr + "user.archive": "value"}, bestEffort, nil)
		assert.ErrorIs(t, err, os.ErrNotExist)
	}
	// Headers without xattrs must not resolve an xattr destination.
	_, err = applyXattrs(root, "missing/file", map[string]string{"unrelated": "value"}, false, nil)
	assert.NilError(t, err)
}

func TestApplyXattrsAfterRootRename(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "root")
	assert.NilError(t, os.Mkdir(dest, 0o755))
	assert.NilError(t, os.WriteFile(filepath.Join(dest, "file"), nil, 0o200))
	root, err := os.OpenRoot(dest)
	assert.NilError(t, err)
	defer root.Close()

	moved := dest + "-moved"
	assert.NilError(t, os.Rename(dest, moved))
	outside := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(outside, "file"), nil, 0o600))
	assert.NilError(t, os.Symlink(outside, dest))
	_, err = applyXattrs(root, "file", map[string]string{paxSchilyXattr + "user.archive": "value"}, false, nil)
	assert.NilError(t, err)
	// Reading the xattr needs read permission, even though setting it does not.
	assert.NilError(t, os.Chmod(filepath.Join(moved, "file"), 0o600))
	value, err := lgetxattr(filepath.Join(moved, "file"), "user.archive")
	assert.NilError(t, err)
	assert.Equal(t, string(value), "value")
	value, err = lgetxattr(filepath.Join(outside, "file"), "user.archive")
	assert.NilError(t, err)
	assert.Assert(t, len(value) == 0)
}

func TestUntarXattrsThroughSymlinkedParent(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, hdr := range []*tar.Header{
		{Name: "bin", Typeflag: tar.TypeSymlink, Linkname: "usr/bin", Mode: 0o777},
		{Name: "usr", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "usr/bin", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "bin/tool", Typeflag: tar.TypeReg, Mode: 0o600, PAXRecords: map[string]string{paxSchilyXattr + "user.archive": "value"}},
	} {
		assert.NilError(t, tw.WriteHeader(hdr))
	}
	assert.NilError(t, tw.Close())
	dest := t.TempDir()
	assert.NilError(t, Untar(&buf, dest, &TarOptions{NoLchown: true}))
	value, err := lgetxattr(filepath.Join(dest, "usr", "bin", "tool"), "user.archive")
	assert.NilError(t, err)
	assert.Equal(t, string(value), "value")
}

// Regression coverage for https://github.com/moby/go-archive/issues/109.
func TestUntarSELinuxXattrOnDanglingSymlink(t *testing.T) {
	const (
		attr  = "security.selinux"
		value = "system_u:object_r:container_file_t:s0"
	)
	probe := filepath.Join(t.TempDir(), "probe")
	assert.NilError(t, os.Symlink("usr/bin", probe))
	if err := lsetxattr(probe, attr, []byte(value), 0); err != nil {
		t.Skipf("cannot set %q on symlinks: %v", attr, err)
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, hdr := range []*tar.Header{
		{Name: "bin", Typeflag: tar.TypeSymlink, Linkname: "usr/bin", Mode: 0o777, PAXRecords: map[string]string{paxSchilyXattr + attr: value}},
		{Name: "usr/", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "usr/bin/", Typeflag: tar.TypeDir, Mode: 0o755},
	} {
		assert.NilError(t, tw.WriteHeader(hdr))
	}
	assert.NilError(t, tw.Close())
	dest := t.TempDir()
	assert.NilError(t, Untar(&buf, dest, &TarOptions{NoLchown: true}))
	got, err := lgetxattr(filepath.Join(dest, "bin"), attr)
	assert.NilError(t, err)
	assert.Equal(t, string(got), value)
}

// setupOverlayTestDir creates files in a directory with overlay whiteouts
// Tree layout
//
//	.
//	├── d1     # opaque, 0700
//	│   └── f1 # empty file, 0600
//	├── d2     # opaque, 0750
//	│   └── f1 # empty file, 0660
//	└── d3     # 0700
//	    └── f1 # whiteout, 0644
func setupOverlayTestDir(t *testing.T, src string) {
	skip.If(t, os.Getuid() != 0, "skipping test that requires root")
	skip.If(t, userns.RunningInUserNS(), "skipping test that requires initial userns (trusted.overlay.opaque xattr cannot be set in userns, even with Ubuntu kernel)")
	// Create opaque directory containing single file and permission 0700
	err := os.Mkdir(filepath.Join(src, "d1"), 0o700)
	assert.NilError(t, err)

	err = unix.Lsetxattr(filepath.Join(src, "d1"), "trusted.overlay.opaque", []byte("y"), 0)
	assert.NilError(t, err)

	err = os.WriteFile(filepath.Join(src, "d1", "f1"), []byte{}, 0o600)
	assert.NilError(t, err)

	// Create another opaque directory containing single file but with permission 0750
	err = os.Mkdir(filepath.Join(src, "d2"), 0o750)
	assert.NilError(t, err)

	err = unix.Lsetxattr(filepath.Join(src, "d2"), "trusted.overlay.opaque", []byte("y"), 0)
	assert.NilError(t, err)

	err = os.WriteFile(filepath.Join(src, "d2", "f1"), []byte{}, 0o660)
	assert.NilError(t, err)

	// Create regular directory with deleted file
	err = os.Mkdir(filepath.Join(src, "d3"), 0o700)
	assert.NilError(t, err)

	err = unix.Mknod(filepath.Join(src, "d3", "f1"), unix.S_IFCHR, 0)
	assert.NilError(t, err)
}

func checkOpaqueness(t *testing.T, path string, opaque string) {
	xattrOpaque, err := lgetxattr(path, "trusted.overlay.opaque")
	assert.NilError(t, err)

	if string(xattrOpaque) != opaque {
		t.Fatalf("Unexpected opaque value: %q, expected %q", string(xattrOpaque), opaque)
	}
}

func checkOverlayWhiteout(t *testing.T, path string) {
	stat, err := os.Stat(path)
	assert.NilError(t, err)

	statT, ok := stat.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("Unexpected type: %t, expected *syscall.Stat_t", stat.Sys())
	}
	if statT.Rdev != 0 {
		t.Fatalf("Non-zero device number for whiteout")
	}
}

func checkFileMode(t *testing.T, path string, perm os.FileMode) {
	stat, err := os.Stat(path)
	assert.NilError(t, err)

	if stat.Mode() != perm {
		t.Fatalf("Unexpected file mode for %s: %o, expected %o", path, stat.Mode(), perm)
	}
}

func TestOverlayTarUntar(t *testing.T) {
	restore := overrideUmask(0)
	defer restore()

	src, err := os.MkdirTemp("", "docker-test-overlay-tar-src")
	assert.NilError(t, err)
	defer os.RemoveAll(src)

	setupOverlayTestDir(t, src)

	dst, err := os.MkdirTemp("", "docker-test-overlay-tar-dst")
	assert.NilError(t, err)
	defer os.RemoveAll(dst)

	reader, err := TarWithOptions(src, &TarOptions{
		WhiteoutFormat: OverlayWhiteoutFormat,
	})
	assert.NilError(t, err)
	archive, err := io.ReadAll(reader)
	assert.NilError(t, reader.Close())
	assert.NilError(t, err)

	// The archive should encode opaque directories and file whiteouts
	// in AUFS format.
	entries := make(map[string]struct{})
	rdr := tar.NewReader(bytes.NewReader(archive))
	for {
		h, err := rdr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		assert.NilError(t, err)
		assert.Check(t, is.Equal(h.Devmajor, int64(0)), "unexpected device file in archive")
		assert.Check(t, is.DeepEqual(h.PAXRecords, map[string]string(nil)))
		entries[h.Name] = struct{}{}
	}

	assert.DeepEqual(t, entries, map[string]struct{}{
		"d1/":                         {},
		"d1/" + WhiteoutOpaqueDir:     {},
		"d1/f1":                       {},
		"d2/":                         {},
		"d2/" + WhiteoutOpaqueDir:     {},
		"d2/f1":                       {},
		"d3/":                         {},
		"d3/" + WhiteoutPrefix + "f1": {},
	})

	err = Untar(bytes.NewReader(archive), dst, &TarOptions{
		WhiteoutFormat: OverlayWhiteoutFormat,
	})
	assert.NilError(t, err)

	checkFileMode(t, filepath.Join(dst, "d1"), 0o700|os.ModeDir)
	checkFileMode(t, filepath.Join(dst, "d2"), 0o750|os.ModeDir)
	checkFileMode(t, filepath.Join(dst, "d3"), 0o700|os.ModeDir)
	checkFileMode(t, filepath.Join(dst, "d1", "f1"), 0o600)
	checkFileMode(t, filepath.Join(dst, "d2", "f1"), 0o660)
	checkFileMode(t, filepath.Join(dst, "d3", "f1"), os.ModeCharDevice|os.ModeDevice)

	checkOpaqueness(t, filepath.Join(dst, "d1"), "y")
	checkOpaqueness(t, filepath.Join(dst, "d2"), "y")
	checkOpaqueness(t, filepath.Join(dst, "d3"), "")
	checkOverlayWhiteout(t, filepath.Join(dst, "d3", "f1"))
}

// TestOverlayUntarInvalidWhiteoutNames verifies that malformed whiteout names
// are rejected. In particular, it rejects whiteout targets of "." and "..";
// traversal in archive paths is handled by Untar's normal path validation.
func TestOverlayUntarInvalidWhiteoutNames(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: WhiteoutPrefix},
		{name: WhiteoutPrefix + "."},
		{name: WhiteoutPrefix + ".."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var archive bytes.Buffer
			tw := tar.NewWriter(&archive)

			err := tw.WriteHeader(&tar.Header{
				Name:     tc.name,
				Typeflag: tar.TypeReg,
				Mode:     0o600,
			})
			assert.NilError(t, err)
			assert.NilError(t, tw.Close())

			err = Untar(bytes.NewReader(archive.Bytes()), t.TempDir(), &TarOptions{
				WhiteoutFormat: OverlayWhiteoutFormat,
			})
			assert.ErrorContains(t, err, "invalid whiteout entry")
		})
	}
}

func TestOverlayTarAUFSUntar(t *testing.T) {
	restore := overrideUmask(0)
	defer restore()

	src, err := os.MkdirTemp("", "docker-test-overlay-tar-src")
	assert.NilError(t, err)
	defer os.RemoveAll(src)

	setupOverlayTestDir(t, src)

	dst, err := os.MkdirTemp("", "docker-test-overlay-tar-dst")
	assert.NilError(t, err)
	defer os.RemoveAll(dst)

	archive, err := TarWithOptions(src, &TarOptions{
		WhiteoutFormat: OverlayWhiteoutFormat,
	})
	assert.NilError(t, err)
	defer archive.Close()

	err = Untar(archive, dst, &TarOptions{
		WhiteoutFormat: AUFSWhiteoutFormat,
	})
	assert.NilError(t, err)

	checkFileMode(t, filepath.Join(dst, "d1"), 0o700|os.ModeDir)
	checkFileMode(t, filepath.Join(dst, "d1", WhiteoutOpaqueDir), 0o700)
	checkFileMode(t, filepath.Join(dst, "d2"), 0o750|os.ModeDir)
	checkFileMode(t, filepath.Join(dst, "d2", WhiteoutOpaqueDir), 0o750)
	checkFileMode(t, filepath.Join(dst, "d3"), 0o700|os.ModeDir)
	checkFileMode(t, filepath.Join(dst, "d1", "f1"), 0o600)
	checkFileMode(t, filepath.Join(dst, "d2", "f1"), 0o660)
	checkFileMode(t, filepath.Join(dst, "d3", WhiteoutPrefix+"f1"), 0o600)
}

// TestChmodNoSymlinkFallback verifies that the chmod fallback applies modes to
// non-symlink entries, including device nodes on a nodev mount.
//
// Regression test for https://github.com/moby/go-archive/issues/98
// Regression test for https://github.com/moby/moby/issues/53299
func TestChmodNoSymlinkFallback(t *testing.T) {
	for _, tc := range []struct {
		name   string
		nodev  bool
		create func(string) error

		needsRoot bool
	}{
		{
			name: "regular-file",
			create: func(p string) error {
				return os.WriteFile(p, nil, 0o600)
			},
		},
		{
			name: "regular-file-no-read-permission",
			create: func(p string) error {
				return os.WriteFile(p, nil, 0o200)
			},
		},
		{
			name: "directory",
			create: func(p string) error {
				return os.Mkdir(p, 0o700)
			},
		},
		{
			name: "fifo",
			create: func(p string) error {
				return unix.Mkfifo(p, 0o600)
			},
		},
		{
			name:  "character-device-on-nodev",
			nodev: true,
			create: func(p string) error {
				return mknod(p, unix.S_IFCHR|0o600, unix.Mkdev(1, 3))
			},
			needsRoot: true,
		},
		{
			name:  "block-device-on-nodev",
			nodev: true,
			create: func(p string) error {
				return mknod(p, unix.S_IFBLK|0o600, unix.Mkdev(7, 0))
			},
			needsRoot: true,
		},
		{
			// see https://github.com/moby/moby/issues/53299
			name: "ptmx-device",
			create: func(entryPath string) error {
				return mknod(entryPath, unix.S_IFCHR|0o666, unix.Mkdev(5, 2))
			},
			needsRoot: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.needsRoot {
				skip.If(t, os.Getuid() != 0, "requires root")
				skip.If(t, userns.RunningInUserNS(), "requires initial user namespace")
			}

			tmpDir := t.TempDir()
			if tc.nodev {
				assert.NilError(t, unix.Mount("tmpfs", tmpDir, "tmpfs", unix.MS_NODEV, ""))
				t.Cleanup(func() {
					assert.Check(t, unix.Unmount(tmpDir, 0))
				})
			}

			entryPath := filepath.Join(tmpDir, tc.name)
			assert.NilError(t, tc.create(entryPath))

			parent, err := os.Open(tmpDir)
			assert.NilError(t, err)
			defer parent.Close()

			// #nosec G115 -- file descriptors fit in int on supported platforms.
			err = chmodNoSymlinkFallback(int(parent.Fd()), tc.name, tc.name, 0o640, nil)
			assert.NilError(t, err, "entryPath: %s", entryPath)

			fi, err := os.Lstat(entryPath)
			assert.NilError(t, err)
			assert.Equal(t, fi.Mode().Perm(), os.FileMode(0o640))
		})
	}
}
