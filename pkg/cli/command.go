package cli

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// tools are the programs projekt runs on its own account. Anything else — an
// editor, a picker — is named by the user and runs through its own code path;
// this one only ever starts a program from this list. sh runs the `after:`
// lines of templates and recipes, found on PATH the way `/usr/bin/env sh`
// would find it: a static binary cannot rely on /usr/bin/env, which Termux,
// for one, does not have.
var tools = map[string]bool{"git": true, "cp": true, "sh": true}

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

// WithoutRepoEnv drops the variables that point git at a repository other
// than the folder it is run in. Inherited from a git hook, an alias or
// `git rebase --exec`, GIT_DIR would answer for that repository: one with no
// remote would vouch for a cloned store, and one that ignores nothing would
// let a git-ignored file into a prompt.
func WithoutRepoEnv(env []string) []string {
	kept := make([]string, 0, len(env))
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		switch name {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE",
			"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES",
			"GIT_CEILING_DIRECTORIES", "GIT_DISCOVERY_ACROSS_FILESYSTEM", "GIT_NAMESPACE":
			continue
		}
		kept = append(kept, entry)
	}
	return kept
}
