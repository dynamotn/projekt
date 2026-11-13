package folderutil

import (
	"bytes"
	"errors"
	"io"
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

// linkedNote lays out a project whose note is a link to a file holding a
// secret, and returns the project and that file.
func linkedNote(t *testing.T) (project, secret string) {
	t.Helper()
	project = t.TempDir()
	secret = filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte("TOPSECRET\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(project, ".projekt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(project, NoteFile)); err != nil {
		t.Fatal(err)
	}
	return project, secret
}

func TestReadNote_RefusesALink(t *testing.T) {
	project, _ := linkedNote(t)

	if text, err := ReadNote(project); err == nil || strings.Contains(text, "TOPSECRET") {
		t.Errorf("ReadNote() = %q, %v, want the link refused", text, err)
	}
	if got := NoteHeadline(project); got != "" {
		t.Errorf("NoteHeadline() = %q, want nothing through a link", got)
	}
}

func TestAppendNote_RefusesALink(t *testing.T) {
	project, secret := linkedNote(t)

	if err := appendNote(filepath.Join(project, NoteFile), "x", nil); err == nil {
		t.Error("appendNote() through a link = nil, want an error")
	}
	if got, _ := os.ReadFile(secret); string(got) != "TOPSECRET\n" {
		t.Errorf("the linked file was changed: %q", got)
	}
}

func TestReadNote_RefusesALinkedFolder(t *testing.T) {
	project := t.TempDir()
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "notes.md"), []byte("TOPSECRET\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, filepath.Join(project, ".projekt")); err != nil {
		t.Fatal(err)
	}

	if text, err := ReadNote(project); err == nil || strings.Contains(text, "TOPSECRET") {
		t.Errorf("ReadNote() = %q, %v, want the linked folder refused", text, err)
	}
}

func TestReadNote_NoNoteIsEmpty(t *testing.T) {
	if text, err := ReadNote(t.TempDir()); text != "" || err != nil {
		t.Errorf("ReadNote() = %q, %v, want empty and no error", text, err)
	}
}

func TestNoteFolder_EditRefusesALink(t *testing.T) {
	_, project := editable(t)
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte("TOPSECRET\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(project, ".projekt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(project, NoteFile)); err != nil {
		t.Fatal(err)
	}
	// An editor that leaves a marker behind, so a run of it shows.
	dir := t.TempDir()
	marker := filepath.Join(dir, "ran")
	editor := filepath.Join(dir, "editor")
	if err := os.WriteFile(editor, []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", editor)

	var out bytes.Buffer
	if err := NoteFolder(&out, "work-myapp", NoteOptions{Edit: true}); err == nil {
		t.Error("NoteFolder(edit) through a link = nil, want an error")
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("the editor ran on a linked note")
	}
}

func TestDescribeFolder_RefusesControlCharacters(t *testing.T) {
	editable(t)

	var out bytes.Buffer
	for _, text := range []string{"a\x00b", "a\x1b[31mb", "a\tb", "a\u0085b"} {
		if err := DescribeFolder(&out, "work-myapp", text, false); err == nil {
			t.Errorf("DescribeFolder(%q) error = nil, want an error", text)
		}
	}
	if data, _ := os.ReadFile(lazypath.CfgFile); strings.Contains(string(data), "description") {
		t.Errorf("config = %q, want no description written", data)
	}
}

func generateSetup(t *testing.T) (project string) {
	t.Helper()
	_, project = editable(t)
	if err := os.WriteFile(filepath.Join(project, "README.md"), []byte("Invoices for payments teams"), 0o644); err != nil {
		t.Fatal(err)
	}
	return project
}

func generateOpts(out *bytes.Buffer, answer string) GenerateOptions {
	return GenerateOptions{
		Yes: true, In: strings.NewReader("y\n"), Out: out, Err: &bytes.Buffer{},
		Ask: func(string, io.Writer) (string, error) { return answer, nil },
	}
}

func configHas(t *testing.T, want string) bool {
	t.Helper()
	data, _ := os.ReadFile(lazypath.CfgFile)
	return strings.Contains(string(data), want)
}

func TestGenerateDescription_PrintsAndKeepsWithYes(t *testing.T) {
	generateSetup(t)
	var out bytes.Buffer
	if err := GenerateDescription("work-myapp", generateOpts(&out, "\"The billing API.\"\n")); err != nil {
		t.Fatal(err)
	}
	if !configHas(t, "description: The billing API\n") {
		t.Errorf("config does not hold the description")
	}
}

func TestGenerateDescription_NotATerminalOnlyPrints(t *testing.T) {
	generateSetup(t)
	var out bytes.Buffer
	o := generateOpts(&out, "The billing API")
	o.Yes = false
	if err := GenerateDescription("work-myapp", o); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "The billing API") || configHas(t, "description") {
		t.Errorf("out = %q, want the draft printed and nothing written", out.String())
	}
}

func TestGenerateDescription_ShowPromptRunsNothing(t *testing.T) {
	generateSetup(t)
	var out bytes.Buffer
	o := generateOpts(&out, "")
	o.ShowPrompt = true
	o.Ask = func(string, io.Writer) (string, error) { t.Fatal("Ask called"); return "", nil }
	if err := GenerateDescription("work-myapp", o); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"work-myapp", "Invoices for payments teams"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("prompt lacks %q: %q", want, out.String())
		}
	}
}

func TestGenerateDescription_RefusesWhatIsNotOneLine(t *testing.T) {
	generateSetup(t)
	for _, answer := range []string{"two\nlines", strings.Repeat("x", 400), ""} {
		var out bytes.Buffer
		if err := GenerateDescription("work-myapp", generateOpts(&out, answer)); err == nil {
			t.Errorf("answer %q was kept", answer)
		}
		if configHas(t, "description") {
			t.Errorf("answer %q reached the configuration", answer)
		}
	}
}

func TestGenerateDescription_AnswerIsText(t *testing.T) {
	project := generateSetup(t)
	var out bytes.Buffer
	if err := GenerateDescription("work-myapp", generateOpts(&out, "$(touch pwned)")); err != nil {
		t.Fatal(err)
	}
	if !configHas(t, "$(touch pwned)") {
		t.Error("the literal text was not kept")
	}
	if _, err := os.Stat(filepath.Join(project, "pwned")); err == nil {
		t.Error("the answer was run")
	}
}
