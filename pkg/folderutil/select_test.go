package folderutil

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

// selectable configures three projects and a scratch history.
func selectable(t *testing.T) {
	t.Helper()

	useHistoryFor(t)
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{
		{Path: "/tmp/api-gateway", Tags: []string{"work"}},
		{Path: "/tmp/api-docs", Tags: []string{"work"}},
		{Path: "/tmp/notes"},
	}})
	t.Cleanup(lazypath.ResetTestConfig)

	// Nothing on PATH, so the numbered fallback is what runs unless a test
	// says otherwise.
	t.Setenv("PROJEKT_PICKER", "")
	t.Setenv("PATH", t.TempDir())
}

func TestSelectFolder_OneCandidateNeedsNoQuestion(t *testing.T) {
	selectable(t)

	var out, errOut bytes.Buffer
	o := SelectOptions{Query: "notes", In: strings.NewReader(""), Err: &errOut, NoRecord: true}
	if err := SelectFolder(&out, o); err != nil {
		t.Fatalf("SelectFolder() error = %v", err)
	}
	if strings.TrimSpace(out.String()) != "/tmp/notes" {
		t.Errorf("out = %q, want the only candidate", out.String())
	}
	if errOut.String() != "" {
		t.Errorf("stderr = %q, want nothing asked", errOut.String())
	}
}

func TestSelectFolder_AWholeNameNeedsNoQuestion(t *testing.T) {
	selectable(t)
	lazypath.ResetTestConfig()
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{
		{Path: "/tmp/api"},
		{Path: "/tmp/api-gateway"},
	}})

	var out, errOut bytes.Buffer
	o := SelectOptions{Query: "api", In: strings.NewReader(""), Err: &errOut, NoRecord: true}
	if err := SelectFolder(&out, o); err != nil {
		t.Fatalf("SelectFolder() error = %v", err)
	}
	if strings.TrimSpace(out.String()) != "/tmp/api" {
		t.Errorf("out = %q, want the exact name", out.String())
	}
	if errOut.String() != "" {
		t.Errorf("stderr = %q, want nothing asked for a whole name", errOut.String())
	}
}

func TestSelectFolder_NumberedFallback(t *testing.T) {
	selectable(t)

	var out, errOut bytes.Buffer
	o := SelectOptions{Query: "api", In: strings.NewReader("2\n"), Err: &errOut, NoRecord: true}
	if err := SelectFolder(&out, o); err != nil {
		t.Fatalf("SelectFolder() error = %v", err)
	}

	// The list is drawn on stderr so that the path on stdout is the answer.
	for _, want := range []string{"api-docs", "api-gateway", "Which one?"} {
		if !strings.Contains(errOut.String(), want) {
			t.Errorf("stderr = %q, want it to contain %q", errOut.String(), want)
		}
	}
	if strings.Contains(out.String(), "Which one?") {
		t.Errorf("stdout = %q, want only the path", out.String())
	}

	// Ranked shorter-first, so 1 is api-docs and 2 is api-gateway.
	if strings.TrimSpace(out.String()) != "/tmp/api-gateway" {
		t.Errorf("out = %q, want the second candidate", out.String())
	}
}

func TestSelectFolder_BadAnswers(t *testing.T) {
	selectable(t)

	for _, answer := range []string{"0\n", "9\n", "banana\n", ""} {
		var out, errOut bytes.Buffer
		o := SelectOptions{Query: "api", In: strings.NewReader(answer), Err: &errOut, NoRecord: true}
		if err := SelectFolder(&out, o); err == nil {
			t.Errorf("SelectFolder() with answer %q error = nil, want an error", answer)
		}
		if out.String() != "" {
			t.Errorf("out = %q, want nothing chosen", out.String())
		}
	}
}

func TestSelectFolder_UsesAPicker(t *testing.T) {
	selectable(t)

	// A stand-in for fzf: it reads the list on stdin and writes one line back.
	// Shell built-ins only, since this test empties PATH.
	dir := t.TempDir()
	picker := filepath.Join(dir, "fakepicker")
	script := "#!/bin/sh\nwhile read -r line; do\n  case \"$line\" in api-gateway) echo \"$line\" ;; esac\ndone\n"
	if err := os.WriteFile(picker, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	o := SelectOptions{Query: "api", With: picker, Err: &errOut, NoRecord: true}
	if err := SelectFolder(&out, o); err != nil {
		t.Fatalf("SelectFolder() error = %v", err)
	}
	if strings.TrimSpace(out.String()) != "/tmp/api-gateway" {
		t.Errorf("out = %q, want what the picker chose", out.String())
	}
}

func TestSelectFolder_PickerChoosingNothing(t *testing.T) {
	selectable(t)

	dir := t.TempDir()
	picker := filepath.Join(dir, "fakepicker")
	// Exiting non-zero is how a filter says "never mind".
	if err := os.WriteFile(picker, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	o := SelectOptions{Query: "api", With: picker, Err: &errOut, NoRecord: true}
	if err := SelectFolder(&out, o); err == nil {
		t.Error("SelectFolder() error = nil, want it to report that nothing was chosen")
	}
	if out.String() != "" {
		t.Errorf("out = %q, want nothing printed", out.String())
	}
}

func TestSelectFolder_Tags(t *testing.T) {
	selectable(t)

	var out, errOut bytes.Buffer
	o := SelectOptions{Tags: []string{"work"}, In: strings.NewReader("1\n"), Err: &errOut, NoRecord: true}
	if err := SelectFolder(&out, o); err != nil {
		t.Fatalf("SelectFolder() error = %v", err)
	}
	if strings.Contains(errOut.String(), "notes") {
		t.Errorf("stderr = %q, want the untagged project left out", errOut.String())
	}
}

func TestSelectFolder_NothingToChooseFrom(t *testing.T) {
	useHistoryFor(t)
	lazypath.SetTestConfig(lazypath.Config{})
	t.Cleanup(lazypath.ResetTestConfig)

	var out, errOut bytes.Buffer
	if err := SelectFolder(&out, SelectOptions{Err: &errOut}); err == nil {
		t.Error("SelectFolder() with no projects error = nil, want an error")
	}
	if err := SelectFolder(&out, SelectOptions{Query: "nope", Err: &errOut}); err == nil {
		t.Error("SelectFolder() with no match error = nil, want an error")
	}
}

func TestSelectFolder_RecordsTheJump(t *testing.T) {
	selectable(t)

	var out, errOut bytes.Buffer
	o := SelectOptions{Query: "notes", In: strings.NewReader(""), Err: &errOut}
	if err := SelectFolder(&out, o); err != nil {
		t.Fatalf("SelectFolder() error = %v", err)
	}

	// Choosing is a jump, so `pj -` can come back from it.
	visits := lazypath.ReadHistory()
	if len(visits) != 1 || visits[0].Name != "notes" {
		t.Errorf("history = %v, want the choice remembered", visits)
	}
}

func TestPickerCommand(t *testing.T) {
	t.Setenv("PROJEKT_PICKER", "")
	t.Setenv("PATH", t.TempDir())

	if got := pickerCommand(""); got != nil {
		t.Errorf("pickerCommand() = %v, want nothing when none is installed", got)
	}
	t.Setenv("PROJEKT_PICKER", "myfilter --height 40%")
	if got := pickerCommand(""); len(got) != 3 || got[0] != "myfilter" {
		t.Errorf("pickerCommand() = %v, want the environment, arguments kept", got)
	}
	if got := pickerCommand("other"); len(got) != 1 || got[0] != "other" {
		t.Errorf("pickerCommand() = %v, want the argument to win", got)
	}
}
