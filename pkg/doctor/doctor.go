// Package doctor answers the question a fresh machine raises: is any of this
// going to work.
//
// It checks the things projekt needs and cannot do without — git, a readable
// configuration, the shell integration, the stores the other commands read —
// and says which of them is missing, rather than leaving it to be discovered
// one failed command at a time.
package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gitlab.com/dynamo.foss/projekt/pkg/bplutil"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
	"gitlab.com/dynamo.foss/projekt/pkg/templates"
	"gitlab.com/dynamo.foss/projekt/pkg/tplutil"
)

// Status is how a check came out.
type Status string

const (
	// StatusOK means there is nothing to do about it.
	StatusOK Status = "ok"
	// StatusWarn means it works, but something is worth knowing.
	StatusWarn Status = "warn"
	// StatusFail means something will not work until it is fixed.
	StatusFail Status = "fail"
)

// Check is one thing that was looked at.
type Check struct {
	// Name is what was checked.
	Name string
	// Status is how it came out.
	Status Status
	// Detail says what was found, and for a warning or a failure, what to do.
	Detail string
}

// Run performs every check, in the order they are worth reading.
func Run() []Check {
	return []Check{
		checkGit(),
		checkBinaries(),
		checkShell(),
		checkConfig(),
		checkFolders(),
		checkTemplates(),
		checkBoilerplates(),
	}
}

// checkGit looks for git, without which cloning, syncing and working trees are
// all out.
func checkGit() Check {
	path, err := exec.LookPath("git")
	if err != nil {
		return Check{"git", StatusFail, "not on PATH; cloning, sync and working trees need it"}
	}

	version, err := exec.Command("git", "--version").Output()
	if err != nil {
		return Check{"git", StatusWarn, fmt.Sprintf("%s does not run: %v", path, err)}
	}
	return Check{"git", StatusOK, strings.TrimSpace(string(version))}
}

// checkBinaries looks for the commands `make install` puts on PATH.
func checkBinaries() Check {
	var missing []string
	for _, name := range []string{"projekt", "t", "b"} {
		if _, err := exec.LookPath(name); err != nil {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return Check{"binaries", StatusWarn, fmt.Sprintf("not on PATH: %s; `make install` puts them in ~/.local/bin",
			strings.Join(missing, ", "))}
	}
	return Check{"binaries", StatusOK, "projekt, t and b are on PATH"}
}

// shellFiles are where the integration is usually sourced from.
var shellFiles = map[string][]string{
	"bash": {".bashrc", ".bash_profile", ".profile"},
	"zsh":  {".zshrc", ".zprofile"},
	"fish": {".config/fish/config.fish"},
}

// checkShell looks for the integration in the usual startup files.
//
// It cannot ask the shell whether `pj` is defined — a child process has no way
// to — so it reads the files that would have defined it. A setup that sources
// it from somewhere else reads as missing, which is the one false alarm here.
func checkShell() Check {
	home, err := os.UserHomeDir()
	if err != nil {
		return Check{"shell integration", StatusWarn, "cannot find your home directory to look in"}
	}

	var found []string
	for _, shell := range templates.Shells() {
		for _, name := range shellFiles[shell] {
			data, err := os.ReadFile(filepath.Join(home, name))
			if err != nil {
				continue
			}
			if strings.Contains(string(data), "projekt init") {
				found = append(found, shell)
				break
			}
		}
	}

	if len(found) == 0 {
		return Check{"shell integration", StatusWarn,
			"no `projekt init` found in your startup files; `pj` is a shell function and has to be sourced"}
	}
	return Check{"shell integration", StatusOK, "sourced for " + strings.Join(found, ", ")}
}

// checkConfig reads the configuration and counts what it says.
func checkConfig() Check {
	path := lazypath.ConfigFile()

	if err := lazypath.LoadError(); err != nil {
		return Check{"configuration", StatusFail, fmt.Sprintf("%s cannot be read: %v", path, err)}
	}

	config := lazypath.GetConfig()
	var errors, warnings int
	for _, diag := range config.Diagnose() {
		if diag.Severity == lazypath.SeverityError {
			errors++
			continue
		}
		warnings++
	}

	summary := fmt.Sprintf("%s: %d folder(s), %d worktree(s), %d git server(s)",
		path, len(config.Folders), len(config.Worktrees), len(config.GitServers))

	switch {
	case errors > 0:
		return Check{"configuration", StatusFail,
			fmt.Sprintf("%s, %d error(s); run `projekt config check`", summary, errors)}
	case warnings > 0:
		return Check{"configuration", StatusWarn,
			fmt.Sprintf("%s, %d warning(s); run `projekt config check`", summary, warnings)}
	case len(config.Folders) == 0:
		return Check{"configuration", StatusWarn,
			summary + "; nothing to jump to yet, start with `projekt folder add`"}
	default:
		return Check{"configuration", StatusOK, summary}
	}
}

// checkFolders looks for entries whose folder is gone.
func checkFolders() Check {
	if err := lazypath.LoadError(); err != nil {
		return Check{"folders on disk", StatusWarn, "not checked: the configuration cannot be read"}
	}

	var missing []string
	for _, folder := range lazypath.GetConfig().Folders {
		if _, err := os.Stat(folder.Path); os.IsNotExist(err) {
			missing = append(missing, folder.Path)
		}
	}
	for _, worktree := range lazypath.GetConfig().Worktrees {
		if _, err := os.Stat(worktree.Path); os.IsNotExist(err) {
			missing = append(missing, worktree.Path)
		}
	}

	if len(missing) > 0 {
		return Check{"folders on disk", StatusWarn,
			fmt.Sprintf("%d configured path(s) are gone, starting with %s", len(missing), missing[0])}
	}
	return Check{"folders on disk", StatusOK, "every configured path is there"}
}

// checkTemplates counts the template store.
func checkTemplates() Check {
	dir, err := tplutil.Dir()
	if err != nil {
		return Check{"templates", StatusWarn, err.Error()}
	}

	list, err := tplutil.List()
	if err != nil {
		return Check{"templates", StatusWarn, fmt.Sprintf("%s: %v", dir, err)}
	}
	if len(list) == 0 {
		return Check{"templates", StatusWarn, dir + " is empty; `t add` puts something in it"}
	}
	return Check{"templates", StatusOK, fmt.Sprintf("%d in %s", len(list), dir)}
}

// checkBoilerplates counts the boilerplate store, and the templates it needs.
func checkBoilerplates() Check {
	dir, err := bplutil.Dir()
	if err != nil {
		return Check{"boilerplates", StatusWarn, err.Error()}
	}

	recipes, err := bplutil.List()
	if err != nil {
		return Check{"boilerplates", StatusWarn, fmt.Sprintf("%s: %v", dir, err)}
	}
	if len(recipes) == 0 {
		return Check{"boilerplates", StatusWarn, dir + " is empty; a recipe is one YAML file"}
	}

	// A recipe pointing at a template that was renamed fails only when it is
	// used, which is the wrong moment to find out.
	var broken []string
	for _, recipe := range recipes {
		if recipe.Source.Template == "" {
			continue
		}
		if _, err := tplutil.Get(recipe.Source.Template); err != nil {
			broken = append(broken, recipe.Name)
		}
	}
	if len(broken) > 0 {
		return Check{"boilerplates", StatusFail,
			fmt.Sprintf("%d in %s, but %s creates from a template that is not there",
				len(recipes), dir, strings.Join(broken, ", "))}
	}

	return Check{"boilerplates", StatusOK, fmt.Sprintf("%d in %s", len(recipes), dir)}
}

// Worst returns the most serious status among the checks.
func Worst(checks []Check) Status {
	worst := StatusOK
	for _, check := range checks {
		switch check.Status {
		case StatusFail:
			return StatusFail
		case StatusWarn:
			worst = StatusWarn
		}
	}
	return worst
}
