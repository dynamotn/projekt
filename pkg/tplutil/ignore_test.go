package tplutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseIgnore_Matching(t *testing.T) {
	set := ParseIgnore(`
# a comment
*.log
build/
docs/**/draft.md
!keep.log
`)

	cases := []struct {
		path  string
		isDir bool
		want  bool
	}{
		{"app.log", false, true},
		{"cmd/app.log", false, true},
		{"keep.log", false, false},
		{"build", true, true},
		{"build", false, false},
		{"docs/a/b/draft.md", false, true},
		{"docs/draft.md", false, true},
		{"main.go", false, false},
	}
	for _, c := range cases {
		if got := set.Match(c.path, c.isDir); got != c.want {
			t.Errorf("Match(%q, dir=%v) = %v, want %v", c.path, c.isDir, got, c.want)
		}
	}
}

func TestParseIgnore_AnchoredPatternStaysAtTheRoot(t *testing.T) {
	set := ParseIgnore("/README.md\n")

	if !set.Match("README.md", false) {
		t.Error("Match(README.md) = false, want the root file ignored")
	}
	if set.Match("doc/README.md", false) {
		t.Error("Match(doc/README.md) = true, want an anchored pattern to stay at the root")
	}
}

func TestRender_IgnoreLeavesFilesOut(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "app", "main.go.tmpl"), "package main\n")
	writeTemplate(t, filepath.Join(store, "app", ".github", "workflows", "ci.yml.tmpl"), "on: push\n")
	writeTemplate(t, filepath.Join(store, "app", IgnoreFile), "{{ if not .Values.ci }}.github/{{ end }}\n")

	tpl, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	without := t.TempDir()
	if _, err := Render(RenderOptions{Template: tpl, Dest: without, Name: "app"}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(without, ".github")); !os.IsNotExist(err) {
		t.Errorf("Stat(.github) error = %v, want it left out", err)
	}
	if _, err := os.Stat(filepath.Join(without, "main.go")); err != nil {
		t.Errorf("Stat(main.go) error = %v, want it written", err)
	}

	with := t.TempDir()
	if _, err := Render(RenderOptions{Template: tpl, Dest: with, Name: "app", Values: Values{"ci": true}}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(with, ".github", "workflows", "ci.yml")); err != nil {
		t.Errorf("Stat(ci.yml) error = %v, want it written when ci is set", err)
	}
	if _, err := os.Stat(filepath.Join(with, IgnoreFile)); !os.IsNotExist(err) {
		t.Errorf("the ignore file was written to the output, it describes the template")
	}
}
