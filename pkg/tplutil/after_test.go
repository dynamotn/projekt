package tplutil

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runHooks renders a template and then runs whatever its manifest asks for,
// the way `t new` does.
func runHooks(t *testing.T, tpl Template, o RenderOptions, log *bytes.Buffer) error {
	t.Helper()
	dir, err := Destination(o)
	if err != nil {
		t.Fatalf("Destination() error = %v", err)
	}
	base, err := BaseContext(o)
	if err != nil {
		t.Fatalf("BaseContext() error = %v", err)
	}
	return RunAfter(HookOptions{
		Template: tpl,
		Dir:      dir,
		Context:  base,
		DryRun:   o.DryRun,
		Log:      log,
		Out:      log,
		Err:      log,
	})
}

func TestRunAfter_RunsInTheFolderItWrote(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "app", ".vars.yaml"),
		"after:\n  - 'echo {{ .Name }} > hook.txt'\n  - \"{{ if .Values.tidy }}echo tidied >> hook.txt{{ end }}\"\n")
	writeTemplate(t, filepath.Join(store, "app", "main.txt.tmpl"), "x\n")

	tpl, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := t.TempDir()
	o := RenderOptions{Template: tpl, Dest: dest, Name: "myapp", Values: Values{"tidy": true}}
	if _, err := Render(o); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	var log bytes.Buffer
	if err := runHooks(t, tpl, o, &log); err != nil {
		t.Fatalf("RunAfter() error = %v", err)
	}

	if got := readFile(t, filepath.Join(dest, "hook.txt")); got != "myapp\ntidied\n" {
		t.Errorf("hook.txt = %q, want both commands run in the destination", got)
	}
	if !strings.Contains(log.String(), "Running: echo myapp") {
		t.Errorf("log = %q, want each command printed before it runs", log.String())
	}
}

func TestRunAfter_SkipsACommandThatRendersAway(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "app", ".vars.yaml"),
		"after:\n  - \"{{ if .Values.ci }}echo never > hook.txt{{ end }}\"\n")
	writeTemplate(t, filepath.Join(store, "app", "main.txt.tmpl"), "x\n")

	tpl, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := t.TempDir()
	o := RenderOptions{Template: tpl, Dest: dest, Name: "myapp"}
	if _, err := Render(o); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if err := runHooks(t, tpl, o, &bytes.Buffer{}); err != nil {
		t.Fatalf("RunAfter() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(dest, "hook.txt")); !os.IsNotExist(err) {
		t.Errorf("Stat(hook.txt) error = %v, want the command skipped", err)
	}
}

func TestRunAfter_DryRunOnlySaysWhatItWouldDo(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "app", ".vars.yaml"), "after:\n  - 'echo hi > hook.txt'\n")
	writeTemplate(t, filepath.Join(store, "app", "main.txt.tmpl"), "x\n")

	tpl, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := t.TempDir()
	var log bytes.Buffer
	o := RenderOptions{Template: tpl, Dest: dest, Name: "myapp", DryRun: true, Out: io.Discard}
	if err := runHooks(t, tpl, o, &log); err != nil {
		t.Fatalf("RunAfter() error = %v", err)
	}

	if !strings.Contains(log.String(), "[DRY RUN] Would run: echo hi") {
		t.Errorf("log = %q, want the command described and not run", log.String())
	}
	if _, err := os.Stat(filepath.Join(dest, "hook.txt")); !os.IsNotExist(err) {
		t.Errorf("a dry run ran the command")
	}
}

func TestRunAfter_AFailureStopsTheRest(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "app", ".vars.yaml"),
		"after:\n  - 'exit 3'\n  - 'echo second > hook.txt'\n")
	writeTemplate(t, filepath.Join(store, "app", "main.txt.tmpl"), "x\n")

	tpl, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := t.TempDir()
	o := RenderOptions{Template: tpl, Dest: dest, Name: "myapp"}
	if _, err := Render(o); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if err := runHooks(t, tpl, o, &bytes.Buffer{}); err == nil {
		t.Fatal("RunAfter() error = nil, want the failure reported")
	}
	if _, err := os.Stat(filepath.Join(dest, "hook.txt")); !os.IsNotExist(err) {
		t.Errorf("the second command ran after the first failed")
	}
}

func TestRunAfter_NothingToRunIsNotAnError(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "plain.tmpl"), "x\n")

	tpl, err := Get("plain")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := filepath.Join(t.TempDir(), "plain.txt")
	if err := runHooks(t, tpl, RenderOptions{Template: tpl, Dest: dest}, &bytes.Buffer{}); err != nil {
		t.Errorf("RunAfter() error = %v, want a template without commands to be fine", err)
	}
}
