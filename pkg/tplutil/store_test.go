package tplutil

import (
	"os"
	"path/filepath"
	"testing"
)

// useStore points the package at a temporary template folder for one test.
func useStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	previous := TemplateDir
	TemplateDir = dir
	t.Setenv("PROJEKT_TEMPLATE_DIR", "")
	t.Cleanup(func() { TemplateDir = previous })
	return dir
}

func writeTemplate(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func TestDir_Default(t *testing.T) {
	previous := TemplateDir
	TemplateDir = ""
	t.Setenv("PROJEKT_TEMPLATE_DIR", "")
	t.Cleanup(func() { TemplateDir = previous })

	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir() error = %v", err)
	}
	if dir != DefaultTemplateDir() {
		t.Errorf("Dir() = %v, want %v", dir, DefaultTemplateDir())
	}
}

func TestDir_FromEnv(t *testing.T) {
	previous := TemplateDir
	TemplateDir = ""
	t.Setenv("PROJEKT_TEMPLATE_DIR", "/tmp/templates/")
	t.Cleanup(func() { TemplateDir = previous })

	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir() error = %v", err)
	}
	if dir != "/tmp/templates" {
		t.Errorf("Dir() = %v, want /tmp/templates", dir)
	}
}

func TestDir_FlagWinsOverEnv(t *testing.T) {
	previous := TemplateDir
	TemplateDir = "/tmp/from-flag"
	t.Setenv("PROJEKT_TEMPLATE_DIR", "/tmp/from-env")
	t.Cleanup(func() { TemplateDir = previous })

	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir() error = %v", err)
	}
	if dir != "/tmp/from-flag" {
		t.Errorf("Dir() = %v, want /tmp/from-flag", dir)
	}
}

func TestList_Empty(t *testing.T) {
	useStore(t)

	templates, err := List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(templates) != 0 {
		t.Errorf("List() = %v, want no template", templates)
	}
}

func TestList_MissingStoreIsNotAnError(t *testing.T) {
	previous := TemplateDir
	TemplateDir = filepath.Join(t.TempDir(), "does-not-exist")
	t.Setenv("PROJEKT_TEMPLATE_DIR", "")
	t.Cleanup(func() { TemplateDir = previous })

	templates, err := List()
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if len(templates) != 0 {
		t.Errorf("List() = %v, want no template", templates)
	}
}

func TestList_FilesAndFolders(t *testing.T) {
	dir := useStore(t)
	writeTemplate(t, filepath.Join(dir, "license.tmpl"), "MIT")
	writeTemplate(t, filepath.Join(dir, "Makefile"), "all:")
	writeTemplate(t, filepath.Join(dir, "go-cli", "main.go.tmpl"), "package main")
	writeTemplate(t, filepath.Join(dir, ".hidden"), "ignored")

	templates, err := List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	want := []struct {
		name string
		kind Kind
	}{
		{"Makefile", KindFile},
		{"go-cli", KindDir},
		{"license", KindFile},
	}
	if len(templates) != len(want) {
		t.Fatalf("List() returned %d templates, want %d: %v", len(templates), len(want), templates)
	}
	for i, expected := range want {
		if templates[i].Name != expected.name {
			t.Errorf("List()[%d].Name = %v, want %v", i, templates[i].Name, expected.name)
		}
		if templates[i].Kind != expected.kind {
			t.Errorf("List()[%d].Kind = %v, want %v", i, templates[i].Kind, expected.kind)
		}
	}
}

func TestGet(t *testing.T) {
	dir := useStore(t)
	writeTemplate(t, filepath.Join(dir, "license.tmpl"), "MIT")

	tpl, err := Get("license")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if tpl.Path != filepath.Join(dir, "license.tmpl") {
		t.Errorf("Get().Path = %v, want %v", tpl.Path, filepath.Join(dir, "license.tmpl"))
	}
	if tpl.IsDir() {
		t.Error("Get().IsDir() = true, want false")
	}
}

func TestGet_Errors(t *testing.T) {
	dir := useStore(t)
	writeTemplate(t, filepath.Join(dir, "license.tmpl"), "MIT")

	for _, name := range []string{"", "unknown", "../license", "sub/license", "..", "."} {
		if _, err := Get(name); err == nil {
			t.Errorf("Get(%q) error = nil, want an error", name)
		}
	}
}
