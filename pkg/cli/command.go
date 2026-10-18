package cli

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// Command builds the command for a tool such as git, resolved to the absolute
// path it has on PATH before anything runs.
//
// Every tool projekt drives — git, cp — is the user's own, found where their
// shell finds it, so PATH is the right place to look. What this adds is that
// the lookup happens in one place, is never answered with a relative path
// (a `./git` planted in the folder being worked on), and the resolved path is
// what a debug log shows. A tool that cannot be found makes the returned
// command fail when it is run, with the lookup's error.
func Command(name string, args ...string) *exec.Cmd {
	resolved, err := exec.LookPath(name)
	if err == nil && !filepath.IsAbs(resolved) {
		err = fmt.Errorf("%s resolves to the relative path %s", name, resolved)
	}
	if err != nil {
		cmd := exec.Command(name, args...)
		cmd.Err = fmt.Errorf("cannot run %s: %w", name, err)
		return cmd
	}
	return exec.Command(resolved, args...)
}
