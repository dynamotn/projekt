package tplutil

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(data)
}

func TestRender_File(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "license.tmpl"), "Copyright {{ .Year }} {{ .Values.author }} for {{ .Name }}\n")

	tpl, err := Get("license")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := filepath.Join(t.TempDir(), "LICENSE.txt")

	written, err := Render(RenderOptions{
		Template: tpl,
		Dest:     dest,
		Values:   Values{"author": "Jane"},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(written) != 1 || written[0] != dest {
		t.Fatalf("Render() = %v, want [%s]", written, dest)
	}

	content := readFile(t, dest)
	if !strings.Contains(content, "Jane") || !strings.Contains(content, "LICENSE") {
		t.Errorf("rendered content = %q, want the author and the file name", content)
	}
}

func TestRender_FileIntoFolderKeepsTemplateName(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "Makefile.tmpl"), "all:\n")

	tpl, err := Get("Makefile")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := t.TempDir()

	written, err := Render(RenderOptions{Template: tpl, Dest: dest})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	want := filepath.Join(dest, "Makefile")
	if len(written) != 1 || written[0] != want {
		t.Errorf("Render() = %v, want [%s]", written, want)
	}
}

func TestRender_NameFlagOverridesFileName(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "doc.tmpl"), "{{ .Name }}\n")

	tpl, err := Get("doc")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := t.TempDir()

	written, err := Render(RenderOptions{Template: tpl, Dest: dest, Name: "README.md"})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	want := filepath.Join(dest, "README.md")
	if written[0] != want {
		t.Fatalf("Render() = %v, want [%s]", written, want)
	}
	if got := readFile(t, want); got != "README.md\n" {
		t.Errorf("rendered content = %q, want the name given by --name", got)
	}
}

func TestRender_RefusesToOverwrite(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "doc.tmpl"), "new\n")

	tpl, err := Get("doc")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := filepath.Join(t.TempDir(), "doc")
	if err := os.WriteFile(dest, []byte("old\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := Render(RenderOptions{Template: tpl, Dest: dest}); err == nil {
		t.Fatal("Render() error = nil, want an error on an existing file")
	}
	if got := readFile(t, dest); got != "old\n" {
		t.Errorf("content = %q, want the existing file to be untouched", got)
	}

	if _, err := Render(RenderOptions{Template: tpl, Dest: dest, Force: true}); err != nil {
		t.Fatalf("Render() with Force error = %v", err)
	}
	if got := readFile(t, dest); got != "new\n" {
		t.Errorf("content = %q, want the file to be overwritten with Force", got)
	}
}

func TestRender_DryRunWritesNothing(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "doc.tmpl"), "hello\n")

	tpl, err := Get("doc")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := filepath.Join(t.TempDir(), "doc")

	var buf bytes.Buffer
	if _, err := Render(RenderOptions{Template: tpl, Dest: dest, DryRun: true, Out: &buf}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if buf.String() != "hello\n" {
		t.Errorf("dry run output = %q, want the rendered content", buf.String())
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Error("Render() with DryRun created a file")
	}
}

func TestRender_Folder(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "go-cli", "go.mod.tmpl"), "module {{ .Values.module }}\n")
	writeTemplate(t, filepath.Join(store, "go-cli", "cmd", "{{ .Name }}", "main.go.tmpl"), "package main // {{ .Project }}\n")
	writeTemplate(t, filepath.Join(store, "go-cli", ".gitignore.tmpl"), "/{{ .Name }}\n")
	writeTemplate(t, filepath.Join(store, "go-cli", ".git", "config"), "[core]\n")

	tpl, err := Get("go-cli")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !tpl.IsDir() {
		t.Fatal("Get().IsDir() = false, want a folder template")
	}
	dest := filepath.Join(t.TempDir(), "myapp")

	written, err := Render(RenderOptions{
		Template: tpl,
		Dest:     dest,
		Name:     "myapp",
		Values:   Values{"module": "example.com/myapp"},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(written) != 3 {
		t.Fatalf("Render() wrote %v, want 3 files", written)
	}

	if got := readFile(t, filepath.Join(dest, "go.mod")); got != "module example.com/myapp\n" {
		t.Errorf("go.mod = %q", got)
	}
	// The path segment itself is a template and the .tmpl suffix is stripped.
	main := filepath.Join(dest, "cmd", "myapp", "main.go")
	if got := readFile(t, main); got != "package main // myapp\n" {
		t.Errorf("%s = %q", main, got)
	}
	// A dotfile is part of the project, repository metadata is not.
	if got := readFile(t, filepath.Join(dest, ".gitignore")); got != "/myapp\n" {
		t.Errorf(".gitignore = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); !os.IsNotExist(err) {
		t.Error("Render() copied the .git folder of the template")
	}
}

func TestRender_FolderRejectsEscapingPath(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "bad", "{{ .Values.name }}", "file.tmpl"), "x")

	tpl, err := Get("bad")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := t.TempDir()

	for _, name := range []string{"../escaped", "", "a/b"} {
		if _, err := Render(RenderOptions{Template: tpl, Dest: dest, Values: Values{"name": name}}); err == nil {
			t.Errorf("Render() with name %q error = nil, want an error", name)
		}
	}
}

func TestRender_MissingValueStaysEmpty(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "doc.tmpl"), "[{{ .Values.missing | default \"fallback\" }}]\n")

	tpl, err := Get("doc")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	var buf bytes.Buffer
	if _, err := Render(RenderOptions{Template: tpl, Dest: filepath.Join(t.TempDir(), "doc"), DryRun: true, Out: &buf}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if buf.String() != "[fallback]\n" {
		t.Errorf("output = %q, want the sprig default to kick in", buf.String())
	}
}

func TestRender_SprigFunctionsAreAvailable(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "doc.tmpl"), "{{ .Values.name | upper | quote }}\n")

	tpl, err := Get("doc")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	var buf bytes.Buffer
	if _, err := Render(RenderOptions{
		Template: tpl,
		Dest:     filepath.Join(t.TempDir(), "doc"),
		Values:   Values{"name": "projekt"},
		DryRun:   true,
		Out:      &buf,
	}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if buf.String() != "\"PROJEKT\"\n" {
		t.Errorf("output = %q, want the sprig pipeline applied", buf.String())
	}
}

func TestRender_BrokenTemplate(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "doc.tmpl"), "{{ .Values.name \n")

	tpl, err := Get("doc")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := filepath.Join(t.TempDir(), "doc")

	if _, err := Render(RenderOptions{Template: tpl, Dest: dest}); err == nil {
		t.Fatal("Render() error = nil, want a parse error")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Error("Render() wrote a file although the template is broken")
	}
}
