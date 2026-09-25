package root

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/dynamo-tools/projekt/pkg/tplutil"
)

// store writes a template store for one test and points the package at it.
func store(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
	}

	previous := tplutil.TemplateDir
	tplutil.TemplateDir = dir
	t.Setenv("PROJEKT_TEMPLATE_DIR", "")
	t.Cleanup(func() { tplutil.TemplateDir = previous })
	return dir
}

func TestNewCmd_Interactive(t *testing.T) {
	store(t, map[string]string{
		"note.tmpl": "{{ .Values.title }} / {{ .Values.author }}\n",
		"note" + tplutil.VarsFile: `
vars:
  - name: title
    prompt: What is it called
    required: true
  - name: author
    default: anonymous
`,
	})

	var out, errOut bytes.Buffer
	target := filepath.Join(t.TempDir(), "note.md")

	cmd := NewTemplateNewCmd(&out)
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetIn(strings.NewReader("A title\n\n"))
	cmd.SetArgs([]string{"note", target, "--interactive"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "A title / anonymous\n" {
		t.Errorf("rendered = %q", data)
	}
	// The questions belong on stderr, so a piped dry run stays clean.
	if !strings.Contains(errOut.String(), "What is it called") {
		t.Errorf("stderr = %q, want the question", errOut.String())
	}
	if strings.Contains(out.String(), "What is it called") {
		t.Errorf("stdout = %q, want no question on stdout", out.String())
	}
}

func TestNewCmd_InteractiveDoesNotAskForWhatIsSet(t *testing.T) {
	store(t, map[string]string{
		"note.tmpl": "{{ .Values.title }}/{{ .Values.author }}\n",
	})

	var out, errOut bytes.Buffer
	target := filepath.Join(t.TempDir(), "note.md")

	cmd := NewTemplateNewCmd(&out)
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetIn(strings.NewReader("Jane\n"))
	cmd.SetArgs([]string{"note", target, "-i", "--set", "title=Preset"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "Preset/Jane\n" {
		t.Errorf("rendered = %q, want the preset title and the typed author", data)
	}
	if strings.Contains(errOut.String(), "title") {
		t.Errorf("stderr = %q, want no question for a value given with --set", errOut.String())
	}
}

func TestNewCmd_InteractiveWithoutValues(t *testing.T) {
	store(t, map[string]string{"static.tmpl": "nothing to ask\n"})

	var out, errOut bytes.Buffer
	target := filepath.Join(t.TempDir(), "static.txt")

	cmd := NewTemplateNewCmd(&out)
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetIn(strings.NewReader(""))
	cmd.SetArgs([]string{"static", target, "-i"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(errOut.String(), "takes no values") {
		t.Errorf("stderr = %q, want it said there is nothing to ask", errOut.String())
	}
}

func TestNewCmd_NonInteractiveAsksNothing(t *testing.T) {
	store(t, map[string]string{
		"note.tmpl": "{{ .Values.title | default \"untitled\" }}\n",
	})

	var out, errOut bytes.Buffer
	target := filepath.Join(t.TempDir(), "note.md")

	cmd := NewTemplateNewCmd(&out)
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetIn(strings.NewReader("never read\n"))
	cmd.SetArgs([]string{"note", target})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "untitled\n" {
		t.Errorf("rendered = %q, want the template default", data)
	}
	if errOut.String() != "" {
		t.Errorf("stderr = %q, want nothing asked without --interactive", errOut.String())
	}
}
