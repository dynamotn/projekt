package folderutil

import (
	"fmt"
	"io"
	"os"
	"sort"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// PruneWorktreeOptions drives `projekt worktree prune`.
type PruneWorktreeOptions struct {
	// DryRun says what would happen without changing anything.
	DryRun bool
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

	worktrees := lazypath.GetConfig().Worktrees

	// One `git worktree prune` per project, not per working tree.
	projects := map[string]string{}
	var gone []lazypath.Worktree
	for _, worktree := range worktrees {
		if folder, err := FindFolder(worktree.Project); err == nil {
			projects[worktree.Project] = folder.Path
		}
		if _, err := os.Stat(worktree.Path); os.IsNotExist(err) {
			gone = append(gone, worktree)
		}
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
		if !IsGitRepo(path) {
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
		_, err := fmt.Fprintln(out, "Nothing to prune")
		return err
	}
	_, err := fmt.Fprintf(out, "%d entr%s forgotten, %d repositor%s pruned\n",
		len(gone), entryPlural(len(gone)), pruned, repoPlural(pruned))
	return err
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

func repoPlural(count int) string {
	if count == 1 {
		return "y"
	}
	return "ies"
}
