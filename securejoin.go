package archive

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// secureJoinScope joins unsafePath onto root, resolving any symlink components
// that already exist on disk but clamping every traversal (via ".." or via a
// symlink target) so the result can never escape root.
//
// It is used when TarOptions.ConfineSymlinksToRoot is set — most notably by the
// chrootarchive subpackage on Windows, where there is no chroot to contain the
// extraction (see moby/moby#47107). On Linux, chrootarchive performs an actual
// chroot, so an escaping symlink is contained by the kernel; on Windows the
// unpack runs inline against the real filesystem, and secureJoinScope provides
// the equivalent containment: a following entry written *through* an escaping
// symlink resolves back inside root instead of breaking out.
//
// The traversal mirrors github.com/containerd/continuity/fs.RootPath:
// non-existent trailing components are accepted verbatim (so it can be used for
// paths being created), symlink targets are resolved relative to root, and any
// ".." that would climb above root is clamped to root.
func secureJoinScope(root, unsafePath string) (string, error) {
	unsafePath = filepath.FromSlash(unsafePath)

	// resolved is always a cleaned, root-relative path that stays within root.
	var resolved string
	linksWalked := 0

	for unsafePath != "" {
		part, rest := splitFirstComponent(unsafePath)
		unsafePath = rest

		switch part {
		case "", ".":
			continue
		case "..":
			resolved = parentOf(resolved)
			continue
		}

		next := filepath.Join(resolved, part)
		full := filepath.Join(root, next)

		fi, err := os.Lstat(full)
		if err != nil {
			if os.IsNotExist(err) {
				// Nothing exists here yet; accept the component verbatim. This
				// is where new files/dirs/symlinks get created.
				resolved = next
				continue
			}
			return "", err
		}

		if fi.Mode()&os.ModeSymlink == 0 {
			resolved = next
			continue
		}

		linksWalked++
		if linksWalked > 255 {
			return "", errors.New("securejoin: too many symbolic links")
		}

		dest, err := os.Readlink(full)
		if err != nil {
			return "", err
		}
		dest = filepath.FromSlash(dest)

		if filepath.IsAbs(dest) || strings.HasPrefix(dest, string(filepath.Separator)) {
			// Absolute target: reinterpret relative to root (the container
			// root), dropping any volume name and leading separators.
			resolved = ""
			dest = strings.TrimPrefix(dest, filepath.VolumeName(dest))
			dest = strings.TrimLeft(dest, `\/`)
		} else {
			// Relative target resolves from the symlink's parent directory.
			resolved = parentOf(resolved)
		}

		if unsafePath == "" {
			unsafePath = dest
		} else if dest != "" {
			unsafePath = dest + string(filepath.Separator) + unsafePath
		}
	}

	return filepath.Join(root, resolved), nil
}

// splitFirstComponent splits p into its first path component and the remainder.
func splitFirstComponent(p string) (first, rest string) {
	if i := strings.IndexRune(p, filepath.Separator); i >= 0 {
		return p[:i], p[i+1:]
	}
	return p, ""
}

// parentOf returns the parent of a cleaned, root-relative path, never ascending
// above the root (represented by the empty string).
func parentOf(p string) string {
	if p == "" {
		return ""
	}
	parent := filepath.Dir(p)
	if parent == "." || parent == string(filepath.Separator) {
		return ""
	}
	return parent
}
