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
	Volumes []volumeEntry `yaml:"volumes"`
}

// volumeEntry handles both the short form ("host:container") and the long form
// (map with type/source/target keys).
type volumeEntry struct {
	raw    string // set when short-form string
	Type   string `yaml:"type"`
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}

func (v *volumeEntry) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		v.raw = value.Value
		return nil
	}
	type alias volumeEntry
	return value.Decode((*alias)(v))
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

	mounts := extractBindMounts(cf)
	if len(mounts) == 0 {
		fmt.Printf("%s: no bind mounts found\n", composePath)
		return nil
	}

	fmt.Printf("%s:\n", composePath)
	for _, m := range mounts {
		if dryRun {
			kind := "dir"
			if m.isFile {
				kind = "file"
			}
			fmt.Printf("  (dry-run) %s [%s]\n", m.path, kind)
			continue
		}
		if m.isFile {
			if err := ensureFile(m.path, uid, gid); err != nil {
				return fmt.Errorf("creating %s: %w", m.path, err)
			}
		} else {
			if info, err := os.Stat(m.path); err == nil && !info.IsDir() {
				// already exists as a non-directory — skip
				continue
			}
			newRoot := firstMissingAncestor(m.path)
			if err := os.MkdirAll(m.path, 0755); err != nil {
				return fmt.Errorf("creating %s: %w", m.path, err)
			}
			fmt.Printf("  created %s\n", m.path)
			if uid >= 0 && gid >= 0 && newRoot != "" {
				if err := chownTree(newRoot, uid, gid); err != nil {
					return fmt.Errorf("chown %s: %w", newRoot, err)
				}
				fmt.Printf("  chowned %s to %d:%d\n", newRoot, uid, gid)
			}
		}
	}

	return nil
}

func ensureFile(path string, uid, gid int) error {
	if _, err := os.Stat(path); err == nil {
		// already exists — skip
		return nil
	}
	newRoot := firstMissingAncestor(path)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	f.Close()
	fmt.Printf("  created %s\n", path)
	if uid >= 0 && gid >= 0 && newRoot != "" {
		if err := chownTree(newRoot, uid, gid); err != nil {
			return fmt.Errorf("chown %s: %w", newRoot, err)
		}
		fmt.Printf("  chowned %s to %d:%d\n", newRoot, uid, gid)
	}
	return nil
}

type bindMount struct {
	path   string
	isFile bool
}

func extractBindMounts(cf composeFile) []bindMount {
	seen := make(map[string]bool)
	var mounts []bindMount

	for _, svc := range cf.Services {
		for _, vol := range svc.Volumes {
			var hostPath string
			if vol.raw != "" {
				// Short form: "host:container" or "host:container:options"
				parts := strings.SplitN(vol.raw, ":", 2)
				hostPath = parts[0]
			} else {
				// Long form: only bind mounts have a host path
				if vol.Type != "bind" {
					continue
				}
				hostPath = vol.Source
			}

			if !isBindMount(hostPath) {
				continue
			}

			hostPath = filepath.Clean(hostPath)
			if !seen[hostPath] {
				seen[hostPath] = true
				mounts = append(mounts, bindMount{
					path:   hostPath,
					isFile: filepath.Ext(hostPath) != "",
				})
			}
		}
	}

	return mounts
}

// firstMissingAncestor returns the topmost path component that does not yet exist,
// so we know where to start chowning after MkdirAll creates the tree.
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
			break
		}
		current = parent
	}
	return result
}

// chownTree recursively chowns all files and directories under root.
func chownTree(root string, uid, gid int) error {
	return filepath.WalkDir(root, func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(path, uid, gid)
	})
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
