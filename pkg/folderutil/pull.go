package folderutil

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
)

// What pullRepo did to a working tree.
const (
	// pullUpdated means the branch was fast-forwarded.
	pullUpdated = "updated"
	// pullUpToDate means there was nothing to bring in.
	pullUpToDate = "up to date"
	// pullSkipped means the fetch happened but the tree was left where it was,
	// for the Reason given.
	pullSkipped = "skipped"
)

// pullResult says what happened to one working tree.
type pullResult struct {
	Outcome string
	// Reason says why a tree was skipped.
	Reason string
	// Commits is how many commits an update brought in.
	Commits int
}

// pullRepo fetches a working tree's remotes and fast-forwards its branch onto
// its upstream when that is safe: a clean tree, on a branch, with an upstream,
// strictly behind it. Anything else is fetched and reported as skipped, never
// merged or rebased — a tree with work in it is the user's to sort out.
//
// A fetch that fails is an error; a tree that cannot be moved is not.
func pullRepo(path string, batch bool) (pullResult, error) {
	if err := fetch(path, batch); err != nil {
		return pullResult{}, fmt.Errorf("fetch: %w", err)
	}

	if reason := pullBlocker(path); reason != "" {
		return pullResult{Outcome: pullSkipped, Reason: reason}, nil
	}

	behind, err := revCount(path, "HEAD..@{u}")
	if err != nil {
		return pullResult{}, fmt.Errorf("compare with upstream: %w", err)
	}
	if behind == 0 {
		return pullResult{Outcome: pullUpToDate}, nil
	}
	ahead, err := revCount(path, "@{u}..HEAD")
	if err != nil {
		return pullResult{}, fmt.Errorf("compare with upstream: %w", err)
	}
	if ahead > 0 {
		return pullResult{Outcome: pullSkipped, Reason: "diverged"}, nil
	}

	if err := runGit(path, "merge", "--ff-only", "--quiet", "@{u}"); err != nil {
		return pullResult{}, fmt.Errorf("fast-forward: %w", err)
	}
	return pullResult{Outcome: pullUpdated, Commits: behind}, nil
}

// pullBlocker names what stops a tree from being fast-forwarded, or returns
// "" when nothing does. The cheap checks come first.
func pullBlocker(path string) string {
	if runGit(path, "symbolic-ref", "--quiet", "HEAD") != nil {
		return "detached HEAD"
	}
	if runGit(path, "rev-parse", "--verify", "--quiet", "@{u}") != nil {
		return "no upstream"
	}
	status, err := gitOutput(path, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return "cannot read status"
	}
	if strings.TrimSpace(status) != "" {
		return "uncommitted changes"
	}
	return ""
}

// fetch brings every remote up to date, pruning what is gone. In a batch no
// question can be answered, so git is told not to ask one.
func fetch(path string, batch bool) error {
	cmd := exec.Command("git", "-C", path, "fetch", "--all", "--prune", "--quiet")
	if batch {
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_SSH_COMMAND=ssh -oBatchMode=yes")
	}

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Run(); err != nil {
		if text := strings.TrimSpace(output.String()); text != "" {
			return fmt.Errorf("%w: %s", err, text)
		}
		return err
	}
	return nil
}

// revCount counts the commits in a range.
func revCount(path, rangeSpec string) (int, error) {
	out, err := gitOutput(path, "rev-list", "--count", rangeSpec)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(out))
}

// pullTrees pulls a repository and each of its configured working trees that
// is on disk, reporting each one by name.
func pullTrees(job syncJob, dryRun bool) error {
	trees := []struct{ name, path string }{{job.repo.Name, job.repoPath}}
	for i, worktree := range job.repo.Worktrees {
		path := job.worktreePaths[i]
		if _, err := os.Stat(path); err == nil {
			trees = append(trees, struct{ name, path string }{job.repo.Name + "@" + worktree.Branch, path})
		}
	}

	var errs []error
	for _, tree := range trees {
		if dryRun {
			cli.Info("[DRY RUN] Would fetch and fast-forward: %s", tree.path)
			continue
		}

		result, err := pullRepo(tree.path, job.batch)
		if err != nil {
			cli.Error("Failed to pull %s: %v", tree.name, err)
			errs = append(errs, fmt.Errorf("pull %s: %w", tree.name, err))
			continue
		}
		switch result.Outcome {
		case pullUpdated:
			cli.Info("Pulled %s: %d new commit(s)", tree.name, result.Commits)
		case pullSkipped:
			cli.Warn("Skipped %s: %s", tree.name, result.Reason)
		default:
			cli.Debug("%s is up to date", tree.name)
		}
	}

	return errors.Join(errs...)
}
