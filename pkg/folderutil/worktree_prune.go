package folderutil

import (
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strings"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// PruneWorktreeOptions drives `projekt worktree prune`.
type PruneWorktreeOptions struct {
	// DryRun says what would happen without changing anything.
	DryRun bool
	// Merged also removes the working trees whose branch is merged into the
	// project's default branch and that hold no change or untracked file.
	Merged bool
}

// PruneWorktrees forgets the working trees whose folder is gone, on both
// sides: the configuration entry, and git's own record of it.
//
// A working tree removed with `rm -rf` leaves git still listing it until
// `git worktree prune` runs, and leaves projekt still offering the name.
// Neither half is much use without the other.
func PruneWorktrees(out io.Writer, o PruneWorktreeOptions) error {
	if err := lazypath.LoadError(); err != nil {
		return err
	}

	parsed, err := ParseConfig(lazypath.GetConfig())
	if err != nil {
		return err
	}
	byName := make(map[string]ParsedFolder, len(parsed))
	for _, folder := range parsed {
		byName[folder.ShortName] = folder
	}

	// One `git worktree prune` per project, not per working tree.
	projects := map[string]string{}
	var gone []lazypath.Worktree
	for _, worktree := range lazypath.GetConfig().Worktrees {
		if folder, ok := byName[worktree.Project]; ok {
			projects[worktree.Project] = folder.Path
		}
		if _, err := os.Stat(worktree.Path); !os.IsNotExist(err) {
			continue
		}
		if worktree.Source != "" {
			// An included file is read, never written: say so instead of
			// failing to forget what was never ours.
			if _, err := fmt.Fprintf(out, "%s is gone but comes from an included file; edit that file to forget it\n", worktree.ShortName()); err != nil {
				return err
			}
			continue
		}
		gone = append(gone, worktree)
	}

	if o.DryRun {
		for _, worktree := range gone {
			if _, err := fmt.Fprintf(out, "[DRY RUN] Would forget %s: %s\n", worktree.ShortName(), worktree.Path); err != nil {
				return err
			}
		}
		for _, path := range sortedPaths(projects) {
			if _, err := fmt.Fprintf(out, "[DRY RUN] Would run git worktree prune in %s\n", path); err != nil {
				return err
			}
		}
		if o.Merged {
			return pruneMerged(out, o, parsed)
		}
		return nil
	}

	for _, worktree := range gone {
		if err := lazypath.RemoveWorktreeFromConfig(worktree.Project, worktree.Name); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "Forgot %s\n", worktree.ShortName()); err != nil {
			return err
		}
	}

	var pruned int
	for _, path := range sortedPaths(projects) {
		if !IsRepoRoot(path) {
			continue
		}
		if err := runGit(path, "worktree", "prune"); err != nil {
			// One repository that will not prune is not a reason to stop: the
			// rest still want cleaning, and the failure is worth naming.
			if _, printErr := fmt.Fprintf(out, "Cannot prune %s: %v\n", path, err); printErr != nil {
				return printErr
			}
			continue
		}
		pruned++
	}

	if len(gone) == 0 && pruned == 0 {
		_, err = fmt.Fprintln(out, "Nothing to prune")
	} else {
		_, err = fmt.Fprintf(out, "%d entr%s forgotten, %d repositor%s pruned\n",
			len(gone), entryPlural(len(gone)), pruned, entryPlural(pruned))
	}
	if err != nil || !o.Merged {
		return err
	}
	return pruneMerged(out, o, parsed)
}

// pruneMerged removes the merged, clean working trees of every project and
// names each tree it leaves alone with the reason.
//
// Removal is `git worktree remove` without --force, so git itself refuses a
// tree the checks here misjudged; the branch is kept.
func pruneMerged(out io.Writer, o PruneWorktreeOptions, projects []ParsedFolder) error {
	config := lazypath.GetConfig()
	// Removing an entry shifts the slice the configuration shares; work on
	// a copy.
	worktrees := slices.Clone(config.Worktrees)

	say := func(format string, args ...any) error {
		_, err := fmt.Fprintf(out, format+"\n", args...)
		return err
	}
	removed := 0

	for _, project := range projects {
		// ParseConfig also lists every working tree as a folder; a tree is
		// not a project, and its own listing would repeat the project's.
		if _, _, isTree := lazypath.SplitWorktreeName(project.ShortName); isTree || !IsRepoRoot(project.Path) {
			continue
		}
		listing, err := gitOutput(project.Path, "worktree", "list", "--porcelain")
		if err != nil {
			if err := say("Cannot list the working trees of %s: %v", project.ShortName, err); err != nil {
				return err
			}
			continue
		}
		base := defaultBranch(project.Path)

		declared := map[string]struct{}{}
		for _, folder := range config.Folders {
			if folder.Git != nil && lazypath.SamePath(folder.Path, project.Path) {
				for _, repo := range folder.Git.Repos {
					for _, worktree := range repo.Worktrees {
						declared[lazypath.Canonical(worktreeTargetPath(folder, worktree))] = struct{}{}
					}
				}
			}
		}

		// The first entry is the main checkout, which is never removed.
		for _, path := range listedWorktrees(listing)[1:] {
			var recorded *lazypath.Worktree
			for i, worktree := range worktrees {
				if worktree.Project == project.ShortName && lazypath.SamePath(worktree.Path, path) {
					recorded = &worktrees[i]
					break
				}
			}

			name, reason, configured := path, "", ""
			if recorded == nil {
				reason = "not recorded by projekt"
				if _, ok := declared[lazypath.Canonical(path)]; ok {
					reason = "declared under git.repos"
				}
			} else {
				name, configured = recorded.ShortName(), recorded.Branch
				if recorded.Source != "" {
					reason = "from " + recorded.Source
				}
			}
			if reason == "" {
				reason = mergedBlocker(path, base, configured)
			}
			if reason != "" {
				if err := say("%s: %s", name, reason); err != nil {
					return err
				}
				continue
			}

			if o.DryRun {
				if err := say("[DRY RUN] Would remove %s: %s", name, path); err != nil {
					return err
				}
				continue
			}
			if err := runGit(project.Path, "worktree", "remove", "--", path); err != nil {
				if err := say("%s: cannot remove: %v", name, err); err != nil {
					return err
				}
				continue
			}
			if err := lazypath.RemoveWorktreeFromConfig(recorded.Project, recorded.Name); err != nil {
				return err
			}
			removed++
			if err := say("Removed %s", name); err != nil {
				return err
			}
		}
	}

	if o.DryRun {
		return nil
	}
	return say("merged working trees removed: %d", removed)
}

// listedWorktrees reads the paths out of `git worktree list --porcelain`.
func listedWorktrees(porcelain string) []string {
	var paths []string
	for _, line := range outputLines(porcelain) {
		if path, ok := strings.CutPrefix(line, "worktree "); ok {
			paths = append(paths, path)
		}
	}
	return paths
}

// mergedBlocker says why a working tree must stay, or "" when it holds
// nothing that is not already in the default branch.
//
// configured is the branch the configuration records, "" when it records none.
func mergedBlocker(path, base, configured string) string {
	if base == "" {
		return "default branch unknown"
	}
	out, err := gitOutput(path, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return "detached HEAD"
	}
	branch := strings.TrimSpace(out)
	if configured != "" && branch != configured {
		return fmt.Sprintf("on %s, configured %s", branch, configured)
	}
	if branch == base || "origin/"+branch == base {
		return "default branch"
	}
	if runGit(path, "merge-base", "--is-ancestor", "HEAD", base) != nil {
		return "not merged"
	}
	status, err := gitOutput(path, "status", "--porcelain", "--ignored")
	if err != nil {
		return "cannot read status"
	}
	lines := outputLines(status)
	for _, line := range lines {
		if strings.HasPrefix(line, "??") {
			return "untracked files"
		}
	}
	// `git worktree remove` ignores ignored files too, and would lose them.
	for _, line := range lines {
		if strings.HasPrefix(line, "!!") {
			return "ignored files"
		}
	}
	if len(lines) > 0 {
		return "uncommitted changes"
	}
	return ""
}

// sortedPaths returns the project paths in a stable order.
func sortedPaths(projects map[string]string) []string {
	paths := make([]string, 0, len(projects))
	for _, path := range projects {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func entryPlural(count int) string {
	if count == 1 {
		return "y"
	}
	return "ies"
}
