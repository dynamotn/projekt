package folderutil

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

// WorktreeFolder is where a working tree goes by default: inside the project,
// under a dotted folder, which keeps it beside the code and out of the way of
// a workspace's default regex.
const WorktreeFolder = ".worktrees"

// FindFolder returns the project folder a short name resolves to.
func FindFolder(shortName string) (ParsedFolder, error) {
	parsed, err := ParseConfig(lazypath.GetConfig())
	if err != nil {
		return ParsedFolder{}, err
	}

	for _, folder := range parsed {
		if folder.ShortName == shortName {
			return folder, nil
		}
	}
	return ParsedFolder{}, fmt.Errorf("no project named %q", shortName)
}

// AddWorktreeOptions drives `projekt worktree add`.
type AddWorktreeOptions struct {
	// Project is the short name of the project, as `pj` knows it.
	Project string
	// Branch is what the working tree checks out. It is created from the
	// current HEAD when it does not exist yet.
	Branch string
	// Name overrides the name after the "@", which otherwise comes from the
	// last element of the branch.
	Name string
	// Path overrides where the working tree is created. A relative path is
	// resolved against the project.
	Path string
	// DryRun reports what would happen without touching anything.
	DryRun bool
}

// AddWorktree creates a working tree of a project and records it, so that it
// can be jumped to as `<project>@<name>`.
func AddWorktree(out io.Writer, o AddWorktreeOptions) (lazypath.Worktree, error) {
	if err := lazypath.LoadError(); err != nil {
		return lazypath.Worktree{}, err
	}

	project, err := FindFolder(o.Project)
	if err != nil {
		return lazypath.Worktree{}, err
	}
	if _, _, isWorktree := lazypath.SplitWorktreeName(o.Project); isWorktree {
		// A working tree of a working tree would hang off a name that is
		// itself derived, and `git worktree add` wants the repository anyway.
		return lazypath.Worktree{}, fmt.Errorf("%s is a worktree; add the new one to its project instead", o.Project)
	}
	if !IsGitRepo(project.Path) {
		return lazypath.Worktree{}, fmt.Errorf("%s is not a git repository: %s", o.Project, project.Path)
	}

	branch := strings.TrimSpace(o.Branch)
	if branch == "" {
		return lazypath.Worktree{}, fmt.Errorf("no branch given")
	}

	name := strings.TrimSpace(o.Name)
	if name == "" {
		name = WorktreeNameFromBranch(branch)
	}
	if name == "" {
		return lazypath.Worktree{}, fmt.Errorf("cannot work out a name from branch %q, pass --name", branch)
	}

	worktree := lazypath.Worktree{
		Project: o.Project,
		Name:    name,
		Branch:  branch,
		Path:    worktreePath(project.Path, name, o.Path),
	}
	if err := worktree.Validate(); err != nil {
		return lazypath.Worktree{}, fmt.Errorf("worktree %s %v", worktree.ShortName(), err)
	}
	if _, _, found := lazypath.FindWorktree(worktree.Project, worktree.Name); found {
		return lazypath.Worktree{}, fmt.Errorf("%s is already in the configuration", worktree.ShortName())
	}
	if _, err := os.Stat(worktree.Path); err == nil {
		return lazypath.Worktree{}, fmt.Errorf("%s already exists", worktree.Path)
	} else if !os.IsNotExist(err) {
		return lazypath.Worktree{}, fmt.Errorf("cannot access %s: %w", worktree.Path, err)
	}

	if workspace, covered := CoveringWorkspace(worktree.Path); covered {
		cli.Warn("%s is directly inside the workspace %s, so it will also answer to its own name there; %s/%s keeps it out of the way",
			worktree.Path, workspace.Path, WorktreeFolder, name)
	}

	creates := !branchExists(project.Path, branch)
	if o.DryRun {
		verb := "check out"
		if creates {
			verb = "create"
		}
		_, err := fmt.Fprintf(out, "[DRY RUN] Would %s %s in %s, reachable with `pj %s`\n",
			verb, branch, worktree.Path, worktree.ShortName())
		return worktree, err
	}

	if err := addWorktree(project.Path, worktree.Path, branch); err != nil {
		return lazypath.Worktree{}, fmt.Errorf("cannot add worktree %s: %w", worktree.Path, err)
	}
	if err := worktree.AddToConfig(); err != nil {
		// The working tree is there; say so rather than pretending the whole
		// thing failed, so that it can be removed or recorded by hand.
		return worktree, fmt.Errorf("created %s but could not record it: %w", worktree.Path, err)
	}

	if creates {
		cli.Debug("Created branch %s", branch)
	}
	_, err = fmt.Fprintf(out, "%s\n", worktree.Path)
	return worktree, err
}

// worktreePath works out where a working tree belongs.
func worktreePath(projectPath, name, override string) string {
	if override == "" {
		return filepath.Join(projectPath, WorktreeFolder, name)
	}
	if filepath.IsAbs(override) {
		return filepath.Clean(override)
	}
	return filepath.Join(projectPath, override)
}

// unsafeInName is everything a folder name is better off without.
var unsafeInName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// WorktreeNameFromBranch turns a branch into the name after the "@".
//
// Only the last element is kept, because `feature/PROJ-123` and
// `bugfix/PROJ-123` are told apart by the ticket, not by the prefix, and a
// short name is something to type.
func WorktreeNameFromBranch(branch string) string {
	name := branch
	if index := strings.LastIndex(name, "/"); index >= 0 {
		name = name[index+1:]
	}
	name = unsafeInName.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-.")
	return name
}

// IsGitRepo reports whether a folder is a git repository, or sits inside one.
func IsGitRepo(path string) bool {
	return runGit(path, "rev-parse", "--git-dir") == nil
}

// IsRepoRoot reports whether a folder is a repository in its own right,
// rather than a folder that merely sits inside one.
//
// The difference matters whenever something is about to act on "the
// repository": pulling a folder that happens to live inside a checkout would
// pull that checkout, which is nobody's idea of updating the folder.
func IsRepoRoot(path string) bool {
	out, err := gitOutput(path, "rev-parse", "--show-toplevel")
	if err != nil {
		return false
	}
	top, err := filepath.Abs(strings.TrimSpace(out))
	if err != nil {
		return false
	}
	here, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	return top == here
}

// RemoveWorktreeOptions drives `projekt worktree remove`.
type RemoveWorktreeOptions struct {
	// Project and Name say which working tree to remove.
	Project string
	Name    string
	// Force removes a working tree with changes in it.
	Force bool
	// Keep leaves the files alone and only drops the configuration entry.
	Keep bool
}

// RemoveWorktree removes a working tree and forgets about it.
//
// The branch is left alone: it is the work, and deleting it is a separate
// decision from putting away the folder it was checked out in.
func RemoveWorktree(out io.Writer, o RemoveWorktreeOptions) error {
	if err := lazypath.LoadError(); err != nil {
		return err
	}

	worktree, _, found := lazypath.FindWorktree(o.Project, o.Name)
	if !found {
		return fmt.Errorf("no worktree named %s%s%s", o.Project, lazypath.WorktreeSeparator, o.Name)
	}

	if !o.Keep {
		project, err := FindFolder(worktree.Project)
		if err != nil {
			return fmt.Errorf("%s: %w, use --keep to drop the entry anyway", worktree.ShortName(), err)
		}

		args := []string{"worktree", "remove"}
		if o.Force {
			args = append(args, "--force")
		}
		args = append(args, worktree.Path)

		if err := runGit(project.Path, args...); err != nil {
			return fmt.Errorf("cannot remove worktree %s: %w", worktree.Path, err)
		}
	}

	if err := lazypath.RemoveWorktreeFromConfig(worktree.Project, worktree.Name); err != nil {
		return err
	}

	_, err := fmt.Fprintf(out, "Removed %s\n", worktree.ShortName())
	return err
}

// Worktree status values, as a listing prints them.
const (
	worktreeOK       = "ok"
	worktreeMissing  = "MISSING"
	worktreeUnknown  = "NOT A WORKTREE"
	worktreeOrphaned = "NO PROJECT"
	worktreeDrifted  = "BRANCH DRIFTED"
)

// ListWorktreeOption contains options for listing working trees.
type ListWorktreeOption struct {
	// Project keeps only the working trees of one project.
	Project string
	// Tags keeps only the working trees whose project carries all of them.
	Tags []string
	// NoStatus skips reading git, which is what makes the listing fast.
	NoStatus  bool
	NamesOnly bool
	cli.ListOutputOption
}

// ListWorktrees displays the configured working trees.
func ListWorktrees(out io.Writer, o *ListWorktreeOption) error {
	worktrees := lazypath.GetConfig().Worktrees
	parsed, err := ParseConfig(lazypath.GetConfig())
	if err != nil {
		return err
	}

	projects := make(map[string]ParsedFolder, len(parsed))
	for _, folder := range parsed {
		projects[folder.ShortName] = folder
	}

	view := cli.ListView{Columns: []cli.ListColumn{
		{Header: "NAME", Key: "name"},
		{Header: "BRANCH", Key: "branch"},
		{Header: "PATH", Key: "path"},
		{Header: "STATUS", Key: "status"},
	}}
	if o.NamesOnly {
		view.Columns = view.Columns[:1]
	} else if o.NoStatus {
		view.Columns = view.Columns[:3]
	}

	// One `git worktree list` per project, not per working tree.
	listed := map[string]map[string]string{}
	shown := 0

	for _, worktree := range worktrees {
		project, hasProject := projects[worktree.Project]
		if o.Project != "" && worktree.Project != o.Project {
			continue
		}
		if len(lazypath.NormalizeTags(o.Tags)) > 0 && !lazypath.HasTags(project.Tags, o.Tags) {
			continue
		}
		shown++

		if o.NamesOnly {
			view.AppendRow(worktree.ShortName())
			continue
		}
		if o.NoStatus {
			view.AppendRow(worktree.ShortName(), worktree.Branch, worktree.Path)
			continue
		}

		if hasProject {
			if _, done := listed[worktree.Project]; !done {
				listed[worktree.Project] = gitWorktrees(project.Path)
			}
		}
		view.AppendRow(worktree.ShortName(), worktree.Branch, worktree.Path,
			worktreeStatus(worktree, hasProject, listed[worktree.Project]))
	}

	if shown == 0 && o.Output != cli.OutputJSON {
		cli.Warn("No worktree configured")
		return nil
	}

	return cli.EncodeList(out, view, o.ListOutputOption)
}

// worktreeStatus compares what the configuration says with what git says.
func worktreeStatus(worktree lazypath.Worktree, hasProject bool, branches map[string]string) string {
	if !hasProject {
		return worktreeOrphaned
	}
	if _, err := os.Stat(worktree.Path); err != nil {
		return worktreeMissing
	}

	branch, known := branches[filepath.Clean(worktree.Path)]
	switch {
	case !known:
		// The folder is there but git does not count it as a working tree of
		// this repository any more.
		return worktreeUnknown
	case branch != "" && branch != worktree.Branch:
		return worktreeDrifted
	default:
		return worktreeOK
	}
}

// gitWorktrees reads the working trees git knows about, as path -> branch.
func gitWorktrees(repoPath string) map[string]string {
	cmd := exec.Command("git", "-C", repoPath, "worktree", "list", "--porcelain")

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		cli.Debug("Cannot list worktrees of %s: %v", repoPath, err)
		return nil
	}

	result := map[string]string{}
	var current string

	scanner := bufio.NewScanner(&output)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "worktree "):
			current = filepath.Clean(strings.TrimPrefix(line, "worktree "))
			result[current] = ""
		case strings.HasPrefix(line, "branch ") && current != "":
			result[current] = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
		}
	}

	return result
}

// DiagnoseWorktrees reports the working trees whose project no longer
// resolves.
//
// lazypath checks what a working tree says about itself; only this package can
// say whether the project it hangs off still exists, because that means
// resolving a short name.
func DiagnoseWorktrees(c lazypath.Config) []lazypath.Diagnostic {
	if len(c.Worktrees) == 0 {
		return nil
	}

	parsed, err := ParseConfig(c)
	if err != nil {
		// The folders themselves are broken; that is reported on its own, and
		// naming every worktree as well would only bury it.
		return nil
	}

	projects := make(map[string]struct{}, len(parsed))
	for _, folder := range parsed {
		projects[folder.ShortName] = struct{}{}
	}

	var diags []lazypath.Diagnostic
	for _, worktree := range c.Worktrees {
		if err := worktree.Validate(); err != nil {
			// lazypath already reported this one.
			continue
		}
		if _, ok := projects[worktree.Project]; ok {
			continue
		}
		diags = append(diags, lazypath.Diagnostic{
			Severity: lazypath.SeverityError,
			Message: fmt.Sprintf("worktree %s hangs off %q, which is not a project: it cannot be reached",
				worktree.ShortName(), worktree.Project),
		})
	}

	return diags
}

// CoveringWorkspace returns the configured workspace that already reaches a
// path, and whether one does.
//
// A folder a workspace picks up needs no entry of its own: it is already a
// project, under the workspace's prefix.
func CoveringWorkspace(path string) (lazypath.Folder, bool) {
	clean := filepath.Clean(path)
	parent := filepath.Dir(clean)
	base := filepath.Base(clean)

	for _, folder := range lazypath.GetConfig().Folders {
		if !folder.IsWorkspace {
			continue
		}
		if filepath.Clean(folder.Path) != parent {
			continue
		}
		re, err := regexp.Compile(folder.GetRegexMatch())
		if err != nil {
			cli.Debug("Cannot compile regex of workspace %s: %v", folder.Path, err)
			continue
		}
		if re.MatchString(base) {
			return folder, true
		}
	}

	return lazypath.Folder{}, false
}
