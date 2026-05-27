package compose

import (
	"path/filepath"
	"strings"
)

// BindMount is a host filesystem path that dcvols may need to create before a
// stack starts.
type BindMount struct {
	// Path is the cleaned host path (filepath.Clean has been applied).
	Path string
	// IsFile is true when the path looks like a file rather than a directory.
	// dcvols infers this purely from the path's extension — see BindMounts for
	// the heuristic and its limits.
	IsFile bool
}

// BindMounts returns the deduplicated bind-mount host paths declared across all
// services in f, in first-seen order.
//
// Named volumes (e.g. "pgdata:/var/lib/postgresql/data") are skipped: they are
// managed by Docker and not backed by a host path dcvols needs to create. Only
// entries that look like bind mounts — host paths that are absolute or
// explicitly relative, per isBindMount — are included.
//
// Whether each path is treated as a file or a directory is inferred from its
// extension: "config.yml" is a file, "data" is a directory. This is a heuristic.
// It misclassifies an extensionless file, or a directory whose name contains a
// dot, but it matches the overwhelmingly common case and lets dcvols decide
// without touching the filesystem.
//
// The outer iteration is over the services map, so the relative order of mounts
// from different services is not deterministic. Deduplication (first occurrence
// wins) and the resulting set are stable regardless of that order.
func (f File) BindMounts() []BindMount {
	seen := make(map[string]bool)
	var mounts []BindMount

	for _, svc := range f.Services {
		for _, vol := range svc.Volumes {
			hostPath, ok := vol.hostPath()
			if !ok {
				continue
			}
			if !isBindMount(hostPath) {
				continue
			}

			hostPath = filepath.Clean(hostPath)
			if seen[hostPath] {
				continue
			}
			seen[hostPath] = true
			mounts = append(mounts, BindMount{
				Path:   hostPath,
				IsFile: filepath.Ext(hostPath) != "",
			})
		}
	}

	return mounts
}

// hostPath extracts the host-side path from a volume entry and reports whether
// the entry has one at all.
//
// For short-form entries the host path is the segment before the first colon in
// "HOST:CONTAINER[:OPTS]". For long-form entries only bind mounts carry a host
// path (in Source); volume, tmpfs, and other types return ok=false so callers
// skip them.
func (v VolumeEntry) hostPath() (string, bool) {
	if v.raw != "" {
		// Short form: "host:container" or "host:container:options". The host
		// path is everything up to the first colon.
		parts := strings.SplitN(v.raw, ":", 2)
		return parts[0], true
	}
	if v.Type != "bind" {
		return "", false
	}
	return v.Source, true
}

// isBindMount reports whether a host path denotes a bind mount rather than a
// named volume. Named volumes are bare names like "pgdata"; bind mounts are
// paths that are absolute ("/srv"), explicitly relative ("./data", "../data"),
// or home-relative ("~/data").
func isBindMount(hostPath string) bool {
	return strings.HasPrefix(hostPath, "/") ||
		strings.HasPrefix(hostPath, "./") ||
		strings.HasPrefix(hostPath, "../") ||
		strings.HasPrefix(hostPath, "~")
}
