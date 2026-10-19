/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package archive

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

var errTooManyLinks = errors.New("too many links")

type fsRootPathResult struct {
	path                         string
	followedAbsoluteLink         bool
	relativeEscapeBeforeAbsolute bool
}

// fsRootPath returns a host path for operations that os.Root does not support.
// It resolves path through the root handle, then fails if the host path's
// parent no longer leads to the directory found through the handle, such as
// after the root was renamed and replaced.
func fsRootPath(root *os.Root, path string) (string, error) {
	result, err := resolveRootPath(root, path)
	if err != nil {
		return "", err
	}
	parent := filepath.Dir(result.path)
	// This only rejects a root that was already replaced.
	// The name can still be swapped after the check, but that now requires
	// winning a race rather than preparing a replacement tree in advance.
	// Closing the race needs handle-relative xattr and mknod calls, which
	// x/sys does not provide on every supported platform.
	// Stat follows a destination opened through a symlink, and root.Stat
	// rejects a parent swapped for a symlink that escapes the root.
	want, err := root.Stat(parent)
	if err != nil {
		return "", err
	}
	hostParent := filepath.Join(root.Name(), parent)
	got, err := os.Stat(hostParent)
	if err != nil {
		return "", err
	}
	if !os.SameFile(got, want) {
		return "", breakoutError(fmt.Errorf("%q no longer refers to %q in the opened root", hostParent, parent))
	}
	return filepath.Join(root.Name(), result.path), nil
}

// resolveRootPath resolves symlinks through the directory handle, including
// absolute targets interpreted relative to root rather than the host filesystem.
func resolveRootPath(root *os.Root, path string) (fsRootPathResult, error) {
	result := fsRootPathResult{path: "."}
	parts := strings.Split(filepath.FromSlash(path), string(os.PathSeparator))
	resolved := make([]string, 0, len(parts))
	linksWalked := 0
	for len(parts) > 0 {
		part := parts[0]
		parts = parts[1:]
		switch part {
		case "", ".":
			continue
		case "..":
			if len(resolved) > 0 {
				resolved = resolved[:len(resolved)-1]
			} else if !result.followedAbsoluteLink {
				// Keep the path bounded, but do not let a later absolute link
				// hide a relative escape from resolveArchivePath.
				result.relativeEscapeBeforeAbsolute = true
			}
			continue
		}

		candidate := filepath.Join(append(resolved, part)...)
		fi, err := root.Lstat(candidate)
		if os.IsNotExist(err) {
			// A later .. can return to an existing parent. Continue resolving
			// instead of cleaning the remaining path and skipping its symlinks.
			resolved = append(resolved, part)
			continue
		}
		if err != nil {
			return fsRootPathResult{}, err
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			if len(parts) > 0 && !fi.IsDir() {
				return fsRootPathResult{}, &os.PathError{Op: "resolve", Path: filepath.Join(root.Name(), candidate), Err: syscall.ENOTDIR}
			}
			resolved = append(resolved, part)
			continue
		}
		if linksWalked == 255 {
			return fsRootPathResult{}, errTooManyLinks
		}
		linksWalked++
		target, err := root.Readlink(candidate)
		if err != nil {
			return fsRootPathResult{}, err
		}
		if filepath.IsAbs(target) {
			result.followedAbsoluteLink = true
			resolved = resolved[:0]
		}
		// Expand symlinks before processing a following ..; lexical cleaning
		// would remove the link rather than the directory it resolves to.
		parts = append(strings.Split(filepath.FromSlash(target), string(os.PathSeparator)), parts...)
	}
	if len(resolved) > 0 {
		result.path = filepath.Join(resolved...)
	}
	return result, nil
}
