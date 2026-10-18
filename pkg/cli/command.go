package cli

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// tools are the programs projekt runs on its own account. Anything else — an
// editor, a picker, a template's hook — is named by the user and runs through
// its own code path; this one only ever starts a program from this list.
var tools = map[string]bool{"git": true, "cp": true}

// Command builds the command for one of projekt's tools, resolved to the
// absolute path it has on PATH before anything runs.
//
// Every tool projekt drives is the user's own, found where their shell finds
// it, so PATH is the right place to look. What this adds is that the lookup
// happens in one place, is never answered with a relative path (a `./git`
// planted in the folder being worked on), and only a known tool is started.
// A tool that cannot be found makes the returned command fail when it is run,
// with the lookup's error.
func Command(name string, args ...string) *exec.Cmd {
	if !tools[name] {
		return failed(name, args, fmt.Errorf("%s is not a tool projekt runs", name))
	}
	resolved, err := exec.LookPath(name)
	if err != nil {
		return failed(name, args, err)
	}
	if !filepath.IsAbs(resolved) {
		return failed(name, args, fmt.Errorf("%s resolves to the relative path %s", name, resolved))
	}
	// The program is one of the fixed tools above, at the absolute path PATH
	// gives it, and the arguments are passed as argv, never through a shell.
	return exec.Command(resolved, args...) // nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
}

// failed returns a command that names no program at all and only reports err
// when it is started: exec.Cmd returns Err before it would look at anything
// else.
func failed(name string, _ []string, err error) *exec.Cmd {
	return &exec.Cmd{Err: fmt.Errorf("cannot run %s: %w", name, err)}
}
