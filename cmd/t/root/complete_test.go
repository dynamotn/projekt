package root

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/tplutil"
)

var completionVars = []tplutil.Var{
	{Name: "module", Prompt: "Go module path", Default: "example.com/{{ .Name }}"},
	{Name: "author", Default: "{{ .User }}"},
	{Name: "runner", Type: tplutil.VarChoice, Choices: []string{"ubuntu-latest", "macos-latest"}, Default: "ubuntu-latest"},
	{Name: "coverage", Type: tplutil.VarBool},
	{Name: "netDays", Type: tplutil.VarInt, Default: "14"},
	{Name: "module", Prompt: "a duplicate"},
}

func TestCompleteValueAssignment_Keys(t *testing.T) {
	got, directive := CompleteValueAssignment(completionVars, "")

	if directive&cobra.ShellCompDirectiveNoSpace == 0 {
		t.Error("directive has no NoSpace, so the shell would end the word at the =")
	}
	if len(got) != 5 {
		t.Fatalf("CompleteValueAssignment() = %v, want one entry per distinct key", got)
	}

	want := map[string]string{
		"module=":   "Go module path",
		"author=":   "",
		"runner=":   "choice [ubuntu-latest]",
		"coverage=": "bool",
		"netDays=":  "int [14]",
	}
	for _, entry := range got {
		key, description, _ := strings.Cut(entry, "\t")
		expected, ok := want[key]
		if !ok {
			t.Errorf("unexpected completion %q", entry)
			continue
		}
		if description != expected {
			t.Errorf("%s described as %q, want %q", key, description, expected)
		}
	}
}

func TestCompleteValueAssignment_LeavesATemplatedDefaultOut(t *testing.T) {
	got, _ := CompleteValueAssignment(completionVars, "")
	for _, entry := range got {
		if strings.Contains(entry, "{{") {
			t.Errorf("completion %q shows a raw template as a default", entry)
		}
	}
}

func TestCompleteValueAssignment_Answers(t *testing.T) {
	got, directive := CompleteValueAssignment(completionVars, "runner=")
	if directive&cobra.ShellCompDirectiveNoFileComp == 0 {
		t.Error("directive allows file completion for a choice")
	}
	if len(got) != 2 || got[0] != "runner=ubuntu-latest" || got[1] != "runner=macos-latest" {
		t.Errorf("CompleteValueAssignment() = %v, want the choices", got)
	}

	got, _ = CompleteValueAssignment(completionVars, "coverage=")
	if len(got) != 2 || got[0] != "coverage=true" {
		t.Errorf("CompleteValueAssignment() = %v, want true and false", got)
	}

	// A free-text value has nothing to offer, and must not fall back to files.
	got, directive = CompleteValueAssignment(completionVars, "module=")
	if len(got) != 0 || directive&cobra.ShellCompDirectiveNoFileComp == 0 {
		t.Errorf("CompleteValueAssignment() = %v, %v, want nothing and no file completion", got, directive)
	}
}

func writeTemplateFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func TestCompleteMoreTemplateNames_LeavesOutWhatIsNamedAndWhatCannotApply(t *testing.T) {
	store := t.TempDir()
	previous := tplutil.TemplateDir
	tplutil.TemplateDir = store
	t.Setenv("PROJEKT_TEMPLATE_DIR", "")
	t.Cleanup(func() { tplutil.TemplateDir = previous })

	writeTemplateFile(t, store+"/app/main.txt.tmpl", "x\n")
	writeTemplateFile(t, store+"/ci/job.yml.tmpl", "x\n")
	writeTemplateFile(t, store+"/license.tmpl", "x\n")

	got, _ := completeMoreTemplateNames(&cobra.Command{}, []string{"app"}, "")
	if len(got) != 1 || !strings.HasPrefix(got[0], "ci\t") {
		t.Errorf("completeMoreTemplateNames() = %v, want the folder template that is not named yet", got)
	}
}
