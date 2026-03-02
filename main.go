package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

var composeFileNames = []string{
	"compose.yaml",
	"compose.yml",
	"docker-compose.yaml",
	"docker-compose.yml",
}

type composeFile struct {
	Services map[string]service `yaml:"services"`
}

type service struct {
	Volumes []string `yaml:"volumes"`
}

func main() {
	uid := flag.Int("uid", -1, "User ID to chown created directories to")
	gid := flag.Int("gid", -1, "Group ID to chown created directories to")
	recursive := flag.Bool("r", false, "Recursively search subdirectories for compose files")
	dryRun := flag.Bool("dry-run", false, "Print directories that would be created without creating them")
	flag.Parse()

	searchPath := "."
	if flag.NArg() > 0 {
		searchPath = flag.Arg(0)
	}

	composeFiles, err := findComposeFiles(searchPath, *recursive)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error finding compose files: %v\n", err)
		os.Exit(1)
	}

	if len(composeFiles) == 0 {
		fmt.Fprintln(os.Stderr, "no compose files found")
		os.Exit(1)
	}

	for _, composePath := range composeFiles {
		if err := processComposeFile(composePath, *uid, *gid, *dryRun); err != nil {
			fmt.Fprintf(os.Stderr, "error processing %s: %v\n", composePath, err)
			os.Exit(1)
		}
	}
}

func findComposeFiles(root string, recursive bool) ([]string, error) {
	var found []string

	if recursive {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && d.Name() == ".git" {
				return filepath.SkipDir
			}
			if !d.IsDir() {
				for _, name := range composeFileNames {
					if d.Name() == name {
						found = append(found, path)
						break
					}
				}
			}
			return nil
		})
		return found, err
	}

	for _, name := range composeFileNames {
		path := filepath.Join(root, name)
		if _, err := os.Stat(path); err == nil {
			found = append(found, path)
			break
		}
	}
	return found, nil
}

func processComposeFile(composePath string, uid, gid int, dryRun bool) error {
	composeDir := filepath.Dir(composePath)

	env, err := loadEnv(composeDir)
	if err != nil {
		return fmt.Errorf("loading env: %w", err)
	}

	data, err := os.ReadFile(composePath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	expanded := os.Expand(string(data), func(key string) string {
		if val, ok := env[key]; ok {
			return val
		}
		return os.Getenv(key)
	})

	var cf composeFile
	if err := yaml.Unmarshal([]byte(expanded), &cf); err != nil {
		return fmt.Errorf("parsing yaml: %w", err)
	}

	dirs := extractBindMountDirs(cf)
	if len(dirs) == 0 {
		fmt.Printf("%s: no bind mounts found\n", composePath)
		return nil
	}

	fmt.Printf("%s:\n", composePath)
	for _, dir := range dirs {
		if dryRun {
			fmt.Printf("  (dry-run) %s\n", dir)
			continue
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
		fmt.Printf("  created %s\n", dir)
		if uid >= 0 && gid >= 0 {
			if err := os.Chown(dir, uid, gid); err != nil {
				return fmt.Errorf("chown %s: %w", dir, err)
			}
			fmt.Printf("  chowned %s to %d:%d\n", dir, uid, gid)
		}
	}

	return nil
}

func extractBindMountDirs(cf composeFile) []string {
	seen := make(map[string]bool)
	var dirs []string

	for _, svc := range cf.Services {
		for _, vol := range svc.Volumes {
			// volumes can be "host:container" or "host:container:options"
			parts := strings.SplitN(vol, ":", 2)
			hostPath := parts[0]

			if !isBindMount(hostPath) {
				continue
			}

			hostPath = filepath.Clean(hostPath)
			if !seen[hostPath] {
				seen[hostPath] = true
				dirs = append(dirs, hostPath)
			}
		}
	}

	return dirs
}

// isBindMount returns true if the volume entry is a bind mount (has a host path)
// rather than a named volume. Named volumes are just plain names like "pgdata".
func isBindMount(hostPath string) bool {
	return strings.HasPrefix(hostPath, "/") ||
		strings.HasPrefix(hostPath, "./") ||
		strings.HasPrefix(hostPath, "../") ||
		strings.HasPrefix(hostPath, "~")
}

// loadEnv loads a .env file from the compose directory, then walks up to find
// additional .env files (up to the repo root). Variables from deeper files
// take precedence over parent ones.
func loadEnv(composeDir string) (map[string]string, error) {
	env := make(map[string]string)

	// Walk up the directory tree looking for .env files, stopping at the repo root
	dirs := parentDirs(composeDir)
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

// parentDirs returns all directories from root down to dir, stopping when a
// .git directory is found (treating that as the repo root).
func parentDirs(dir string) []string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return []string{dir}
	}

	var dirs []string
	current := abs
	for {
		dirs = append([]string{current}, dirs...)
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return dirs
}
