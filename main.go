// Command dcvols pre-creates the host-side bind-mount directories declared in
// Docker Compose files, so they exist (and are owned by the right user) before
// `docker compose up` would otherwise create them as root.
//
// Usage:
//
//	dcvols [flags] [path]
//
// With no path argument it operates on the current directory. Run `dcvols -h`
// or see the project README for the full flag reference.
//
// This file is intentionally thin: it parses flags and orchestrates the run,
// delegating the real work to three internal packages, one per concern:
//
//   - internal/compose — find compose files, parse them, extract bind mounts
//   - internal/envfile — load .env files and expand ${VAR} references
//   - internal/fsops   — create directories/files and chown what was created
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bkenks/dcvols/internal/compose"
	"github.com/bkenks/dcvols/internal/envfile"
	"github.com/bkenks/dcvols/internal/fsops"
)

// config holds the parsed command-line options for a single dcvols run.
type config struct {
	uid       int  // chown target user id; a negative value disables chown
	gid       int  // chown target group id; a negative value disables chown
	recursive bool // search subdirectories for compose files
	dryRun    bool // print what would be created without touching disk
}

func main() {
	var cfg config
	flag.IntVar(&cfg.uid, "uid", -1, "User ID to chown created directories to")
	flag.IntVar(&cfg.gid, "gid", -1, "Group ID to chown created directories to")
	flag.BoolVar(&cfg.recursive, "r", false, "Recursively search subdirectories for compose files")
	flag.BoolVar(&cfg.dryRun, "dry-run", false, "Print directories that would be created without creating them")
	flag.Parse()

	searchPath := "."
	if flag.NArg() > 0 {
		searchPath = flag.Arg(0)
	}

	if err := run(searchPath, cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run finds the compose files under searchPath and processes each in turn,
// stopping at and returning the first error encountered (fail-fast).
func run(searchPath string, cfg config) error {
	composeFiles, err := compose.Find(searchPath, cfg.recursive)
	if err != nil {
		return fmt.Errorf("error finding compose files: %w", err)
	}
	if len(composeFiles) == 0 {
		return fmt.Errorf("no compose files found")
	}

	for _, path := range composeFiles {
		if err := processComposeFile(path, cfg); err != nil {
			return fmt.Errorf("error processing %s: %w", path, err)
		}
	}
	return nil
}

// processComposeFile creates the bind-mount paths declared in a single compose
// file. It loads and expands env vars, parses the file, extracts its bind
// mounts, and — unless cfg.dryRun is set — creates each missing path, printing a
// line for every file/directory created and every chown performed.
func processComposeFile(composePath string, cfg config) error {
	composeDir := filepath.Dir(composePath)

	env, err := envfile.Load(composeDir)
	if err != nil {
		return fmt.Errorf("loading env: %w", err)
	}

	data, err := os.ReadFile(composePath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	file, err := compose.Parse(envfile.Expand(data, env))
	if err != nil {
		return err
	}

	mounts := file.BindMounts()
	if len(mounts) == 0 {
		fmt.Printf("%s: no bind mounts found\n", composePath)
		return nil
	}

	fmt.Printf("%s:\n", composePath)
	for _, m := range mounts {
		if cfg.dryRun {
			fmt.Printf("  (dry-run) %s [%s]\n", m.Path, kind(m.IsFile))
			continue
		}
		if err := create(m, cfg); err != nil {
			return fmt.Errorf("creating %s: %w", m.Path, err)
		}
	}
	return nil
}

// create ensures a single bind mount's host path exists on disk, printing the
// "created" and "chowned" lines dcvols has always emitted. Files and directories
// differ only in which fsops helper does the work.
func create(m compose.BindMount, cfg config) error {
	ensure := fsops.EnsureDir
	if m.IsFile {
		ensure = fsops.EnsureFile
	}

	out, err := ensure(m.Path, cfg.uid, cfg.gid)
	if err != nil {
		return err
	}
	if out.Created {
		fmt.Printf("  created %s\n", m.Path)
	}
	if out.ChownedRoot != "" {
		fmt.Printf("  chowned %s to %d:%d\n", out.ChownedRoot, cfg.uid, cfg.gid)
	}
	return nil
}

// kind returns the label dcvols uses for a bind mount in --dry-run output.
func kind(isFile bool) string {
	if isFile {
		return "file"
	}
	return "dir"
}
