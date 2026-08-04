//go:build linux

package archive

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/moby/go-archive/internal/mounttree"
	"github.com/moby/go-archive/internal/unshare"
	"github.com/moby/sys/mount"
	"github.com/moby/sys/userns"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

// TestChmodNoSymlinkFallbackInChrootWithoutProc verifies that the chmod
// fallback works after switching into a root without procfs mounted.
//
// The test lives in the archive package instead of chrootarchive because it
// exercises the unexported fallback directly. Reproducing chrootarchive's
// root-switching setup here is a small workaround to keep the test focused;
// moving the shared extraction internals into an internal package may provide
// a cleaner boundary in the future.
func TestChmodNoSymlinkFallbackInChrootWithoutProc(t *testing.T) {
	t.Skip("FIXME: needs changes in chrootarchive to pass through /proc/self/fd/, which isn't present inside the chroot")

	skip.If(t, os.Getuid() != 0, "test requires root")
	skip.If(t, userns.RunningInUserNS(), "test requires the initial user namespace")

	const name = "target-dir/target-file"

	root := t.TempDir()
	assert.NilError(t, os.Mkdir(filepath.Join(root, filepath.Dir(name)), 0o755))
	assert.NilError(t, os.WriteFile(filepath.Join(root, name), nil, 0o600))

	setupFn := func() error {
		if err := mount.MakeRSlave("/"); err != nil {
			return err
		}
		return mounttree.SwitchRoot(root)
	}

	var testErr error
	err := unshare.Go(unix.CLONE_FS|unix.CLONE_NEWNS, setupFn, func() {
		if _, err := os.Stat("/proc/self/fd"); !errors.Is(err, os.ErrNotExist) {
			testErr = fmt.Errorf("/proc/self/fd: expected not to exist, got %w", err)
			return
		}

		parentFD, err := unix.Open("/"+filepath.Dir(name), unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
		if err != nil {
			testErr = err
			return
		}
		defer unix.Close(parentFD)

		if err := chmodNoSymlinkFallback(parentFD, filepath.Base(name), name, 0o644); err != nil {
			testErr = err
			return
		}

		var stat unix.Stat_t
		if err := unix.Stat("/"+name, &stat); err != nil {
			testErr = err
			return
		}
		if got := stat.Mode & 0o777; got != 0o644 {
			testErr = fmt.Errorf("file mode = %#o, want %#o", got, uint32(0o644))
		}
	})
	assert.NilError(t, err)
	assert.NilError(t, testErr)
}
