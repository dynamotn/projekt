package worktree

import (
	"bytes"
	"strings"
	"testing"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

func TestNewWorktreeCmd(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewWorktreeCmd(&buf)

	if cmd.Use != "worktree" {
		t.Errorf("Use = %v, want worktree", cmd.Use)
	}

	aliases := map[string]bool{}
	for _, alias := range cmd.Aliases {
		aliases[alias] = true
	}
	if !aliases["wt"] {
		t.Errorf("Aliases = %v, want wt among them", cmd.Aliases)
	}

	subcommands := map[string]bool{}
	for _, c := range cmd.Commands() {
		subcommands[c.Name()] = true
	}
	for _, name := range []string{"add", "list", "remove"} {
		if !subcommands[name] {
			t.Errorf("missing %q subcommand, got %v", name, subcommands)
		}
	}
}

func TestRemoveCmd_RejectsANameThatIsNotAWorktree(t *testing.T) {
	lazypath.SetTestConfig(lazypath.Config{})
	t.Cleanup(lazypath.ResetTestConfig)

	var buf bytes.Buffer
	cmd := NewWorktreeRemoveCmd(&buf)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"myapp"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want an error for a name with no @")
	}
	if !strings.Contains(err.Error(), "@") {
		t.Errorf("error = %v, want it to say what the name should look like", err)
	}
}

func TestCompListProjects_LeavesOutWorktrees(t *testing.T) {
	lazypath.SetTestConfig(lazypath.Config{
		Folders: []lazypath.Folder{{Path: t.TempDir(), Name: "myapp"}},
		Worktrees: []lazypath.Worktree{
			{Project: "myapp", Name: "PROJ-123", Branch: "feature/x", Path: t.TempDir()},
		},
	})
	t.Cleanup(lazypath.ResetTestConfig)

	projects, _ := compListProjects("")
	for _, name := range projects {
		if strings.Contains(name, lazypath.WorktreeSeparator) {
			t.Errorf("compListProjects() offered %q, which is a worktree", name)
		}
	}
	if len(projects) != 1 || projects[0] != "myapp" {
		t.Errorf("compListProjects() = %v, want just the project", projects)
	}

	// Removing one, on the other hand, is about the worktrees.
	worktrees, _ := compListWorktrees("")
	if len(worktrees) != 1 || worktrees[0] != "myapp@PROJ-123" {
		t.Errorf("compListWorktrees() = %v", worktrees)
	}
}

func TestAddCmd_Flags(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewWorktreeAddCmd(&buf)

	for _, name := range []string{"name", "path", "dry-run"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("add is missing --%s", name)
		}
	}
}

func TestListCmd_Flags(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewWorktreeListCmd(&buf)

	for _, name := range []string{"names-only", "no-status", "output", "tags"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("list is missing --%s", name)
		}
	}
}

func TestRemoveCmd_Flags(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewWorktreeRemoveCmd(&buf)

	for _, name := range []string{"force", "keep"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("remove is missing --%s", name)
		}
	}
}
