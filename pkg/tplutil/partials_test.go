package tplutil

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRender_SharedPartialFromTheStore(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, PartialsDir, "header.tmpl"), "# (c) {{ .Values.author }}")
	writeTemplate(t, filepath.Join(store, "script.tmpl"), "{{ template \"header\" . }}\necho hi\n")

	tpl, err := Get("script")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := filepath.Join(t.TempDir(), "run.sh")
	if _, err := Render(RenderOptions{Template: tpl, Dest: dest, Values: Values{"author": "Jane"}}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if got := readFile(t, dest); !strings.Contains(got, "# (c) Jane") {
		t.Errorf("rendered = %q, want the shared header", got)
	}
}

func TestRender_TemplatePartialShadowsTheStore(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, PartialsDir, "header.tmpl"), "store")
	writeTemplate(t, filepath.Join(store, "app", PartialsDir, "header.tmpl"), "own")
	writeTemplate(t, filepath.Join(store, "app", "main.txt.tmpl"), "{{ template \"header\" . }}\n")

	tpl, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := t.TempDir()
	if _, err := Render(RenderOptions{Template: tpl, Dest: dest, Name: "app"}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if got := strings.TrimSpace(readFile(t, filepath.Join(dest, "main.txt"))); got != "own" {
		t.Errorf("rendered = %q, want the template's own partial", got)
	}
	if _, err := Get(PartialsDir); err == nil {
		t.Errorf("Get(%s) succeeded, the shared folder is not a template", PartialsDir)
	}
}

func TestRender_IncludeTemplateIsPipeable(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, PartialsDir, "note.tmpl"), "a\nb")
	writeTemplate(t, filepath.Join(store, "doc.tmpl"), "{{ includeTemplate \"note\" . | indent 2 }}\n")

	tpl, err := Get("doc")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := filepath.Join(t.TempDir(), "doc.md")
	if _, err := Render(RenderOptions{Template: tpl, Dest: dest}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if got := readFile(t, dest); got != "  a\n  b\n" {
		t.Errorf("rendered = %q, want the partial indented", got)
	}
}

func TestRender_IncludeReadsAFileOfTheTemplate(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "app", "notice.txt"), "(c) {{ .Values.author }}\n")
	writeTemplate(t, filepath.Join(store, "app", "NOTICE.tmpl"), "{{ include \"notice.txt\" }}")

	tpl, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := t.TempDir()
	if _, err := Render(RenderOptions{Template: tpl, Dest: dest, Name: "app", Values: Values{"author": "Jane"}}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// include hands the file over as it is; the same file rendered on its own
	// went through the engine like everything else.
	if got := readFile(t, filepath.Join(dest, "NOTICE")); got != "(c) {{ .Values.author }}\n" {
		t.Errorf("included = %q, want the file unrendered", got)
	}
	if got := readFile(t, filepath.Join(dest, "notice.txt")); got != "(c) Jane\n" {
		t.Errorf("rendered = %q, want the file rendered", got)
	}
}

func TestRender_IncludeRefusesToLeaveTheTemplate(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "secret.txt"), "no\n")
	writeTemplate(t, filepath.Join(store, "app", "OUT.tmpl"), "{{ include \"../secret.txt\" }}")

	tpl, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	_, err = Render(RenderOptions{Template: tpl, Dest: t.TempDir(), Name: "app"})
	if err == nil || !strings.Contains(err.Error(), "outside the template") {
		t.Errorf("Render() error = %v, want a refusal to read outside the template", err)
	}
}

func TestRender_MachineContext(t *testing.T) {
	store := useStore(t)
	t.Setenv("PROJEKT_TEST_TOKEN", "seen")
	writeTemplate(t, filepath.Join(store, "info.tmpl"), "{{ .OS }} {{ .Env.PROJEKT_TEST_TOKEN }} {{ if .Store }}store{{ end }}\n")

	tpl, err := Get("info")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := filepath.Join(t.TempDir(), "info.txt")
	if _, err := Render(RenderOptions{Template: tpl, Dest: dest}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	got := strings.TrimSpace(readFile(t, dest))
	if !strings.Contains(got, "seen") || !strings.HasSuffix(got, "store") {
		t.Errorf("rendered = %q, want the OS, the environment and the store path", got)
	}
}
