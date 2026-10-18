package lazypath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NormalizePath expands a leading "~", makes the path absolute and strips any
// trailing separator, so that the same folder is always stored and looked up
// under one single spelling.
func NormalizePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("folder path is empty")
	}

	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot resolve home directory: %w", err)
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("cannot resolve path %s: %w", path, err)
	}

	return filepath.Clean(abs), nil
}

// Canonical returns one spelling of a path with every symbolic link on it
// resolved, so that two names of the same folder compare equal: on macOS
// /var is a link to /private/var, and git reports the latter where a user
// typed the former.
//
// A path that does not exist yet cannot be resolved as a whole, so the
// longest existing prefix is resolved and the rest is kept as written.
func Canonical(path string) string {
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	parent := filepath.Dir(path)
	if parent == path {
		return path
	}
	return filepath.Join(Canonical(parent), filepath.Base(path))
}

// SamePath reports whether two paths name the same folder, however they
// are spelled.
func SamePath(a, b string) bool {
	return Canonical(a) == Canonical(b)
}
