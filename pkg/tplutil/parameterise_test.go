package tplutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseReplacements_LongestLiteralFirst(t *testing.T) {
	got, err := ParseReplacements([]string{"myapp=Name", "example.com/myapp=Values.module", "x=.Values.a.b"})
	if err != nil {
		t.Fatalf("ParseReplacements() error = %v", err)
	}
	if got[0].Literal != "example.com/myapp" {
		t.Errorf("first = %q, want the longest literal, so it is not eaten by a shorter one", got[0].Literal)
	}
	if got[2].Expr != "Values.a.b" {
		t.Errorf("Expr = %q, want the leading dot stripped", got[2].Expr)
	}
	if got[0].Action() != "{{ .Values.module }}" {
		t.Errorf("Action() = %q", got[0].Action())
	}
}

func TestParseReplacements_SplitsOnTheLastEquals(t *testing.T) {
	got, err := ParseReplacements([]string{"VERSION=1.0=Values.version"})
	if err != nil {
		t.Fatalf("ParseReplacements() error = %v", err)
	}
	if got[0].Literal != "VERSION=1.0" || got[0].Expr != "Values.version" {
		t.Errorf("ParseReplacements() = %+v, want the literal to keep its own =", got[0])
	}
}

func TestParseReplacements_Refusals(t *testing.T) {
	for _, bad := range []string{"noequals", "=Name", "myapp=", "myapp=Values.", "myapp=not a path", "myapp=1bad"} {
		if _, err := ParseReplacements([]string{bad}); err == nil {
			t.Errorf("ParseReplacements(%q) error = nil, want a refusal", bad)
		}
	}
}

func TestAdd_ParameterisesContentsAndNames(t *testing.T) {
	store := useStore(t)
	project := t.TempDir()
	writeTemplate(t, filepath.Join(project, "go.mod"), "module example.com/myapp\n")
	writeTemplate(t, filepath.Join(project, "cmd", "myapp", "main.go"),
		"// myapp, by Jane Doe\npackage main // example.com/myapp\n")

	added, err := Add(AddOptions{
		Source: project,
		Name:   "go-cli",
		Replace: mustParse(t,
			"myapp=Name",
			"example.com/myapp=Values.module",
			"Jane Doe=Values.author",
		),
	})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if got := readFile(t, filepath.Join(store, "go-cli", "go.mod")); got != "module {{ .Values.module }}\n" {
		t.Errorf("go.mod = %q, want the longer literal replaced whole", got)
	}
	// The folder was named after the project, so it is named after .Name now.
	main := filepath.Join(store, "go-cli", "cmd", "{{ .Name }}", "main.go")
	if got := readFile(t, main); !strings.Contains(got, "// {{ .Name }}, by {{ .Values.author }}") {
		t.Errorf("main.go = %q", got)
	}
	if !strings.Contains(readFile(t, main), "{{ .Values.module }}") {
		t.Errorf("main.go did not get the module path replaced")
	}
	if added.Files != 2 || added.Substitutions < 5 {
		t.Errorf("Add() = %+v, want two files and every literal counted", added)
	}
}

func TestAdd_WritesTheQuestionsTheReplacementsImply(t *testing.T) {
	store := useStore(t)
	project := t.TempDir()
	writeTemplate(t, filepath.Join(project, "go.mod"), "module example.com/myapp\n")

	added, err := Add(AddOptions{
		Source:  project,
		Name:    "go-cli",
		Replace: mustParse(t, "myapp=Name", "example.com/myapp=Values.module"),
	})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if added.Manifest != filepath.Join(store, "go-cli", VarsFile) {
		t.Fatalf("Manifest = %q, want it inside the folder template", added.Manifest)
	}

	tpl, err := Get("go-cli")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	manifest, err := LoadManifest(tpl)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	if len(manifest.Vars) != 1 {
		t.Fatalf("Vars = %+v, want only the .Values target to become a question", manifest.Vars)
	}
	if manifest.Vars[0].Name != "module" || manifest.Vars[0].Default != "example.com/myapp" {
		t.Errorf("Vars[0] = %+v, want the project's own value as the default", manifest.Vars[0])
	}
}

func TestAdd_LeavesAnExistingManifestAlone(t *testing.T) {
	store := useStore(t)
	project := t.TempDir()
	writeTemplate(t, filepath.Join(project, "go.mod"), "module example.com/myapp\n")
	writeTemplate(t, filepath.Join(project, VarsFile), "vars:\n  - name: module\n    prompt: The real question\n")

	if _, err := Add(AddOptions{
		Source:  project,
		Name:    "go-cli",
		Replace: mustParse(t, "example.com/myapp=Values.module"),
	}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if got := readFile(t, filepath.Join(store, "go-cli", VarsFile)); !strings.Contains(got, "The real question") {
		t.Errorf("%s = %q, want the manifest the project brought kept", VarsFile, got)
	}
}

func TestAdd_LeavesBinaryFilesUntouched(t *testing.T) {
	store := useStore(t)
	project := t.TempDir()
	binary := []byte{0x7f, 'E', 'L', 'F', 0x00, 'm', 'y', 'a', 'p', 'p', 0x00}
	if err := os.WriteFile(filepath.Join(project, "myapp.bin"), binary, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := Add(AddOptions{Source: project, Name: "go-cli", Replace: mustParse(t, "myapp=Name")}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// The name is text and is rewritten; the contents are not touched.
	got, err := os.ReadFile(filepath.Join(store, "go-cli", "{{ .Name }}.bin"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != string(binary) {
		t.Errorf("the binary was rewritten: %q", got)
	}
}

func TestAdd_WithoutReplaceCopiesAsBefore(t *testing.T) {
	store := useStore(t)
	project := t.TempDir()
	writeTemplate(t, filepath.Join(project, "go.mod"), "module example.com/myapp\n")

	added, err := Add(AddOptions{Source: project, Name: "go-cli"})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if added.Substitutions != 0 || added.Manifest != "" {
		t.Errorf("Add() = %+v, want a plain copy", added)
	}
	if got := readFile(t, filepath.Join(store, "go-cli", "go.mod")); got != "module example.com/myapp\n" {
		t.Errorf("go.mod = %q, want it copied as it is", got)
	}
}

func TestAdd_ParameterisesASingleFile(t *testing.T) {
	store := useStore(t)
	source := filepath.Join(t.TempDir(), "LICENSE")
	writeTemplate(t, source, "Copyright 2026 Jane Doe\n")

	added, err := Add(AddOptions{Source: source, Replace: mustParse(t, "Jane Doe=Values.author")})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if got := readFile(t, added.Path); got != "Copyright 2026 {{ .Values.author }}\n" {
		t.Errorf("LICENSE = %q", got)
	}
	if added.Manifest != filepath.Join(store, "LICENSE"+VarsFile) {
		t.Errorf("Manifest = %q, want it beside the file template", added.Manifest)
	}
}

func mustParse(t *testing.T, arguments ...string) []Replacement {
	t.Helper()
	replacements, err := ParseReplacements(arguments)
	if err != nil {
		t.Fatalf("ParseReplacements() error = %v", err)
	}
	return replacements
}
