package tplutil

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRender_CustomDelims(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "chart", ".vars.yaml"), "delims: [\"<%\", \"%>\"]\n")
	writeTemplate(t, filepath.Join(store, "chart", "values.yaml.tmpl"),
		"name: <% .Name %>\nimage: {{ .Values.image | quote }}\n")

	tpl, err := Get("chart")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := t.TempDir()
	if _, err := Render(RenderOptions{Template: tpl, Dest: dest, Name: "web"}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	got := readFile(t, filepath.Join(dest, "values.yaml"))
	if got != "name: web\nimage: {{ .Values.image | quote }}\n" {
		t.Errorf("rendered = %q, want the foreign braces left alone", got)
	}
}

func TestRender_CustomDelimsReachPathSegments(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "chart", ".vars.yaml"), "delims: [\"<%\", \"%>\"]\n")
	writeTemplate(t, filepath.Join(store, "chart", "<% .Name %>", "main.txt.tmpl"), "x\n")

	tpl, err := Get("chart")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := t.TempDir()
	if _, err := Render(RenderOptions{Template: tpl, Dest: dest, Name: "web"}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if got := readFile(t, filepath.Join(dest, "web", "main.txt")); got != "x\n" {
		t.Errorf("Render() did not name the folder from the custom delimiters: %q", got)
	}
}

func TestRender_SharedPartialsKeepTheDefaultDelims(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, PartialsDir, "header.tmpl"), "# {{ .Name }}")
	writeTemplate(t, filepath.Join(store, "chart", ".vars.yaml"), "delims: [\"<%\", \"%>\"]\n")
	writeTemplate(t, filepath.Join(store, "chart", "out.txt.tmpl"), "<% template \"header\" . %>\n")

	tpl, err := Get("chart")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := t.TempDir()
	if _, err := Render(RenderOptions{Template: tpl, Dest: dest, Name: "web"}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if got := strings.TrimSpace(readFile(t, filepath.Join(dest, "out.txt"))); got != "# web" {
		t.Errorf("rendered = %q, want the store partial rendered with its own delimiters", got)
	}
}

func TestLoadManifest_RefusesABadDelimsPair(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "chart", ".vars.yaml"), "delims: [\"<%\"]\n")
	writeTemplate(t, filepath.Join(store, "chart", "out.txt.tmpl"), "x\n")

	tpl, err := Get("chart")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if _, err := LoadManifest(tpl); err == nil || !strings.Contains(err.Error(), "exactly two") {
		t.Errorf("LoadManifest() error = %v, want it to refuse a single delimiter", err)
	}
}

func TestVars_InferredThroughCustomDelims(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "chart", ".vars.yaml"), "delims: [\"<%\", \"%>\"]\n")
	writeTemplate(t, filepath.Join(store, "chart", "out.txt.tmpl"), "<% .Values.registry %>\n")

	tpl, err := Get("chart")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	vars, err := Vars(tpl)
	if err != nil {
		t.Fatalf("Vars() error = %v", err)
	}
	if len(vars) != 1 || vars[0].Name != "registry" {
		t.Errorf("Vars() = %v, want registry read through the custom delimiters", vars)
	}
}
