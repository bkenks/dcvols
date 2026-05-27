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
// Precedence: the files are merged from the deepest directory up toward the
// root, and each read overwrites keys from the previous one. Because the root is
// read LAST, values defined at the repository root currently OVERRIDE values
// defined in deeper, compose-adjacent .env files.
//
// This is the opposite of the precedence the project README and CLAUDE.md
// describe ("deeper files take precedence"). The behavior is preserved as-is so
// that splitting main.go into packages stays behavior-neutral; see the BUG note
// below for the one-line fix.
func Load(dir string) (map[string]string, error) {
	env := make(map[string]string)

	// dirs is ordered root-first; iterating from the end reads the deepest
	// directory first and the root last, so root values win on conflict.
	dirs := ancestorsToRoot(dir)
	for i := len(dirs) - 1; i >= 0; i-- {
		envPath := filepath.Join(dirs[i], ".env")
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

// BUG(envfile): Load merges root-level .env values LAST, so they override the
// deeper, compose-adjacent .env files instead of the other way around. The
// README and CLAUDE.md both state that deeper files should take precedence. To
// match the documented intent, iterate dirs from index 0 (root) up to the
// deepest directory — i.e. reverse the loop — so the deepest .env is read last
// and wins on conflict.

// Expand replaces $VAR and ${VAR} references in data, looking each name up first
// in env and then, as a fallback, in the process environment via os.Getenv. A
// name found in neither expands to the empty string, matching os.Expand (and
// Docker Compose's own behavior for undefined variables).
func Expand(data []byte, env map[string]string) []byte {
	return []byte(os.Expand(string(data), func(key string) string {
		if val, ok := env[key]; ok {
			return val
		}
		return os.Getenv(key)
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
