// Package compose reads the narrow slice of Docker Compose files that dcvols
// cares about: where to find them on disk, how to parse them, and which of a
// service's volumes are bind mounts whose host paths must exist before the
// stack starts.
//
// The package deliberately models only a fragment of the Compose
// specification — the services map and each service's volumes list. Every other
// part of a compose file (networks, build, environment, healthchecks, ...) is
// ignored by the YAML decoder.
package compose

import (
	"os"
	"path/filepath"
)

// FileNames are the compose file names Docker Compose recognizes, listed in the
// order Compose itself prefers them. Find consults this list when searching a
// directory and, in the non-recursive case, returns the first one that exists.
var FileNames = []string{
	"compose.yaml",
	"compose.yml",
	"docker-compose.yaml",
	"docker-compose.yml",
}

// Find locates Docker Compose files to process.
//
// When recursive is false it looks only in root and returns the first
// recognized file name that exists there (honoring Compose's own precedence via
// FileNames), or an empty slice when none exist. It never returns an error in
// this mode.
//
// When recursive is true it walks the entire tree rooted at root and returns
// every recognized compose file, skipping any .git directory it encounters.
// Paths come back in filepath.WalkDir's lexical traversal order. Any error from
// walking the tree is returned alongside whatever was found so far.
func Find(root string, recursive bool) ([]string, error) {
	if recursive {
		return findRecursive(root)
	}
	return findShallow(root), nil
}

// findShallow returns the single highest-precedence compose file directly inside
// root, or nil when none of the recognized names exist there.
func findShallow(root string) []string {
	for _, name := range FileNames {
		path := filepath.Join(root, name)
		if _, err := os.Stat(path); err == nil {
			return []string{path}
		}
	}
	return nil
}

// findRecursive walks the tree under root and collects every recognized compose
// file, pruning .git directories so repository metadata is never scanned.
func findRecursive(root string) ([]string, error) {
	var found []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if !d.IsDir() && isComposeFileName(d.Name()) {
			found = append(found, path)
		}
		return nil
	})
	return found, err
}

// isComposeFileName reports whether name is one of the recognized compose file
// names in FileNames.
func isComposeFileName(name string) bool {
	for _, n := range FileNames {
		if name == n {
			return true
		}
	}
	return false
}
