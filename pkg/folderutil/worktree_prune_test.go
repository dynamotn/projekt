package folderutil

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// prunableWorktrees sets up a real repository with one working tree that is
// there and one that has been removed behind git's back.
func prunableWorktrees(t *testing.T) (project, gone string) {
	t.Helper()

	root := t.TempDir()
	project = filepath.Join(root, "myapp")
	initRepo(t, project)

	here := filepath.Join(project, ".worktrees", "here")
	gone = filepath.Join(project, ".worktrees", "gone")
	mustGit(t, project, "worktree", "add", "-b", "here", here)
	mustGit(t, project, "worktree", "add", "-b", "gone", gone)
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}

	configFile := filepath.Join(root, "config.yaml")
	previous := lazypath.CfgFile
	lazypath.CfgFile = configFile
	lazypath.ResetTestConfig()
	lazypath.InitConfig()
	t.Cleanup(func() {
		lazypath.CfgFile = previous
		lazypath.ResetTestConfig()
	})

	lazypath.SetTestConfig(lazypath.Config{
		Folders: []lazypath.Folder{{Path: project}},
		Worktrees: []lazypath.Worktree{
			{Project: "myapp", Name: "here", Branch: "here", Path: here},
			{Project: "myapp", Name: "gone", Branch: "gone", Path: gone},
		},
	})

	return project, gone
}

func TestPruneWorktrees(t *testing.T) {
	project, _ := prunableWorktrees(t)

	// git still lists the removed one until it is told.
	before, err := gitOutputForTest(project, "worktree", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(before, "gone") {
		t.Skip("this git already forgot the worktree")
	}

	var out bytes.Buffer
	if err := PruneWorktrees(&out, PruneWorktreeOptions{}); err != nil {
		t.Fatalf("PruneWorktrees() error = %v", err)
	}

	// Both halves: the configuration entry and git's own record.
	worktrees := lazypath.GetConfig().Worktrees
	if len(worktrees) != 1 || worktrees[0].Name != "here" {
		t.Errorf("worktrees = %#v, want only the one that is there", worktrees)
	}
	after, err := gitOutputForTest(project, "worktree", "list")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(after, "gone") {
		t.Errorf("git still lists it:\n%s", after)
	}
	if !strings.Contains(after, "here") {
		t.Errorf("git forgot the one that is still there:\n%s", after)
	}
}

func TestPruneWorktrees_DryRun(t *testing.T) {
	_, _ = prunableWorktrees(t)

	var out bytes.Buffer
	if err := PruneWorktrees(&out, PruneWorktreeOptions{DryRun: true}); err != nil {
		t.Fatalf("PruneWorktrees() error = %v", err)
	}
	if !strings.Contains(out.String(), "DRY RUN") || !strings.Contains(out.String(), "gone") {
		t.Errorf("out = %q, want the plan", out.String())
	}
	if len(lazypath.GetConfig().Worktrees) != 2 {
		t.Error("the dry run changed the configuration")
	}
}

func TestPruneWorktrees_NothingToDo(t *testing.T) {
	lazypath.SetTestConfig(lazypath.Config{})
	t.Cleanup(lazypath.ResetTestConfig)

	var out bytes.Buffer
	if err := PruneWorktrees(&out, PruneWorktreeOptions{}); err != nil {
		t.Fatalf("PruneWorktrees() error = %v", err)
	}
	if !strings.Contains(out.String(), "Nothing to prune") {
		t.Errorf("out = %q", out.String())
	}
}

// mergedWorktrees sets up a project whose default branch is main, with the
// main checkout on another branch so that main can be checked out in a tree,
// and four recorded trees: done (merged, clean), wip (merged, untracked
// file), open (a commit main lacks) and trunk (main itself).
func mergedWorktrees(t *testing.T) (project string, trees map[string]string) {
	t.Helper()

	root := t.TempDir()
	project = filepath.Join(root, "myapp")
	initRepo(t, project)
	mustGit(t, project, "switch", "-c", "dev")

	trees = map[string]string{}
	for _, name := range []string{"done", "wip", "open", "trunk"} {
		trees[name] = filepath.Join(project, ".worktrees", name)
		branch := name
		if name == "trunk" {
			branch = "main"
		}
		args := []string{"worktree", "add", trees[name], branch}
		if name != "trunk" {
			args = []string{"worktree", "add", "-b", name, trees[name]}
		}
		mustGit(t, project, args...)
	}
	if err := os.WriteFile(filepath.Join(trees["wip"], "scratch"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(trees["open"], "more"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, trees["open"], "add", "more")
	mustGit(t, trees["open"], "-c", "user.email=t@example.com", "-c", "user.name=T", "commit", "-m", "more")

	lazypath.CfgFile = filepath.Join(root, "config.yaml")
	lazypath.ResetTestConfig()
	lazypath.InitConfig()
	t.Cleanup(lazypath.ResetTestConfig)

	var recorded []lazypath.Worktree
	for _, name := range []string{"done", "wip", "open", "trunk"} {
		branch := name
		if name == "trunk" {
			branch = "main"
		}
		recorded = append(recorded, lazypath.Worktree{Project: "myapp", Name: name, Branch: branch, Path: trees[name]})
	}
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{Path: project}}, Worktrees: recorded})
	return project, trees
}

func TestPruneWorktrees_Merged(t *testing.T) {
	project, trees := mergedWorktrees(t)

	var out bytes.Buffer
	if err := PruneWorktrees(&out, PruneWorktreeOptions{Merged: true}); err != nil {
		t.Fatalf("PruneWorktrees() error = %v", err)
	}

	if _, err := os.Stat(trees["done"]); !os.IsNotExist(err) {
		t.Errorf("done is still there: %v", err)
	}
	if err := runGit(project, "rev-parse", "--verify", "--quiet", "refs/heads/done"); err != nil {
		t.Error("the branch of done was deleted")
	}
	var left []string
	for _, w := range lazypath.GetConfig().Worktrees {
		left = append(left, w.Name)
	}
	if strings.Join(left, ",") != "wip,open,trunk" {
		t.Errorf("entries left = %v, want wip,open,trunk", left)
	}
	// Each tree once, and the main checkout not at all.
	for _, want := range []string{"myapp@wip: untracked files", "myapp@open: not merged", "myapp@trunk: default branch"} {
		if n := strings.Count(out.String(), want+"\n"); n != 1 {
			t.Errorf("%q appears %d times:\n%s", want, n, out.String())
		}
	}
	if strings.Contains(out.String(), "not recorded by projekt") {
		t.Errorf("a tree or the main checkout was named as unrecorded:\n%s", out.String())
	}
}

func TestPruneWorktrees_MergedKeepsIgnoredFiles(t *testing.T) {
	project, trees := mergedWorktrees(t)
	// info/exclude lives in the common git dir, so it covers every tree.
	if err := os.WriteFile(filepath.Join(project, ".git", "info", "exclude"), []byte("notes.local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(trees["done"], "notes.local"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := PruneWorktrees(&out, PruneWorktreeOptions{Merged: true}); err != nil {
		t.Fatalf("PruneWorktrees() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(trees["done"], "notes.local")); err != nil {
		t.Errorf("the ignored file is gone: %v", err)
	}
	if !strings.Contains(out.String(), "myapp@done: ignored files\n") {
		t.Errorf("out = %q", out.String())
	}
}

func TestPruneWorktrees_MergedNamesDriftedAndDetached(t *testing.T) {
	_, trees := mergedWorktrees(t)
	mustGit(t, trees["done"], "switch", "-c", "elsewhere")
	mustGit(t, trees["wip"], "checkout", "--detach")

	var out bytes.Buffer
	if err := PruneWorktrees(&out, PruneWorktreeOptions{Merged: true}); err != nil {
		t.Fatalf("PruneWorktrees() error = %v", err)
	}
	for _, want := range []string{"myapp@done: on elsewhere, configured done\n", "myapp@wip: detached HEAD\n"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("out lacks %q:\n%s", want, out.String())
		}
	}
	if _, err := os.Stat(trees["done"]); err != nil {
		t.Errorf("done was removed: %v", err)
	}
}

func TestPruneWorktrees_MergedOwnerByPath(t *testing.T) {
	project, trees := mergedWorktrees(t)
	root := filepath.Dir(project)

	// The same name in both files: the own entry points at open, the included
	// one at done. Only the entry that records a path decides about that path.
	included := filepath.Join(root, "team.yaml")
	body := "worktrees:\n  - {project: myapp, name: mine, branch: done, path: " + trees["done"] + "}\n"
	own := "include: [" + included + "]\nfolders:\n  - path: " + project + "\n" +
		"worktrees:\n  - {project: myapp, name: mine, branch: open, path: " + trees["open"] + "}\n"
	if err := os.WriteFile(included, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lazypath.CfgFile, []byte(own), 0o644); err != nil {
		t.Fatal(err)
	}
	lazypath.ResetTestConfig()
	lazypath.InitConfig()

	var out bytes.Buffer
	if err := PruneWorktrees(&out, PruneWorktreeOptions{Merged: true}); err != nil {
		t.Fatalf("PruneWorktrees() error = %v", err)
	}
	if _, err := os.Stat(trees["done"]); err != nil {
		t.Errorf("done, recorded only by the included file, was removed: %v", err)
	}
	for _, want := range []string{"myapp@mine: from " + included + "\n", "myapp@mine: not merged\n"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("out lacks %q:\n%s", want, out.String())
		}
	}
	if own := lazypath.OwnConfig().Worktrees; len(own) != 1 || own[0].Path != trees["open"] {
		t.Errorf("own entries = %#v, want the open one untouched", own)
	}
}

func TestPruneWorktrees_MergedDeclaredUnderGitRepos(t *testing.T) {
	project, trees := mergedWorktrees(t)
	declared := filepath.Join(project, ".worktrees", "declared")
	mustGit(t, project, "worktree", "add", "-b", "declared", declared)
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{
		Path: project,
		Git: &lazypath.GitConfig{Repos: []lazypath.GitRepo{{
			Name:      "myapp",
			Worktrees: []lazypath.GitWorktree{{Path: declared, Branch: "declared"}},
		}}},
	}}})
	_ = trees

	var out bytes.Buffer
	if err := PruneWorktrees(&out, PruneWorktreeOptions{Merged: true}); err != nil {
		t.Fatalf("PruneWorktrees() error = %v", err)
	}
	if _, err := os.Stat(declared); err != nil {
		t.Errorf("a tree declared under git.repos was removed: %v", err)
	}
	if want := declared + ": declared under git.repos\n"; !strings.Contains(out.String(), want) {
		t.Errorf("out lacks %q:\n%s", want, out.String())
	}
}

func TestPruneWorktrees_MergedDryRun(t *testing.T) {
	_, trees := mergedWorktrees(t)

	var out bytes.Buffer
	if err := PruneWorktrees(&out, PruneWorktreeOptions{Merged: true, DryRun: true}); err != nil {
		t.Fatalf("PruneWorktrees() error = %v", err)
	}

	if _, err := os.Stat(trees["done"]); err != nil {
		t.Errorf("the dry run removed done: %v", err)
	}
	if len(lazypath.GetConfig().Worktrees) != 4 {
		t.Error("the dry run changed the configuration")
	}
	for _, want := range []string{"[DRY RUN] Would remove myapp@done", "myapp@wip: untracked files", "myapp@open: not merged", "myapp@trunk: default branch"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("out lacks %q:\n%s", want, out.String())
		}
	}
}

func TestPruneWorktrees_MergedUnknownDefault(t *testing.T) {
	project, trees := mergedWorktrees(t)
	// Neither main nor master: the default cannot be found, so nothing goes.
	mustGit(t, project, "branch", "-m", "main", "trunkline")

	var out bytes.Buffer
	if err := PruneWorktrees(&out, PruneWorktreeOptions{Merged: true}); err != nil {
		t.Fatalf("PruneWorktrees() error = %v", err)
	}
	if _, err := os.Stat(trees["done"]); err != nil {
		t.Errorf("done was removed without a default branch: %v", err)
	}
	if !strings.Contains(out.String(), "myapp@done: default branch unknown") {
		t.Errorf("out = %q", out.String())
	}
}

func TestPruneWorktrees_MergedLeavesOthersNamed(t *testing.T) {
	project, trees := mergedWorktrees(t)
	root := filepath.Dir(project)

	// A tree git knows and the configuration does not.
	stray := filepath.Join(project, ".worktrees", "stray")
	mustGit(t, project, "worktree", "add", "-b", "stray", stray)

	// A tree that only an included file records.
	included := filepath.Join(root, "team.yaml")
	body := "worktrees:\n  - {project: myapp, name: theirs, branch: done, path: " + trees["done"] + "}\n"
	if err := os.WriteFile(included, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	own := "include: [" + included + "]\nfolders:\n  - path: " + project + "\n"
	if err := os.WriteFile(lazypath.CfgFile, []byte(own), 0o644); err != nil {
		t.Fatal(err)
	}
	lazypath.ResetTestConfig()
	lazypath.InitConfig()

	var out bytes.Buffer
	if err := PruneWorktrees(&out, PruneWorktreeOptions{Merged: true}); err != nil {
		t.Fatalf("PruneWorktrees() error = %v", err)
	}
	for _, path := range []string{stray, trees["done"]} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s was removed: %v", path, err)
		}
	}
	for _, want := range []string{"myapp@theirs: from " + included, stray + ": not recorded by projekt"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("out lacks %q:\n%s", want, out.String())
		}
	}
}

func TestPruneWorktrees_MergedKeepsATreeAnotherFileRecords(t *testing.T) {
	project, trees := mergedWorktrees(t)
	root := filepath.Dir(project)

	// Both files record done: the own one through a symlink to it.
	link := filepath.Join(root, "done-link")
	if err := os.Symlink(trees["done"], link); err != nil {
		t.Fatal(err)
	}
	included := filepath.Join(root, "team.yaml")
	body := "worktrees:\n  - {project: myapp, name: theirs, branch: done, path: " + trees["done"] + "}\n"
	own := "include: [" + included + "]\nfolders:\n  - path: " + project + "\n" +
		"worktrees:\n  - {project: myapp, name: mine, branch: done, path: " + link + "}\n"
	if err := os.WriteFile(included, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lazypath.CfgFile, []byte(own), 0o644); err != nil {
		t.Fatal(err)
	}
	lazypath.ResetTestConfig()
	lazypath.InitConfig()

	var out bytes.Buffer
	if err := PruneWorktrees(&out, PruneWorktreeOptions{Merged: true}); err != nil {
		t.Fatalf("PruneWorktrees() error = %v", err)
	}
	if _, err := os.Stat(trees["done"]); err != nil {
		t.Errorf("a tree an included file records was removed: %v", err)
	}
	if want := "myapp@mine: also from " + included + "\n"; !strings.Contains(out.String(), want) {
		t.Errorf("out lacks %q:\n%s", want, out.String())
	}
}

func TestPruneWorktrees_MergedKeepsAnOwnTreeDeclaredUnderGitRepos(t *testing.T) {
	project, trees := mergedWorktrees(t)
	config := lazypath.GetConfig()
	config.Folders[0].Git = &lazypath.GitConfig{Repos: []lazypath.GitRepo{{
		Name:      "myapp",
		Worktrees: []lazypath.GitWorktree{{Path: trees["done"], Branch: "done"}},
	}}}
	lazypath.SetTestConfig(config)
	_ = project

	var out bytes.Buffer
	if err := PruneWorktrees(&out, PruneWorktreeOptions{Merged: true}); err != nil {
		t.Fatalf("PruneWorktrees() error = %v", err)
	}
	if _, err := os.Stat(trees["done"]); err != nil {
		t.Errorf("a recorded tree declared under git.repos was removed: %v", err)
	}
	if want := "myapp@done: declared under git.repos\n"; !strings.Contains(out.String(), want) {
		t.Errorf("out lacks %q:\n%s", want, out.String())
	}
}
