package folderutil

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
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
