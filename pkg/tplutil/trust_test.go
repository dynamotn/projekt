package tplutil

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain keeps the trust decisions of the tests out of the real state
// directory, in both directions.
func TestMain(m *testing.M) {
	state, err := os.MkdirTemp("", "projekt-state-")
	if err != nil {
		panic(err)
	}
	_ = os.Setenv("XDG_STATE_HOME", state)
	code := m.Run()
	_ = os.RemoveAll(state)
	os.Exit(code)
}

// useClonedStore makes the store a repository with a remote, the way
// `t init` leaves it, with a trust file of its own and nothing approved yet.
func useClonedStore(t *testing.T) string {
	t.Helper()
	store := useStore(t)
	for _, args := range [][]string{
		{"init", "-q"},
		{"remote", "add", "origin", "https://example.invalid/templates.git"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", store}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	resetApproved()
	t.Cleanup(resetApproved)
	return store
}

// answerTrust answers the trust question with yes or no, and counts how
// often it was asked.
func answerTrust(t *testing.T, yes bool) *int {
	t.Helper()
	asked := 0
	previous := AskTrust
	AskTrust = func(Origin, string) (bool, error) {
		asked++
		return yes, nil
	}
	t.Cleanup(func() { AskTrust = previous })
	return &asked
}

func renderConf(t *testing.T) error {
	t.Helper()
	tpl, err := Get("conf")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	_, err = Render(RenderOptions{Template: tpl, Dest: filepath.Join(t.TempDir(), "conf.txt")})
	return err
}

func TestTrust_AStoreOfYourOwnRunsCommands(t *testing.T) {
	store := useStore(t)
	resetApproved()
	asked := answerTrust(t, false)
	writeTemplate(t, filepath.Join(store, "conf.tmpl"), `{{ output "echo" "hi" }}`)

	if err := renderConf(t); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if *asked != 0 {
		t.Errorf("asked %d times about a store that is not a clone", *asked)
	}
}

func TestTrust_AClonedStoreIsRefusedUntilTrusted(t *testing.T) {
	store := useClonedStore(t)
	answerTrust(t, false)
	writeTemplate(t, filepath.Join(store, "conf.tmpl"), `{{ output "echo" "hi" }}`)

	err := renderConf(t)
	var untrusted *UntrustedError
	if !errors.As(err, &untrusted) {
		t.Fatalf("Render() error = %v, want an UntrustedError", err)
	}
	if !strings.Contains(err.Error(), "t trust conf") {
		t.Errorf("error = %v, want it to say how to trust the template", err)
	}
}

func TestTrust_YesIsRememberedAcrossRuns(t *testing.T) {
	store := useClonedStore(t)
	asked := answerTrust(t, true)
	writeTemplate(t, filepath.Join(store, "conf.tmpl"), `{{ output "echo" "hi" }}`)

	if err := renderConf(t); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	// The next run is another process: nothing approved in memory.
	resetApproved()
	if err := renderConf(t); err != nil {
		t.Fatalf("second Render() error = %v", err)
	}
	if *asked != 1 {
		t.Errorf("asked %d times, want once", *asked)
	}
}

func TestTrust_AChangeTakesTheTrustBack(t *testing.T) {
	for _, changed := range []string{"conf.tmpl", filepath.Join(PartialsDir, "header.tmpl")} {
		t.Run(changed, func(t *testing.T) {
			store := useClonedStore(t)
			answerTrust(t, false)
			writeTemplate(t, filepath.Join(store, "conf.tmpl"), `{{ output "echo" "hi" }}`)
			writeTemplate(t, filepath.Join(store, PartialsDir, "header.tmpl"), "# header\n")

			tpl, err := Get("conf")
			if err != nil {
				t.Fatal(err)
			}
			if err := Trust(TemplateOrigin(tpl)); err != nil {
				t.Fatalf("Trust() error = %v", err)
			}
			resetApproved()
			if err := renderConf(t); err != nil {
				t.Fatalf("Render() of the trusted version error = %v", err)
			}

			writeTemplate(t, filepath.Join(store, changed), `{{ output "echo" "changed" }}`)
			resetApproved()
			var untrusted *UntrustedError
			if err := renderConf(t); !errors.As(err, &untrusted) {
				t.Errorf("Render() after changing %s error = %v, want an UntrustedError", changed, err)
			}
		})
	}
}

func TestTrust_HooksAndDefaultsAreGuardedToo(t *testing.T) {
	store := useClonedStore(t)
	answerTrust(t, false)
	writeTemplate(t, filepath.Join(store, "app", ".vars.yaml"),
		"vars:\n  - name: who\n    default: '{{ output \"echo\" \"me\" }}'\nafter:\n  - echo ran > hook.txt\n")
	writeTemplate(t, filepath.Join(store, "app", "main.txt.tmpl"), "x\n")

	tpl, err := Get("app")
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "out")
	o := RenderOptions{Template: tpl, Dest: dest}
	if _, err := Render(o); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	var log bytes.Buffer
	var untrusted *UntrustedError
	if err := runHooks(t, tpl, o, &log); !errors.As(err, &untrusted) {
		t.Errorf("RunAfter() error = %v, want an UntrustedError", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "hook.txt")); !os.IsNotExist(err) {
		t.Errorf("the hook ran anyway")
	}

	vars, err := Vars(tpl)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Prompter{In: strings.NewReader(""), Out: &log, Origin: TemplateOrigin(tpl)}.Ask(vars, Values{}, map[string]any{})
	if !errors.As(err, &untrusted) {
		t.Errorf("Ask() error = %v, want the default's command refused", err)
	}
}

func TestTrust_ARepositoryWithoutARemoteIsYours(t *testing.T) {
	store := useStore(t)
	if out, err := exec.Command("git", "-C", store, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	resetApproved()
	answerTrust(t, false)
	writeTemplate(t, filepath.Join(store, "conf.tmpl"), `{{ output "echo" "hi" }}`)

	if err := renderConf(t); err != nil {
		t.Errorf("Render() error = %v, want a local repository trusted", err)
	}
}

func TestAuthorize_RefusesWhatComesFromNowhere(t *testing.T) {
	resetApproved()
	if err := Authorize(Origin{}, "echo hi"); err == nil {
		t.Error("Authorize() with no origin = nil, want a refusal")
	}
}

func TestTrust_AFileTemplatesManifestAndDataAreTrustedWithIt(t *testing.T) {
	for _, changed := range []string{"conf.vars.yaml", "conf.data.yaml"} {
		t.Run(changed, func(t *testing.T) {
			store := useClonedStore(t)
			answerTrust(t, false)
			writeTemplate(t, filepath.Join(store, "conf.tmpl"), `{{ output "echo" "hi" }}`)
			writeTemplate(t, filepath.Join(store, "conf.vars.yaml"), "vars: []\n")
			writeTemplate(t, filepath.Join(store, "conf.data.yaml"), "who: me\n")

			tpl, err := Get("conf")
			if err != nil {
				t.Fatal(err)
			}
			if err := Trust(TemplateOrigin(tpl)); err != nil {
				t.Fatalf("Trust() error = %v", err)
			}

			// A sync brings a hook, or new data, in beside the template.
			writeTemplate(t, filepath.Join(store, changed), "after:\n  - echo pwned\n")
			resetApproved()
			var untrusted *UntrustedError
			if err := renderConf(t); !errors.As(err, &untrusted) {
				t.Errorf("Render() after changing %s error = %v, want an UntrustedError", changed, err)
			}
		})
	}
}

func TestTrust_ARepositoryGitCannotReadIsNotYours(t *testing.T) {
	store := useStore(t)
	// A worktree whose main checkout is gone: `git remote` fails, and that
	// says nothing about where the files came from.
	writeTemplate(t, filepath.Join(store, ".git"), "gitdir: /nonexistent/.git/worktrees/x\n")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	resetApproved()
	t.Cleanup(resetApproved)
	answerTrust(t, false)
	writeTemplate(t, filepath.Join(store, "conf.tmpl"), `{{ output "echo" "hi" }}`)

	var untrusted *UntrustedError
	if err := renderConf(t); !errors.As(err, &untrusted) {
		t.Errorf("Render() error = %v, want an UntrustedError", err)
	}
}

func TestTrust_WithoutAStateDirectoryNothingIsTrusted(t *testing.T) {
	store := useClonedStore(t)
	answerTrust(t, false)
	writeTemplate(t, filepath.Join(store, "conf.tmpl"), `{{ output "echo" "hi" }}`)
	tpl, err := Get("conf")
	if err != nil {
		t.Fatal(err)
	}
	sum, err := TemplateOrigin(tpl).fingerprint()
	if err != nil {
		t.Fatal(err)
	}

	// No home to keep state in: a relative trust file would be the current
	// folder's, which a repository can ship pre-trusting itself.
	work := t.TempDir()
	t.Chdir(work)
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "")
	writeTemplate(t, filepath.Join(work, "projekt", "trust.yaml"),
		"- key: "+tpl.Path+"\n  sha256: "+sum+"\n  trustedAt: 2026-01-01T00:00:00Z\n")

	var untrusted *UntrustedError
	if err := renderConf(t); !errors.As(err, &untrusted) {
		t.Errorf("Render() error = %v, want an UntrustedError", err)
	}
	if err := Trust(TemplateOrigin(tpl)); err == nil {
		t.Error("Trust() error = nil, want a refusal to record trust nowhere")
	}
}

func TestTrust_AnInheritedGitDirDoesNotVouchForAStore(t *testing.T) {
	store := useClonedStore(t)
	answerTrust(t, false)
	writeTemplate(t, filepath.Join(store, "conf.tmpl"), `{{ output "echo" "hi" }}`)

	// Run from a git hook of a repository of your own, with no remote.
	local := t.TempDir()
	if out, err := exec.Command("git", "-C", local, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	t.Setenv("GIT_DIR", filepath.Join(local, ".git"))

	if err := renderConf(t); err == nil {
		t.Error("Render() error = nil, want the cloned store still asked about")
	}
}
