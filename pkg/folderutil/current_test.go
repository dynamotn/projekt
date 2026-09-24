package folderutil

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

func TestCurrentFolder(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "myapp")
	deep := filepath.Join(project, "deep", "nested")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{Path: project}}})
	t.Cleanup(lazypath.ResetTestConfig)

	var out bytes.Buffer
	if err := CurrentFolder(&out, CurrentOptions{From: deep}); err != nil {
		t.Fatalf("CurrentFolder() error = %v", err)
	}
	if strings.TrimSpace(out.String()) != "myapp" {
		t.Errorf("got %q, want the project a deep folder is in", out.String())
	}

	out.Reset()
	if err := CurrentFolder(&out, CurrentOptions{From: project, PrintPath: true}); err != nil {
		t.Fatalf("CurrentFolder() error = %v", err)
	}
	if strings.TrimSpace(out.String()) != project {
		t.Errorf("got %q, want the path", out.String())
	}
}

func TestCurrentFolder_DeepestWins(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "myapp")
	worktree := filepath.Join(project, ".worktrees", "PROJ-1")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}

	lazypath.SetTestConfig(lazypath.Config{
		Folders: []lazypath.Folder{{Path: project}},
		Worktrees: []lazypath.Worktree{
			{Project: "myapp", Name: "PROJ-1", Branch: "feature/x", Path: worktree},
		},
	})
	t.Cleanup(lazypath.ResetTestConfig)

	// Both contain the path; the closer one is the answer to "where am I".
	var out bytes.Buffer
	if err := CurrentFolder(&out, CurrentOptions{From: worktree}); err != nil {
		t.Fatalf("CurrentFolder() error = %v", err)
	}
	if strings.TrimSpace(out.String()) != "myapp@PROJ-1" {
		t.Errorf("got %q, want the working tree", out.String())
	}
}

func TestCurrentFolder_Outside(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "myapp")
	outside := filepath.Join(root, "myapp-docs")
	for _, path := range []string{project, outside} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{Path: project}}})
	t.Cleanup(lazypath.ResetTestConfig)

	var out bytes.Buffer
	// A sibling whose name starts the same is not inside it.
	err := CurrentFolder(&out, CurrentOptions{From: outside})
	if !errors.Is(err, ErrNotInAProject) {
		t.Errorf("CurrentFolder() error = %v, want ErrNotInAProject", err)
	}
	if out.String() != "" {
		t.Errorf("out = %q, want nothing printed", out.String())
	}
}

func TestCurrentFolder_Quiet(t *testing.T) {
	root := t.TempDir()
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{Path: root}}})
	t.Cleanup(lazypath.ResetTestConfig)

	var out bytes.Buffer
	if err := CurrentFolder(&out, CurrentOptions{From: root, Quiet: true}); err != nil {
		t.Fatalf("CurrentFolder() error = %v", err)
	}
	if out.String() != "" {
		t.Errorf("out = %q, want the exit code to be the whole answer", out.String())
	}
}

func TestContains(t *testing.T) {
	cases := map[[2]string]bool{
		{"/home/me/app", "/home/me/app"}:       true,
		{"/home/me/app", "/home/me/app/deep"}:  true,
		{"/home/me/app", "/home/me/app-docs"}:  false,
		{"/home/me/app", "/home/me"}:           false,
		{"/home/me/app/", "/home/me/app/deep"}: true,
	}
	for pair, want := range cases {
		if got := contains(pair[0], pair[1]); got != want {
			t.Errorf("contains(%q, %q) = %v, want %v", pair[0], pair[1], got, want)
		}
	}
}
