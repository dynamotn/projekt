package folderutil

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

func TestDescribeFolder_SetsPrintsAndClears(t *testing.T) {
	_, project := editable(t)

	var out bytes.Buffer
	if err := DescribeFolder(&out, "work-myapp", "The billing API", false); err != nil {
		t.Fatalf("DescribeFolder(set) error = %v", err)
	}
	if !strings.Contains(out.String(), "work-myapp: The billing API") {
		t.Errorf("out = %q", out.String())
	}

	data, err := os.ReadFile(lazypath.CfgFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "description: The billing API") {
		t.Errorf("config = %q, want the description written", data)
	}

	folder, err := FindFolder("work-myapp")
	if err != nil {
		t.Fatal(err)
	}
	if folder.Description != "The billing API" {
		t.Errorf("parsed Description = %q", folder.Description)
	}

	out.Reset()
	if err := DescribeFolder(&out, "work-myapp", "", false); err != nil {
		t.Fatalf("DescribeFolder(print) error = %v", err)
	}
	if strings.TrimSpace(out.String()) != "The billing API" {
		t.Errorf("printed %q", out.String())
	}

	out.Reset()
	if err := DescribeFolder(&out, "work-myapp", "", true); err != nil {
		t.Fatalf("DescribeFolder(clear) error = %v", err)
	}
	data, _ = os.ReadFile(lazypath.CfgFile)
	if strings.Contains(string(data), "description") {
		t.Errorf("config = %q, want the key gone", data)
	}
	if err := DescribeFolder(&out, "work-myapp", "", false); err == nil {
		t.Error("printing a missing description must fail, for scripts")
	}
	_ = project
}

func TestDescribeFolder_WorkspaceChildCarriesTheWorkspaces(t *testing.T) {
	root, _ := editable(t)

	var out bytes.Buffer
	if err := DescribeFolder(&out, "oss-tool", "nope", false); err == nil {
		t.Error("a workspace child has no entry to describe")
	}

	// Describe the workspace itself through the configuration, and the child
	// inherits it the way it inherits tags.
	if err := lazypath.SetFolderDescription(filepath.Join(root, "oss"), "Open source clones"); err != nil {
		t.Fatal(err)
	}
	folder, err := FindFolder("oss-tool")
	if err != nil {
		t.Fatal(err)
	}
	if folder.Description != "Open source clones" {
		t.Errorf("child Description = %q, want the workspace's", folder.Description)
	}
}

func TestDescribeFolder_RefusesSeveralLines(t *testing.T) {
	editable(t)

	var out bytes.Buffer
	if err := DescribeFolder(&out, "work-myapp", "one\ntwo", false); err == nil {
		t.Error("a description is one line")
	}
}

func TestNoteFolder_AddReadHeadline(t *testing.T) {
	_, project := editable(t)
	clock := func() time.Time { return time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC) }

	var out bytes.Buffer
	if err := NoteFolder(&out, "work-myapp", NoteOptions{}); !errors.Is(err, ErrNoNote) {
		t.Errorf("reading a missing note: err = %v, want ErrNoNote", err)
	}

	out.Reset()
	if err := NoteFolder(&out, "work-myapp", NoteOptions{Add: "migration half done", Now: clock}); err != nil {
		t.Fatalf("NoteFolder(add) error = %v", err)
	}
	if err := NoteFolder(&out, "work-myapp", NoteOptions{Add: "see PR 42", Now: clock}); err != nil {
		t.Fatalf("NoteFolder(add) error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(project, NoteFile))
	if err != nil {
		t.Fatal(err)
	}
	want := "- 2026-10-18: migration half done\n- 2026-10-18: see PR 42\n"
	if string(content) != want {
		t.Errorf("note = %q, want %q", content, want)
	}

	out.Reset()
	if err := NoteFolder(&out, "work-myapp", NoteOptions{}); err != nil {
		t.Fatalf("NoteFolder(read) error = %v", err)
	}
	if out.String() != want {
		t.Errorf("read %q", out.String())
	}

	if got := NoteHeadline(project); got != "2026-10-18: migration half done" {
		t.Errorf("NoteHeadline = %q", got)
	}
}

func TestNoteHeadline_StripsHeadingMarks(t *testing.T) {
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".projekt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, NoteFile), []byte("\n\n## Where I left it\n- a bullet\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := NoteHeadline(project); got != "Where I left it" {
		t.Errorf("NoteHeadline = %q", got)
	}
	if got := NoteHeadline(t.TempDir()); got != "" {
		t.Errorf("NoteHeadline(no note) = %q", got)
	}
}

func TestNoteFolder_DefaultsToTheCurrentProject(t *testing.T) {
	_, project := editable(t)
	t.Chdir(project)

	var out bytes.Buffer
	if err := NoteFolder(&out, "", NoteOptions{Add: "from inside"}); err != nil {
		t.Fatalf("NoteFolder() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(project, NoteFile)); err != nil {
		t.Errorf("note not written in the current project: %v", err)
	}
}

func TestStatusFolders_Notes(t *testing.T) {
	clean, _ := statusFolders(t)
	if err := appendNote(filepath.Join(clean, NoteFile), "waiting on review", nil); err != nil {
		t.Fatal(err)
	}

	rows := statusOf(t, &StatusOptions{Notes: true})
	note, _ := rows["clean"]["note"].(string)
	if !strings.HasSuffix(note, ": waiting on review") {
		t.Errorf("note column = %q", note)
	}
	if note, _ := rows["dirty"]["note"].(string); note != "" {
		t.Errorf("a project without a note has note = %q, want empty", note)
	}
}

func TestSelectFolder_PickerSeesDescriptions(t *testing.T) {
	selectable(t)
	lazypath.ResetTestConfig()
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{
		{Path: "/tmp/api-gateway", Description: "The edge"},
		{Path: "/tmp/api-docs"},
	}})

	// A picker that echoes every line it was given, so the test can see the
	// input, and answers with the described one, description and all.
	dir := t.TempDir()
	picker := filepath.Join(dir, "fakepicker")
	script := "#!/bin/sh\nwhile IFS= read -r line; do\n  printf '%s\\n' \"$line\" >&2\n  case \"$line\" in api-gateway*) printf '%s\\n' \"$line\" ;; esac\ndone\n"
	if err := os.WriteFile(picker, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	o := SelectOptions{Query: "api", With: picker, Err: &errOut, NoRecord: true}
	if err := SelectFolder(&out, o); err != nil {
		t.Fatalf("SelectFolder() error = %v", err)
	}
	if !strings.Contains(errOut.String(), "api-gateway\tThe edge\n") {
		t.Errorf("picker input = %q, want the description after a tab", errOut.String())
	}
	if !strings.Contains(errOut.String(), "api-docs\n") || strings.Contains(errOut.String(), "api-docs\t") {
		t.Errorf("picker input = %q, want a bare name without a description", errOut.String())
	}
	if strings.TrimSpace(out.String()) != "/tmp/api-gateway" {
		t.Errorf("out = %q, want the answer read back up to the tab", out.String())
	}
}
