// Package hookutil runs nothing: it works out which shell lines a jump should
// evaluate, and prints them for the `pj` function to run in the user's shell.
// A binary cannot change its parent's environment, which is exactly what an
// enter hook is for.
package hookutil

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
	"gitlab.com/dynamo-tools/projekt/pkg/tplutil"
)

// NoHooksEnv, when set to anything, turns every hook off: for a script, or
// for a shell you would rather keep plain for a while.
const NoHooksEnv = "PROJEKT_NO_HOOKS"

// Hook is one line to evaluate, and where it was declared.
type Hook struct {
	Command string
	// From says which declaration it came from, for a listing: the folder
	// entry, or the tag rule.
	From string
	// Source is the configuration file it was read from; "" is this
	// machine's own.
	Source string
}

// Set is what applies to one project.
type Set struct {
	Enter, Leave []Hook
}

// IsEmpty reports whether there is nothing to run either way.
func (s Set) IsEmpty() bool {
	return len(s.Enter) == 0 && len(s.Leave) == 0
}

// ForProject collects the hooks of a project: the folder entry's own, or the
// workspace's for a child or a working tree, then every tag rule the project
// matches, in configuration order.
func ForProject(folder folderutil.ParsedFolder) Set {
	var set Set
	config := lazypath.GetConfig()

	if entry, ok := entryFor(config.Folders, folder); ok && !entry.Hooks.IsEmpty() {
		from := "folder " + entry.Path
		set.Enter = append(set.Enter, hooks(entry.Hooks.Enter, from, entry.Source)...)
		set.Leave = append(set.Leave, hooks(entry.Hooks.Leave, from, entry.Source)...)
	}

	for _, rule := range config.Hooks {
		tags := lazypath.NormalizeTags(rule.Tags)
		if len(tags) == 0 || !lazypath.HasTags(folder.Tags, tags) {
			continue
		}
		from := "tags " + strings.Join(tags, ",")
		set.Enter = append(set.Enter, hooks(rule.Enter, from, rule.Source)...)
		set.Leave = append(set.Leave, hooks(rule.Leave, from, rule.Source)...)
	}

	return set
}

func hooks(lines []string, from, source string) []Hook {
	var result []Hook
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			result = append(result, Hook{Command: line, From: from, Source: source})
		}
	}
	return result
}

// entryFor finds the configuration entry a project's hooks come from: the
// deepest entry whose path holds the project, which is the project's own
// entry, or the workspace a child or a working tree hangs under.
func entryFor(entries []lazypath.Folder, folder folderutil.ParsedFolder) (lazypath.Folder, bool) {
	var best lazypath.Folder
	var found bool
	for _, candidate := range []string{folder.Path, folder.Workspace} {
		target := lazypath.Canonical(candidate)
		for _, entry := range entries {
			path := lazypath.Canonical(entry.Path)
			if !holds(path, target) {
				continue
			}
			if !found || len(path) > len(lazypath.Canonical(best.Path)) {
				best, found = entry, true
			}
		}
		if found {
			return best, true
		}
	}
	return lazypath.Folder{}, false
}

func holds(folder, path string) bool {
	folder, path = filepath.Clean(folder), filepath.Clean(path)
	return folder == path || strings.HasPrefix(path, folder+string(filepath.Separator))
}

// TransitionOptions drives `projekt folder hooks`.
type TransitionOptions struct {
	// From is the folder being left, or "" when nothing is being left.
	From string
	// To is the folder being entered.
	To string
}

// Transition writes the shell lines a jump from one folder to another should
// evaluate: the leave hooks of the project being left, then the enter hooks
// of the one being entered. Staying in the same project runs nothing, and a
// folder that is in no project has no hooks.
//
// A line from an included file that sits in a repository is written only
// once that version of the file is trusted; on a terminal it asks first.
// A line refused is reported and left out, so that the jump still happens.
func Transition(out io.Writer, o TransitionOptions) error {
	if os.Getenv(NoHooksEnv) != "" {
		cli.Debug("%s is set, no hooks", NoHooksEnv)
		return nil
	}

	var leaving, entering folderutil.ParsedFolder
	var leavingOK, enteringOK bool
	if strings.TrimSpace(o.From) != "" {
		if folder, err := folderutil.CurrentProject(o.From); err == nil {
			leaving, leavingOK = folder, true
		}
	}
	if strings.TrimSpace(o.To) != "" {
		if folder, err := folderutil.CurrentProject(o.To); err == nil {
			entering, enteringOK = folder, true
		}
	}
	if leavingOK && enteringOK && leaving.ShortName == entering.ShortName {
		cli.Debug("Still in %s, no hooks", entering.ShortName)
		return nil
	}

	var lines []Hook
	if leavingOK {
		lines = append(lines, ForProject(leaving).Leave...)
	}
	if enteringOK {
		lines = append(lines, ForProject(entering).Enter...)
	}

	var errs []error
	for _, hook := range lines {
		if err := authorize(hook); err != nil {
			errs = append(errs, err)
			continue
		}
		if _, err := fmt.Fprintln(out, hook.Command); err != nil {
			return err
		}
	}
	return errors.Join(errs...)
}

// authorize says whether a hook may be handed to the shell. A line from this
// machine's own file is yours; one from an included file goes through the
// same trust as a template's commands.
func authorize(hook Hook) error {
	if hook.Source == "" {
		return nil
	}
	return tplutil.Authorize(sourceOrigin(hook.Source), hook.Command)
}

// sourceOrigin is the trust origin of an included configuration file.
func sourceOrigin(source string) tplutil.Origin {
	return tplutil.Origin{
		Kind:  "configuration file",
		Name:  source,
		Key:   "config:" + source,
		Paths: []string{source},
		Hint:  "projekt config trust " + source,
	}
}

// TrustSource records an included configuration file as trusted as it is now.
func TrustSource(source string) error {
	return tplutil.Trust(sourceOrigin(source))
}

// Show prints the hooks of a project, with where each one comes from.
func Show(out io.Writer, shortName string) error {
	folder, err := folderutil.FindFolder(shortName)
	if err != nil {
		return err
	}
	set := ForProject(folder)
	if set.IsEmpty() {
		_, err := fmt.Fprintf(out, "%s has no hooks\n", shortName)
		return err
	}
	for _, section := range []struct {
		name  string
		hooks []Hook
	}{{"enter", set.Enter}, {"leave", set.Leave}} {
		if len(section.hooks) == 0 {
			continue
		}
		if _, err := fmt.Fprintf(out, "%s:\n", section.name); err != nil {
			return err
		}
		for _, hook := range section.hooks {
			where := hook.From
			if hook.Source != "" {
				where += ", from " + hook.Source
			}
			if _, err := fmt.Fprintf(out, "  %s\t# %s\n", hook.Command, where); err != nil {
				return err
			}
		}
	}
	return nil
}
