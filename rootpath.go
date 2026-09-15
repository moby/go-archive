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
	"os"
	"path/filepath"
	"strings"
)

var errTooManyLinks = errors.New("too many links")

type fsRootPathResult struct {
	path                         string
	followedAbsoluteLink         bool
	relativeEscapeBeforeAbsolute bool
}

// fsRootPath joins a path with a root, evaluating and bounding any
// symlink to the root directory.
func fsRootPath(root, path string) (string, error) {
	result, err := resolveFSRootPath(root, path)
	if err != nil {
		return "", err
	}
	return result.path, nil
}

// resolveFSRootPath resolves path inside root with chroot-like semantics:
// absolute symlink targets are resolved from root, and ".." never leaves root.
// Components that do not exist are kept unchanged so callers can create them.
//
// The path is walked one component at a time, and each component is joined to
// the already-resolved, symlink-free prefix. This keeps os.Lstat and
// os.Readlink from following a symlink in an intermediate component, which
// would hide that symlink from the escape bookkeeping below.
func resolveFSRootPath(root, path string) (fsRootPathResult, error) {
	result := fsRootPathResult{path: root}
	if path == "" {
		return result, nil
	}

	var (
		resolved    string // symlink-free path, relative to root
		linksWalked int    // to protect against cycles
	)

	// todo holds the components that are not resolved yet, in reverse order.
	todo := pushPathComponents(nil, path)
	for len(todo) > 0 {
		component := todo[len(todo)-1]
		todo = todo[:len(todo)-1]

		switch component {
		case ".":
			continue
		case "..":
			// resolved contains no symlink, so its parent is the lexical one.
			if resolved = filepath.Dir(resolved); resolved == "." {
				resolved = ""
			}
			continue
		}

		next := filepath.Join(resolved, component)
		realPath := filepath.Join(root, next)
		fi, err := os.Lstat(realPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return fsRootPathResult{}, err
			}
			// The component does not exist yet; treat it as non-symlink.
			resolved = next
			continue
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			resolved = next
			continue
		}

		if linksWalked++; linksWalked > 255 {
			return fsRootPathResult{}, errTooManyLinks
		}
		target, err := os.Readlink(realPath)
		if err != nil {
			return fsRootPathResult{}, err
		}
		if filepath.IsAbs(target) {
			result.followedAbsoluteLink = true
			resolved = "" // an absolute target is resolved from root
		} else if !result.followedAbsoluteLink {
			// Record an escape before a later absolute link can make the original
			// os.Root error appear eligible for resolve-in-root fallback.
			if joined := filepath.Join(resolved, target); joined != "." && !filepath.IsLocal(joined) {
				result.relativeEscapeBeforeAbsolute = true
			}
		}
		todo = pushPathComponents(todo, target)
	}

	result.path = filepath.Join(root, resolved)
	return result, nil
}

// pushPathComponents appends the components of path to todo in reverse order,
// so that the first component is popped from the end of todo first.
func pushPathComponents(todo []string, path string) []string {
	components := strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == filepath.Separator
	})
	for i := len(components) - 1; i >= 0; i-- {
		todo = append(todo, components[i])
	}
	return todo
}
