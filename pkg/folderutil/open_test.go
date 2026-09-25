package folderutil

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// openableFolder configures one project that exists and one that does not.
func openableFolder(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	project := filepath.Join(root, "myapp")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}

	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{
		{Path: project},
		{Path: filepath.Join(root, "gone")},
	}})
	t.Cleanup(lazypath.ResetTestConfig)

	return project
}

func TestOpenFolder(t *testing.T) {
	project := openableFolder(t)
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")

	var out bytes.Buffer
	// `pwd` stands in for an editor: it proves the working directory is the
	// project, which is what makes an editor that ignores its argument work.
	if err := OpenFolder(&out, "myapp", OpenOptions{With: "pwd", Out: &out}); err != nil {
		t.Fatalf("OpenFolder() error = %v", err)
	}
	if strings.TrimSpace(out.String()) != project {
		t.Errorf("the editor ran in %q, want %q", out.String(), project)
	}
}

func TestOpenFolder_PassesThePathAndTheArguments(t *testing.T) {
	project := openableFolder(t)

	var out bytes.Buffer
	if err := OpenFolder(&out, "myapp", OpenOptions{With: "code --new-window", DryRun: true}); err != nil {
		t.Fatalf("OpenFolder() error = %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "code --new-window "+project {
		t.Errorf("command = %q, want the arguments kept and the path last", got)
	}
}

func TestOpenFolder_UsesTheEnvironment(t *testing.T) {
	project := openableFolder(t)
	t.Setenv("VISUAL", "gvim --nofork")
	t.Setenv("EDITOR", "nano")

	var out bytes.Buffer
	if err := OpenFolder(&out, "myapp", OpenOptions{DryRun: true}); err != nil {
		t.Fatalf("OpenFolder() error = %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "gvim --nofork "+project {
		t.Errorf("command = %q, want VISUAL to win", got)
	}

	// And --with beats both.
	out.Reset()
	if err := OpenFolder(&out, "myapp", OpenOptions{With: "emacs", DryRun: true}); err != nil {
		t.Fatalf("OpenFolder() error = %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "emacs "+project {
		t.Errorf("command = %q, want --with to win", got)
	}
}

func TestOpenFolder_Errors(t *testing.T) {
	openableFolder(t)

	var out bytes.Buffer
	if err := OpenFolder(&out, "nope", OpenOptions{With: "true"}); err == nil {
		t.Error("OpenFolder() of an unknown project error = nil, want an error")
	}
	// Configured but not on disk: there is nothing to open.
	if err := OpenFolder(&out, "gone", OpenOptions{With: "true"}); err == nil {
		t.Error("OpenFolder() of a missing folder error = nil, want an error")
	}
	// An editor that fails says so.
	if err := OpenFolder(&out, "myapp", OpenOptions{With: "false", Out: &out}); err == nil {
		t.Error("OpenFolder() with a failing editor error = nil, want an error")
	}
}
