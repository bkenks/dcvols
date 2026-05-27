package fsops

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// ExpandTilde resolves a leading "~" or "~user" in path to the corresponding
// home directory. A path that does not begin with "~" is returned unchanged.
//
//	~        -> the current user's home (os.UserHomeDir, honoring $HOME)
//	~/x      -> <home>/x
//	~user    -> user's home, looked up in the user database
//	~user/x  -> <user home>/x
//
// Only a tilde at the very start of the path is expanded; a "~" anywhere else is
// left alone, matching shell behavior. The result is cleaned via filepath.Join.
//
// Docker Compose does not itself expand "~" in volume paths — it would create a
// literal "~" directory. dcvols expands it as a convenience so a path like
// "~/data" resolves to the home directory the way a shell would. This is a
// deliberate, documented divergence from Compose.
func ExpandTilde(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}

	// Split the leading element ("~" or "~user") from the remainder.
	lead, rest := path, ""
	if i := strings.IndexByte(path, '/'); i >= 0 {
		lead, rest = path[:i], path[i+1:]
	}

	home, err := homeForLead(lead)
	if err != nil {
		return "", err
	}
	if rest == "" {
		return home, nil
	}
	return filepath.Join(home, rest), nil
}

// homeForLead returns the home directory named by a tilde lead element, where
// lead is either "~" (the current user) or "~name" (a named user).
func homeForLead(lead string) (string, error) {
	if lead == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory: %w", err)
		}
		return home, nil
	}

	name := lead[1:] // strip the leading "~"
	u, err := user.Lookup(name)
	if err != nil {
		return "", fmt.Errorf("looking up user %q: %w", name, err)
	}
	return u.HomeDir, nil
}
