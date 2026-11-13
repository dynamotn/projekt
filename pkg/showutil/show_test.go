package showutil

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// project makes a folder with a note and returns its path and short name.
func project(t *testing.T, note string) (dir, name string) {
	t.Helper()
	dir = filepath.Join(t.TempDir(), "api")
	if err := os.MkdirAll(filepath.Join(dir, ".projekt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if note != "" {
		if err := os.WriteFile(filepath.Join(dir, folderutil.NoteFile), []byte(note), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(lazypath.ResetTestConfig)
	return dir, "api"
}

func TestGather_PlainProject(t *testing.T) {
	dir, name := project(t, "half done\n")
	wt := filepath.Join(t.TempDir(), "next")
	lazypath.SetTestConfig(lazypath.Config{
		Folders:   []lazypath.Folder{{Path: dir, Tags: []string{"go"}, Description: "The API"}},
		Worktrees: []lazypath.Worktree{{Project: name, Name: "next", Branch: "next", Path: wt}},
	})

	info, err := Gather(name, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != name || info.Path != dir || info.Description != "The API" || info.Note != "half done\n" {
		t.Errorf("unexpected info: %+v", info)
	}
	if !reflect.DeepEqual(info.Tags, []string{"go"}) {
		t.Errorf("Tags = %v", info.Tags)
	}
	if want := []Worktree{{Name: "next", Branch: "next", Path: wt}}; !reflect.DeepEqual(info.Worktrees, want) {
		t.Errorf("Worktrees = %v, want %v", info.Worktrees, want)
	}
	// dir is no repository.
	want := folderutil.StatusOf(folderutil.ParsedFolder{ShortName: name, Path: dir}).State
	if info.Git == nil || info.Git.State != want {
		t.Errorf("Git = %+v, want state %q", info.Git, want)
	}
	if info.Source != lazypath.ConfigFile() {
		t.Errorf("Source = %q, want %q", info.Source, lazypath.ConfigFile())
	}
}

func TestGather_WorkspaceChild(t *testing.T) {
	ws := t.TempDir()
	if err := os.Mkdir(filepath.Join(ws, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(lazypath.ResetTestConfig)
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{
		{Path: ws, IsWorkspace: true, Tags: []string{"work"}, Description: "All of it"},
	}})

	info, err := Gather("child", Options{NoStatus: true})
	if err != nil {
		t.Fatal(err)
	}
	if info.Workspace != ws || info.Description != "All of it" || !reflect.DeepEqual(info.Tags, []string{"work"}) {
		t.Errorf("unexpected info: %+v", info)
	}
}

func TestGather_WorkingTreeName(t *testing.T) {
	dir, name := project(t, "")
	wt := t.TempDir()
	lazypath.SetTestConfig(lazypath.Config{
		Folders:   []lazypath.Folder{{Path: dir}},
		Worktrees: []lazypath.Worktree{{Project: name, Name: "next", Branch: "next", Path: wt}},
	})

	info, err := Gather(name+"@next", Options{NoStatus: true})
	if err != nil {
		t.Fatal(err)
	}
	if info.Path != wt || len(info.Worktrees) != 0 {
		t.Errorf("unexpected info: %+v", info)
	}
}

func TestGather_UnknownName(t *testing.T) {
	dir, _ := project(t, "")
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{Path: dir}}})
	if _, err := Gather("nope", Options{}); err == nil {
		t.Error("want an error for an unknown name")
	}
}

func TestGather_NoStatus(t *testing.T) {
	dir, name := project(t, "")
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{Path: dir}}})
	info, err := Gather(name, Options{NoStatus: true})
	if err != nil {
		t.Fatal(err)
	}
	if info.Git != nil {
		t.Errorf("Git = %+v, want nil", info.Git)
	}
}

func TestShow_JSONIsOneObjectWithoutEmptyFields(t *testing.T) {
	dir, name := project(t, "")
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{Path: dir}}})

	var buf bytes.Buffer
	o := Options{ListOutputOption: cli.ListOutputOption{Output: cli.OutputJSON}}
	if err := Show(&buf, name, o); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("not one JSON object: %v\n%s", err, buf.String())
	}
	for _, key := range []string{"name", "path"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing %q", key)
		}
	}
	for _, key := range []string{"hooks", "templates", "workspace"} {
		if _, ok := got[key]; ok {
			t.Errorf("unexpected %q", key)
		}
	}
}

func TestShow_Table(t *testing.T) {
	dir, name := project(t, "# Plan\nmore\n")
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{Path: dir, Tags: []string{"go", "cli"}}}})

	var buf bytes.Buffer
	o := Options{NoStatus: true, ListOutputOption: cli.ListOutputOption{Output: cli.OutputTSV}}
	if err := Show(&buf, name, o); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"name\tapi", "tags\tgo, cli", "note\tPlan…"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, buf.String())
		}
	}
}

func TestGather_LinkedNoteIsAnError(t *testing.T) {
	dir, name := project(t, "")
	target := filepath.Join(t.TempDir(), "other.md")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, folderutil.NoteFile)); err != nil {
		t.Fatal(err)
	}
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{Path: dir}}})

	if _, err := Gather(name, Options{}); err == nil {
		t.Error("want the error from ReadNote")
	}
}
