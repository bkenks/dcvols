// Package envfile loads the .env files that apply to a compose directory and
// expands ${VAR} / $VAR references found in compose file contents, mirroring how
// Docker Compose interpolates variables before a stack starts.
package envfile

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// Load reads the .env files that apply to a compose directory and returns their
// merged variables.
//
// Starting at dir, Load walks UP the directory tree to the repository root — the
// first ancestor containing a .git entry, or the filesystem root if none is
// found — and reads a ".env" file from each level that has one. Directories
// without a .env file are simply skipped.
//
// Precedence: the files are merged from the repository root down toward dir, and
// each read overwrites keys from the previous one. Because the deepest
// (compose-adjacent) directory is read LAST, its values WIN on conflict. This is
// the "most specific wins" model: a root .env supplies shared defaults that a
// stack-specific .env next to the compose file can override.
//
// Note that Docker Compose itself does not walk the tree — it reads a single
// .env from the project directory. The walk-up is a dcvols convenience for
// monorepos with a shared root .env; keeping the deepest file authoritative
// preserves parity with Compose whenever the relevant variable is defined there.
// The live shell environment still overrides everything here; see Expand.
func Load(dir string) (map[string]string, error) {
	env := make(map[string]string)

	// dirs is ordered root-first; iterating forward reads the root .env first
	// and the deepest .env last, so the deepest (most specific) values win.
	dirs := ancestorsToRoot(dir)
	for _, d := range dirs {
		envPath := filepath.Join(d, ".env")
		if _, err := os.Stat(envPath); err != nil {
			continue
		}
		parsed, err := godotenv.Read(envPath)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", envPath, err)
		}
		for k, v := range parsed {
			env[k] = v
		}
	}

	return env, nil
}

// Expand replaces $VAR and ${VAR} references in data, looking each name up first
// in the process environment (os.LookupEnv) and then, as a fallback, in the
// merged .env map. A name found in neither expands to the empty string, matching
// os.Expand.
//
// Resolution order — shell environment over .env — mirrors Docker Compose, which
// lets values from the shell or command line override those in a .env file.
// LookupEnv (rather than Getenv) is used so that a variable explicitly set in
// the shell wins even when set to the empty string, matching Compose's
// set-versus-unset semantics.
func Expand(data []byte, env map[string]string) []byte {
	return []byte(os.Expand(string(data), func(key string) string {
		if val, ok := os.LookupEnv(key); ok {
			return val
		}
		return env[key]
	}))
}

// ancestorsToRoot returns the chain of directories from the repository root down
// to dir (inclusive), ordered root-first.
//
// The repository root is the first directory, walking up from dir, that contains
// a .git entry; if none is found the walk stops at the filesystem root. dir is
// resolved to an absolute path first; if that resolution fails, a single-element
// slice holding the original dir is returned as a best effort.
func ancestorsToRoot(dir string) []string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return []string{dir}
	}

	var dirs []string
	current := abs
	for {
		// Prepend so the slice stays ordered root-first as we climb upward.
		dirs = append([]string{current}, dirs...)
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			break // reached the filesystem root
		}
		current = parent
	}
	return dirs
}
