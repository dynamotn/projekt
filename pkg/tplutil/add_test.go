package tplutil

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

func TestAdd_File(t *testing.T) {
	store := useStore(t)
	source := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(source, []byte("package {{ .Name }}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	path, err := Add(AddOptions{Source: source})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	want := filepath.Join(store, "main.go.tmpl")
	if path != want {
		t.Fatalf("Add() = %v, want %v", path, want)
	}
	// Template actions of the source are kept as they are.
	if got := readFile(t, want); got != "package {{ .Name }}\n" {
		t.Errorf("stored template = %q", got)
	}

	tpl, err := Get("main.go")
	if err != nil {
		t.Fatalf("Get() after Add() error = %v", err)
	}
	if tpl.Kind != KindFile {
		t.Errorf("Get().Kind = %v, want %v", tpl.Kind, KindFile)
	}
}

func TestAdd_RefusesToReplaceWithoutForce(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "doc.tmpl"), "old\n")
	source := filepath.Join(t.TempDir(), "doc")
	if err := os.WriteFile(source, []byte("new\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := Add(AddOptions{Source: source}); err == nil {
		t.Fatal("Add() error = nil, want an error on an existing template")
	}
	if got := readFile(t, filepath.Join(store, "doc.tmpl")); got != "old\n" {
		t.Errorf("template = %q, want it untouched", got)
	}

	if _, err := Add(AddOptions{Source: source, Force: true}); err != nil {
		t.Fatalf("Add() with Force error = %v", err)
	}
	if got := readFile(t, filepath.Join(store, "doc.tmpl")); got != "new\n" {
		t.Errorf("template = %q, want it replaced", got)
	}
}

func TestAdd_Folder(t *testing.T) {
	store := useStore(t)
	source := filepath.Join(t.TempDir(), "project")
	writeTemplate(t, filepath.Join(source, "go.mod"), "module example.com/x\n")
	writeTemplate(t, filepath.Join(source, "cmd", "main.go"), "package main\n")
	writeTemplate(t, filepath.Join(source, ".git", "config"), "[core]\n")

	path, err := Add(AddOptions{Source: source, Name: "go-cli"})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if path != filepath.Join(store, "go-cli") {
		t.Fatalf("Add() = %v, want %v", path, filepath.Join(store, "go-cli"))
	}

	if got := readFile(t, filepath.Join(store, "go-cli", "cmd", "main.go")); got != "package main\n" {
		t.Errorf("copied file = %q", got)
	}
	if _, err := os.Stat(filepath.Join(store, "go-cli", ".git")); !os.IsNotExist(err) {
		t.Error("Add() copied the .git folder of the source")
	}
}

func TestAdd_Errors(t *testing.T) {
	store := useStore(t)
	source := filepath.Join(t.TempDir(), "doc")
	if err := os.WriteFile(source, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := Add(AddOptions{Source: filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Error("Add() with a missing source error = nil, want an error")
	}
	if _, err := Add(AddOptions{Source: source, Name: "../escape"}); err == nil {
		t.Error("Add() with an escaping name error = nil, want an error")
	}
	// The store cannot import itself, that would recurse.
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if _, err := Add(AddOptions{Source: store, Name: "self"}); err == nil {
		t.Error("Add() of the store itself error = nil, want an error")
	}
}

func TestShowTemplate(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "doc.tmpl"), "hello {{ .Name }}\n")
	writeTemplate(t, filepath.Join(store, "tree", "a.tmpl"), "A\n")
	writeTemplate(t, filepath.Join(store, "tree", "sub", "b.tmpl"), "B")

	file, err := Get("doc")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	var buf bytes.Buffer
	if err := ShowTemplate(&buf, file); err != nil {
		t.Fatalf("ShowTemplate() error = %v", err)
	}
	if buf.String() != "hello {{ .Name }}\n" {
		t.Errorf("ShowTemplate() = %q", buf.String())
	}

	dir, err := Get("tree")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	buf.Reset()
	if err := ShowTemplate(&buf, dir); err != nil {
		t.Fatalf("ShowTemplate() error = %v", err)
	}
	output := buf.String()
	for _, want := range []string{"# a.tmpl", "A", filepath.Join("sub", "b.tmpl"), "B"} {
		if !strings.Contains(output, want) {
			t.Errorf("ShowTemplate() = %q, want it to contain %q", output, want)
		}
	}
}

func TestListTemplates(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "license.tmpl"), "MIT")
	writeTemplate(t, filepath.Join(store, "go-cli", "go.mod.tmpl"), "module x")

	var buf bytes.Buffer
	o := &ListOption{}
	o.NoColor = true
	if err := ListTemplates(&buf, o); err != nil {
		t.Fatalf("ListTemplates() error = %v", err)
	}
	for _, want := range []string{"NAME", "license", "go-cli", "dir", "file"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("ListTemplates() = %q, want it to contain %q", buf.String(), want)
		}
	}

	// What a script reads: names alone, no headers, no borders.
	buf.Reset()
	o = &ListOption{NamesOnly: true}
	o.Output = cli.OutputTSV
	o.NoHeaders = true
	if err := ListTemplates(&buf, o); err != nil {
		t.Fatalf("ListTemplates() error = %v", err)
	}
	if buf.String() != "go-cli\nlicense\n" {
		t.Errorf("ListTemplates() names only = %q", buf.String())
	}
}

func TestListTemplates_JSON(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "license.tmpl"), "MIT")

	var buf bytes.Buffer
	o := &ListOption{}
	o.Output = cli.OutputJSON
	if err := ListTemplates(&buf, o); err != nil {
		t.Fatalf("ListTemplates() error = %v", err)
	}

	var listing []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &listing); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", buf.String(), err)
	}
	if len(listing) != 1 || listing[0]["name"] != "license" || listing[0]["kind"] != "file" {
		t.Errorf("ListTemplates() json = %#v", listing)
	}
}

func TestListTemplates_EmptyJSONIsAnArray(t *testing.T) {
	useStore(t)

	var buf bytes.Buffer
	o := &ListOption{}
	o.Output = cli.OutputJSON
	if err := ListTemplates(&buf, o); err != nil {
		t.Fatalf("ListTemplates() error = %v", err)
	}
	// A program reading the listing should only ever meet one shape.
	if strings.TrimSpace(buf.String()) != "[]" {
		t.Errorf("ListTemplates() on an empty store = %q, want []", buf.String())
	}
}

func TestListTemplates_Empty(t *testing.T) {
	useStore(t)

	var buf bytes.Buffer
	if err := ListTemplates(&buf, &ListOption{}); err != nil {
		t.Fatalf("ListTemplates() error = %v", err)
	}
	if buf.String() != "" {
		t.Errorf("ListTemplates() = %q, want nothing on an empty store", buf.String())
	}
}
