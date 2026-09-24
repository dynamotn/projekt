package tplutil

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRender_PromptFunctionsAskOnce(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "app", "a.txt.tmpl"), "{{ promptString \"owner\" \"nobody\" }}\n")
	writeTemplate(t, filepath.Join(store, "app", "b.txt.tmpl"), "{{ promptString \"owner\" \"nobody\" }}\n")

	tpl, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := t.TempDir()
	var questions bytes.Buffer

	if _, err := Render(RenderOptions{
		Template:    tpl,
		Dest:        dest,
		Name:        "app",
		In:          strings.NewReader("Jane\n"),
		Prompt:      &questions,
		Interactive: true,
	}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	for _, name := range []string{"a.txt", "b.txt"} {
		if got := strings.TrimSpace(readFile(t, filepath.Join(dest, name))); got != "Jane" {
			t.Errorf("%s = %q, want the single answer", name, got)
		}
	}
	if asked := strings.Count(questions.String(), "owner"); asked != 1 {
		t.Errorf("asked %d times, want the same question asked once", asked)
	}
}

func TestRender_PromptTakesItsDefaultWithoutInteractive(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "conf.tmpl"), "{{ promptInt \"port\" 8080 }} {{ promptBool \"ci\" true }}\n")

	tpl, err := Get("conf")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := filepath.Join(t.TempDir(), "conf.txt")
	if _, err := Render(RenderOptions{Template: tpl, Dest: dest}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if got := strings.TrimSpace(readFile(t, dest)); got != "8080 true" {
		t.Errorf("rendered = %q, want the defaults", got)
	}
}

func TestRender_PromptWithoutDefaultNeedsInteractive(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "conf.tmpl"), "{{ promptString \"owner\" }}\n")

	tpl, err := Get("conf")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	_, err = Render(RenderOptions{Template: tpl, Dest: filepath.Join(t.TempDir(), "conf.txt")})
	if err == nil || !strings.Contains(err.Error(), "--interactive") {
		t.Errorf("Render() error = %v, want it to say how to answer", err)
	}
}

func TestRender_PromptChoiceAsksAgainOnAWrongAnswer(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "conf.tmpl"),
		"{{ promptChoice \"licence\" (list \"MIT\" \"BSD\") }}\n")

	tpl, err := Get("conf")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := filepath.Join(t.TempDir(), "conf.txt")
	var questions bytes.Buffer
	if _, err := Render(RenderOptions{
		Template:    tpl,
		Dest:        dest,
		In:          strings.NewReader("GPL\nBSD\n"),
		Prompt:      &questions,
		Interactive: true,
	}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if got := strings.TrimSpace(readFile(t, dest)); got != "BSD" {
		t.Errorf("rendered = %q, want the second answer", got)
	}
	if !strings.Contains(questions.String(), "not one of") {
		t.Errorf("questions = %q, want the wrong answer refused", questions.String())
	}
}

func TestFuncs_YamlAndStat(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "conf.tmpl"),
		"{{ (fromYaml \"a: 1\").a }} {{ toYaml (dict \"b\" 2) }} {{ if stat .Store }}here{{ end }} {{ joinPath \"a\" \"b\" }}\n")

	tpl, err := Get("conf")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := filepath.Join(t.TempDir(), "conf.txt")
	if _, err := Render(RenderOptions{Template: tpl, Dest: dest}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if got := strings.TrimSpace(readFile(t, dest)); got != "1 b: 2 here a/b" {
		t.Errorf("rendered = %q", got)
	}
}

func TestFuncs_OutputAndLookPath(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "conf.tmpl"),
		"{{ output \"echo\" \"hi\" | trim }} {{ if lookPath \"sh\" }}sh{{ end }} {{ if lookPath \"definitely-not-installed\" }}bad{{ end }}\n")

	tpl, err := Get("conf")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := filepath.Join(t.TempDir(), "conf.txt")
	if _, err := Render(RenderOptions{Template: tpl, Dest: dest}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if got := strings.TrimSpace(readFile(t, dest)); got != "hi sh" {
		t.Errorf("rendered = %q, want the command output and the executable found", got)
	}
}
