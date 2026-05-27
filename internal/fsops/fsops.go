// Package fsops turns the bind-mount paths dcvols extracts into real files and
// directories on disk. It resolves home-relative (~) paths, creates missing
// directories and files, and optionally chowns only the parts of the tree that
// were newly created — never pre-existing parents.
package fsops

import (
	"fmt"
	"os"
	"path/filepath"
)

// Outcome reports what an Ensure call did, so the caller can decide what to
// print. fsops deliberately performs no output of its own.
type Outcome struct {
	// Created is true when a creation step was performed for the target path.
	//
	// For files this means the file did not previously exist and was created.
	// For directories it means the MkdirAll step ran; because MkdirAll is a
	// no-op when the directory already exists, Created can be true even for a
	// directory that was already present. This mirrors dcvols's long-standing
	// output, which prints "created" for already-present directories.
	Created bool

	// ChownedRoot is the topmost newly-created ancestor that was chowned, or ""
	// when nothing was chowned — either because ownership changes were disabled
	// or because nothing new was actually created.
	ChownedRoot string
}

// EnsureDir makes sure the directory at path exists, creating it and any missing
// parents when necessary.
//
// If path already exists but is NOT a directory, EnsureDir leaves it untouched
// and returns a zero Outcome: dcvols never clobbers an existing file by turning
// it into a directory.
//
// When uid and gid are both >= 0, only the newly created portion of the tree is
// chowned — the topmost ancestor that did not previously exist, and everything
// beneath it. Pre-existing parent directories are left alone. When nothing new
// was created there is nothing to chown.
func EnsureDir(path string, uid, gid int) (Outcome, error) {
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		// Already exists as a non-directory — leave it as-is.
		return Outcome{}, nil
	}

	root := firstMissingAncestor(path)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return Outcome{}, err
	}
	return chownNew(Outcome{Created: true}, root, uid, gid)
}

// EnsureFile makes sure an empty file exists at path, creating any missing parent
// directories first.
//
// If path already exists (as anything at all) EnsureFile does nothing and
// returns a zero Outcome. Otherwise it creates the parent directories and an
// empty file, then chowns the newly created portion of the tree under the same
// rules as EnsureDir.
func EnsureFile(path string, uid, gid int) (Outcome, error) {
	if _, err := os.Stat(path); err == nil {
		// Already exists — leave it as-is.
		return Outcome{}, nil
	}

	root := firstMissingAncestor(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Outcome{}, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return Outcome{}, err
	}
	if err := f.Close(); err != nil {
		return Outcome{Created: true}, err
	}
	return chownNew(Outcome{Created: true}, root, uid, gid)
}

// chownNew chowns the newly-created subtree rooted at root when ownership changes
// are requested, recording the chowned root in the returned Outcome.
//
// A chown happens only when uid and gid are both >= 0 and root is non-empty
// (root is "" when nothing new was created). The chown error, if any, is wrapped
// with a "chown <root>" prefix.
func chownNew(out Outcome, root string, uid, gid int) (Outcome, error) {
	if uid < 0 || gid < 0 || root == "" {
		return out, nil
	}
	if err := chownTree(root, uid, gid); err != nil {
		return out, fmt.Errorf("chown %s: %w", root, err)
	}
	out.ChownedRoot = root
	return out, nil
}

// firstMissingAncestor returns the topmost ancestor of path that does not yet
// exist — the directory MkdirAll will create first. Chowning from this point
// downward touches exactly the tree dcvols created and nothing that already
// existed.
//
// path is resolved to an absolute path; if that fails the original path is
// returned as a best effort. If path itself already exists, the result is "".
func firstMissingAncestor(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	current := abs
	result := ""
	for {
		if _, err := os.Stat(current); os.IsNotExist(err) {
			result = current
		} else {
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			break // reached the filesystem root
		}
		current = parent
	}
	return result
}

// chownTree recursively chowns root and everything beneath it to uid:gid.
func chownTree(root string, uid, gid int) error {
	return filepath.WalkDir(root, func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(path, uid, gid)
	})
}
