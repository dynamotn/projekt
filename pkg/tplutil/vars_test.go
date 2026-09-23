package tplutil

import (
	"path/filepath"
	"reflect"
	"testing"
)

func varNames(vars []Var) []string {
	names := make([]string, 0, len(vars))
	for _, v := range vars {
		names = append(names, v.Name)
	}
	return names
}

func TestInferVars_File(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "doc.tmpl"), `
{{ .Values.title }} by {{ .Values.author.name }}
{{ with .Values.note }}note: {{ . }}{{ end }}
{{ if .Values.draft }}DRAFT{{ end }}
{{ .Values.title | upper }}
{{ printf "%s" (.Values.subtitle | default "none") }}
`)

	tpl, err := Get("doc")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	vars, err := InferVars(tpl)
	if err != nil {
		t.Fatalf("InferVars() error = %v", err)
	}

	want := []string{"title", "author.name", "note", "draft", "subtitle"}
	if got := varNames(vars); !reflect.DeepEqual(got, want) {
		t.Errorf("InferVars() = %v, want %v", got, want)
	}
}

func TestInferVars_SkipsContainers(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "doc.tmpl"), `
{{ .Values.title }}
{{ range .Values.items }}
- {{ .name }} {{ .price }}
{{ end }}
{{ range $key, $value := .Values.labels }}{{ $key }}={{ $value }}{{ end }}
{{ with .Values.tags }}{{ range . }}#{{ . }}{{ end }}{{ end }}
{{ .Values.items.first }}
`)

	tpl, err := Get("doc")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	vars, err := InferVars(tpl)
	if err != nil {
		t.Fatalf("InferVars() error = %v", err)
	}

	// A list or a map is not something to type at a prompt, and neither is
	// anything reached through one.
	want := []string{"title"}
	if got := varNames(vars); !reflect.DeepEqual(got, want) {
		t.Errorf("InferVars() = %v, want %v", got, want)
	}
}

func TestInferVars_FollowsValuesAlias(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "doc.tmpl"), `
{{- $v := .Values -}}
{{ $v.currency }} {{ $v.taxPercent }}
{{ range $v.items }}{{ .name }}{{ $.Values.suffix }}{{ end }}
`)

	tpl, err := Get("doc")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	vars, err := InferVars(tpl)
	if err != nil {
		t.Fatalf("InferVars() error = %v", err)
	}

	want := []string{"currency", "taxPercent", "suffix"}
	if got := varNames(vars); !reflect.DeepEqual(got, want) {
		t.Errorf("InferVars() = %v, want %v", got, want)
	}
}

func TestInferVars_Folder(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "app", "go.mod.tmpl"), "module {{ .Values.module }}\n")
	writeTemplate(t, filepath.Join(store, "app", "{{ .Values.folder }}", "main.go.tmpl"), "package {{ .Values.pkg }}\n")

	tpl, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	vars, err := InferVars(tpl)
	if err != nil {
		t.Fatalf("InferVars() error = %v", err)
	}

	// The path segments are templates too, so what they read is asked for.
	got := map[string]bool{}
	for _, name := range varNames(vars) {
		got[name] = true
	}
	for _, want := range []string{"module", "folder", "pkg"} {
		if !got[want] {
			t.Errorf("InferVars() = %v, missing %q", varNames(vars), want)
		}
	}
}

func TestLoadManifest(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "doc.tmpl"), "{{ .Values.title }}")
	writeTemplate(t, filepath.Join(store, "doc"+VarsFile), `
vars:
  - name: title
    prompt: What is it called
    required: true
  - name: status
    type: choice
    choices: [draft, final]
    default: draft
`)

	tpl, err := Get("doc")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	manifest, err := LoadManifest(tpl)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	if len(manifest.Vars) != 2 {
		t.Fatalf("LoadManifest() = %#v, want 2 variables", manifest.Vars)
	}
	if !manifest.Vars[0].Required || manifest.Vars[0].Prompt != "What is it called" {
		t.Errorf("first variable = %#v", manifest.Vars[0])
	}

	// A manifest replaces what would have been inferred.
	vars, err := Vars(tpl)
	if err != nil {
		t.Fatalf("Vars() error = %v", err)
	}
	if got := varNames(vars); !reflect.DeepEqual(got, []string{"title", "status"}) {
		t.Errorf("Vars() = %v", got)
	}
}

func TestLoadManifest_Errors(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "broken.tmpl"), "x")
	writeTemplate(t, filepath.Join(store, "broken"+VarsFile), "vars:\n  - prompt: no name here\n")

	tpl, err := Get("broken")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if _, err := LoadManifest(tpl); err == nil {
		t.Error("LoadManifest() error = nil, want an error on a nameless variable")
	}

	writeTemplate(t, filepath.Join(store, "broken"+VarsFile), "vars:\n  - name: pick\n    type: choice\n")
	if _, err := LoadManifest(tpl); err == nil {
		t.Error("LoadManifest() error = nil, want an error on a choice with no choices")
	}
}

func TestManifestIsNotATemplate(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "doc.tmpl"), "{{ .Values.title }}")
	writeTemplate(t, filepath.Join(store, "doc"+VarsFile), "vars: []\n")
	writeTemplate(t, filepath.Join(store, "app", "go.mod.tmpl"), "module x\n")
	writeTemplate(t, filepath.Join(store, "app", VarsFile), "vars: []\n")

	templates, err := List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got := len(templates); got != 2 {
		t.Fatalf("List() = %v, want only the two templates", templates)
	}

	// Nor is it rendered as part of a folder template.
	tpl, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := filepath.Join(t.TempDir(), "out")
	written, err := Render(RenderOptions{Template: tpl, Dest: dest, Name: "out"})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(written) != 1 || filepath.Base(written[0]) != "go.mod" {
		t.Errorf("Render() = %v, want only go.mod", written)
	}
}
