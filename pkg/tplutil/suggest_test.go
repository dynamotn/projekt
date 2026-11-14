package tplutil

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func suggestSource(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":    "module example.com/billing\n",
		"README.md": "# billing\nby Jane Doe\nJane's app\n",
		".env":      "TOKEN=abc123\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func suggest(t *testing.T, source, answer string) ([]Replacement, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	got, err := SuggestReplacements(source, SuggestOptions{
		Out: &out, Err: &errOut,
		Ask: func(string, io.Writer) (string, error) { return answer, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	return got, out.String(), errOut.String()
}

func TestSuggestReplacements_KeepsWhatHolds(t *testing.T) {
	got, _, errOut := suggest(t, suggestSource(t),
		"example.com/billing: module\nJane Doe: author\nnot-there: x\nbilling: Bad Key\n")
	want := []Replacement{{"example.com/billing", "Values.module"}, {"Jane Doe", "Values.author"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if !strings.Contains(errOut, "not-there") || !strings.Contains(errOut, "Bad Key") {
		t.Errorf("Err = %q, want both dropped lines named", errOut)
	}
}

func TestSuggestReplacements_PrintsAQuotedCommand(t *testing.T) {
	src := suggestSource(t)
	_, out, _ := suggest(t, src, "Jane's app: name\n")
	if want := `--replace 'Jane'\''s app=Values.name'`; !strings.Contains(out, want) {
		t.Errorf("Out = %q, want it to hold %s", out, want)
	}
	if !strings.Contains(out, "'"+src+"'") {
		t.Errorf("Out = %q, want the quoted source path", out)
	}
}

func TestSuggestReplacements_AnswerNeverRuns(t *testing.T) {
	src := suggestSource(t)
	t.Chdir(t.TempDir())
	got, _, _ := suggest(t, src, "$(touch pwned): x\n`touch pwned2`: y\n")
	if len(got) != 0 {
		t.Errorf("kept %v, want nothing", got)
	}
	for _, f := range []string{"pwned", "pwned2"} {
		if _, err := os.Stat(f); err == nil {
			t.Errorf("%s was created", f)
		}
	}
}

func TestSuggestReplacements_ShowPromptRunsNothing(t *testing.T) {
	var out bytes.Buffer
	got, err := SuggestReplacements(suggestSource(t), SuggestOptions{
		ShowPrompt: true, Out: &out, Err: &bytes.Buffer{},
		Ask: func(string, io.Writer) (string, error) { t.Fatal("Ask called"); return "", nil },
	})
	if err != nil || got != nil {
		t.Fatalf("got %v, %v", got, err)
	}
	if !strings.Contains(out.String(), "module example.com/billing") {
		t.Errorf("Out = %q, want go.mod in the prompt", out.String())
	}
}

func TestSuggestReplacements_NothingFromSecrets(t *testing.T) {
	var prompt string
	var out, errOut bytes.Buffer
	got, err := SuggestReplacements(suggestSource(t), SuggestOptions{
		Out: &out, Err: &errOut,
		Ask: func(p string, _ io.Writer) (string, error) { prompt = p; return "abc123: token\n", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(prompt, "abc123") {
		t.Error("the prompt holds the secret")
	}
	if len(got) != 0 || !strings.Contains(errOut.String(), "abc123") {
		t.Errorf("got %v, Err %q: want it dropped as not found", got, errOut.String())
	}
}

func TestSuggestReplacements_DropsDuplicateKey(t *testing.T) {
	got, _, errOut := suggest(t, suggestSource(t), "Jane Doe: n\nexample.com/billing: n\n")
	if len(got) != 1 || got[0].Literal != "Jane Doe" || !strings.Contains(errOut, "the key came earlier") {
		t.Errorf("got %v, Err %q: want only the first pair", got, errOut)
	}
}

func TestSuggestReplacements_DropsControlCharacters(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("a\rb here\nx\x1b[2Ky here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, _, errOut := suggest(t, dir, "a\rb here: one\nx\x1b[2Ky here: two\n")
	if len(got) != 0 || strings.Count(errOut, "control character") != 2 {
		t.Errorf("got %v, Err %q: want both dropped as control characters", got, errOut)
	}
}

func TestSuggestReplacements_DropsTemplateSyntax(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"),
		[]byte("{{ .Name }} and {{ .Title }}\nTitle again\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, _, errOut := suggest(t, dir, "{{: a\nName: b\nTitle: c\n")
	if len(got) != 1 || got[0].Literal != "Title" {
		t.Errorf("got %v, Err %q: want only Title (it also occurs outside an action)", got, errOut)
	}
	if strings.Count(errOut, "dropped") != 2 || !strings.Contains(errOut, "\"{{: a\": the literal is template") {
		t.Errorf("Err = %q, want {{ and Name dropped as template syntax", errOut)
	}
}

func TestSuggestReplacements_DropsShortLiterals(t *testing.T) {
	got, _, errOut := suggest(t, suggestSource(t), "by: a\n")
	if len(got) != 0 || !strings.Contains(errOut, "shorter") {
		t.Errorf("got %v, Err %q: want it dropped as too short", got, errOut)
	}
}
