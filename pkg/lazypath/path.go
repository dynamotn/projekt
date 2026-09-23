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
