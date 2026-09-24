package folderutil

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

// editable sets up a project with its own entry, a workspace with a child, and
// a configuration file that can be written to.
func editable(t *testing.T) (root, project string) {
	t.Helper()

	root = t.TempDir()
	project = filepath.Join(root, "myapp")
	workspaceChild := filepath.Join(root, "oss", "tool")
	for _, path := range []string{project, workspaceChild} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
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

	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{
		{Path: project, Prefix: "work", Priority: 5, Tags: []string{"go"}},
		{Path: filepath.Join(root, "oss"), Prefix: "oss", IsWorkspace: true, Tags: []string{"oss"}},
	}})

	return root, project
}

func TestMoveFolder_MovesTheFilesAndKeepsTheEntry(t *testing.T) {
	root, project := editable(t)
	target := filepath.Join(root, "renamed")

	var out bytes.Buffer
	if err := MoveFolder(&out, "work-myapp", target, MoveOptions{}); err != nil {
		t.Fatalf("MoveFolder() error = %v", err)
	}

	if _, err := os.Stat(target); err != nil {
		t.Errorf("the folder did not move: %v", err)
	}
	if _, err := os.Stat(project); !os.IsNotExist(err) {
		t.Error("the folder is still at the old path")
	}

	folders := lazypath.GetConfig().Folders
	var moved lazypath.Folder
	for _, folder := range folders {
		if folder.Path == target {
			moved = folder
		}
	}
	// Everything the entry was for survives the move.
	if moved.Prefix != "work" || moved.Priority != 5 || len(moved.Tags) != 1 || moved.Tags[0] != "go" {
		t.Errorf("moved entry = %#v, want the prefix, priority and tags kept", moved)
	}
}

func TestMoveFolder_CatchesUpWhenTheFolderAlreadyMoved(t *testing.T) {
	root, project := editable(t)
	target := filepath.Join(root, "already-there")

	// Moved by hand: only the configuration is behind.
	if err := os.Rename(project, target); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := MoveFolder(&out, "work-myapp", target, MoveOptions{}); err != nil {
		t.Fatalf("MoveFolder() error = %v", err)
	}
	if _, _, ok := lazypath.FindOwnFolder(target); !ok {
		t.Error("the configuration did not catch up")
	}
}

func TestMoveFolder_WorktreesFollow(t *testing.T) {
	root, project := editable(t)
	worktree := filepath.Join(project, ".worktrees", "PROJ-1")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}

	config := lazypath.GetConfig()
	config.Worktrees = []lazypath.Worktree{
		{Project: "work-myapp", Name: "PROJ-1", Branch: "feature/x", Path: worktree},
	}
	lazypath.SetTestConfig(config)

	target := filepath.Join(root, "renamed")
	var out bytes.Buffer
	if err := MoveFolder(&out, "work-myapp", target, MoveOptions{}); err != nil {
		t.Fatalf("MoveFolder() error = %v", err)
	}

	moved := lazypath.GetConfig().Worktrees
	if len(moved) != 1 {
		t.Fatalf("worktrees = %#v", moved)
	}
	// The path followed the folder...
	if moved[0].Path != filepath.Join(target, ".worktrees", "PROJ-1") {
		t.Errorf("worktree path = %v, want it under the new folder", moved[0].Path)
	}
	// ...and so did the name, since moving a project renames it.
	if moved[0].Project != "work-renamed" {
		t.Errorf("worktree project = %v, want work-renamed", moved[0].Project)
	}
	if _, err := ResolveFolder("work-renamed@PROJ-1", GetOptions{ExactOnly: true}); err != nil {
		t.Errorf("the worktree no longer resolves: %v", err)
	}
}

func TestMoveFolder_DryRun(t *testing.T) {
	root, project := editable(t)
	target := filepath.Join(root, "renamed")

	var out bytes.Buffer
	if err := MoveFolder(&out, "work-myapp", target, MoveOptions{DryRun: true}); err != nil {
		t.Fatalf("MoveFolder() error = %v", err)
	}
	if !strings.Contains(out.String(), "DRY RUN") {
		t.Errorf("out = %q, want the plan", out.String())
	}
	if _, err := os.Stat(project); err != nil {
		t.Error("the dry run moved the folder")
	}
}

func TestMoveFolder_Errors(t *testing.T) {
	root, project := editable(t)

	var out bytes.Buffer
	if err := MoveFolder(&out, "nope", filepath.Join(root, "x"), MoveOptions{}); err == nil {
		t.Error("MoveFolder() of an unknown project error = nil, want an error")
	}
	// A workspace child has no entry to move.
	if err := MoveFolder(&out, "oss-tool", filepath.Join(root, "x"), MoveOptions{}); err == nil {
		t.Error("MoveFolder() of a workspace child error = nil, want an error")
	}
	if err := MoveFolder(&out, "work-myapp", project, MoveOptions{}); err == nil {
		t.Error("MoveFolder() to where it already is error = nil, want an error")
	}

	// Both there: which one is the project is not this command's guess to make.
	other := filepath.Join(root, "other")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MoveFolder(&out, "work-myapp", other, MoveOptions{}); err == nil {
		t.Error("MoveFolder() with both paths there error = nil, want an error")
	}
}

func TestTagFolder(t *testing.T) {
	editable(t)

	var out bytes.Buffer
	if err := TagFolder(&out, "work-myapp", []string{"cli", "tools"}, nil); err != nil {
		t.Fatalf("TagFolder() error = %v", err)
	}
	if !strings.Contains(out.String(), "cli, go, tools") {
		t.Errorf("out = %q, want the tags sorted", out.String())
	}

	out.Reset()
	if err := TagFolder(&out, "work-myapp", nil, []string{"go"}); err != nil {
		t.Fatalf("TagFolder() error = %v", err)
	}
	if strings.Contains(out.String(), "go") {
		t.Errorf("out = %q, want the removed tag gone", out.String())
	}

	// Adding and removing at once, and a tag added twice stays once.
	out.Reset()
	if err := TagFolder(&out, "work-myapp", []string{"cli", "new"}, []string{"tools"}); err != nil {
		t.Fatalf("TagFolder() error = %v", err)
	}
	if strings.TrimSpace(out.String()) != "work-myapp: cli, new" {
		t.Errorf("out = %q", out.String())
	}
}

func TestTagFolder_Errors(t *testing.T) {
	editable(t)

	var out bytes.Buffer
	if err := TagFolder(&out, "oss-tool", []string{"x"}, nil); err == nil {
		t.Error("TagFolder() on a workspace child error = nil, want an error")
	}
	if err := TagFolder(&out, "nope", []string{"x"}, nil); err == nil {
		t.Error("TagFolder() of an unknown project error = nil, want an error")
	}
}

func TestListTags(t *testing.T) {
	editable(t)

	var out bytes.Buffer
	if err := ListTags(&out); err != nil {
		t.Fatalf("ListTags() error = %v", err)
	}
	// One project carries go, the workspace child inherits oss.
	if !strings.Contains(out.String(), "go\t1") || !strings.Contains(out.String(), "oss\t1") {
		t.Errorf("out = %q, want each tag and how many carry it", out.String())
	}
}

func TestTagsAfter(t *testing.T) {
	got := tagsAfter([]string{"b", "a"}, []string{"c", "a", "  "}, []string{"b"})
	if strings.Join(got, ",") != "a,c" {
		t.Errorf("tagsAfter() = %v, want a,c sorted and deduplicated", got)
	}
}
