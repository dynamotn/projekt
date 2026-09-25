package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/dynamo-tools/projekt/pkg/bplutil"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
	"gitlab.com/dynamo-tools/projekt/pkg/tplutil"
)

func TestRun_CoversEverything(t *testing.T) {
	checks := Run()
	if len(checks) < 5 {
		t.Fatalf("Run() = %d checks, want the whole list", len(checks))
	}

	seen := map[string]bool{}
	for _, check := range checks {
		if check.Name == "" || check.Detail == "" {
			t.Errorf("check = %#v, want a name and something to read", check)
		}
		switch check.Status {
		case StatusOK, StatusWarn, StatusFail:
		default:
			t.Errorf("check %q has status %q", check.Name, check.Status)
		}
		if seen[check.Name] {
			t.Errorf("check %q appears twice", check.Name)
		}
		seen[check.Name] = true
	}

	for _, want := range []string{"git", "binaries", "shell integration", "configuration", "templates"} {
		if !seen[want] {
			t.Errorf("Run() does not check %q", want)
		}
	}
}

func TestCheckBinaries(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)

	// Nothing on PATH: a warning, and it names what to do about it.
	check := checkBinaries()
	if check.Status != StatusWarn {
		t.Errorf("checkBinaries() = %#v, want a warning", check)
	}
	if !strings.Contains(check.Detail, "projekt") || !strings.Contains(check.Detail, "make install") {
		t.Errorf("detail = %q, want the missing names and the way out", check.Detail)
	}

	for _, name := range []string{"projekt", "t", "b"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if check := checkBinaries(); check.Status != StatusOK {
		t.Errorf("checkBinaries() = %#v, want ok once they are on PATH", check)
	}
}

func TestCheckShell(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	check := checkShell()
	if check.Status != StatusWarn {
		t.Errorf("checkShell() = %#v, want a warning when nothing sources it", check)
	}

	if err := os.WriteFile(filepath.Join(home, ".zshrc"),
		[]byte("# my shell\neval \"$(projekt init zsh)\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	check = checkShell()
	if check.Status != StatusOK {
		t.Fatalf("checkShell() = %#v, want ok", check)
	}
	if !strings.Contains(check.Detail, "zsh") {
		t.Errorf("detail = %q, want the shell it found", check.Detail)
	}

	// fish keeps its configuration somewhere else, which the check knows.
	home2 := t.TempDir()
	t.Setenv("HOME", home2)
	fishConfig := filepath.Join(home2, ".config", "fish", "config.fish")
	if err := os.MkdirAll(filepath.Dir(fishConfig), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fishConfig, []byte("projekt init fish | source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if check := checkShell(); check.Status != StatusOK || !strings.Contains(check.Detail, "fish") {
		t.Errorf("checkShell() = %#v, want fish found", check)
	}
}

func TestCheckConfig(t *testing.T) {
	lazypath.SetTestConfig(lazypath.Config{})
	t.Cleanup(lazypath.ResetTestConfig)

	// Nothing configured is not broken, but it is worth saying.
	check := checkConfig()
	if check.Status != StatusWarn || !strings.Contains(check.Detail, "folder add") {
		t.Errorf("checkConfig() = %#v, want a warning that points somewhere", check)
	}

	lazypath.ResetTestConfig()
	here := t.TempDir()
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{Path: here}}})
	if check := checkConfig(); check.Status != StatusOK {
		t.Errorf("checkConfig() = %#v, want ok", check)
	}
}

func TestCheckFolders(t *testing.T) {
	root := t.TempDir()
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{Path: root}}})
	t.Cleanup(lazypath.ResetTestConfig)

	if check := checkFolders(); check.Status != StatusOK {
		t.Errorf("checkFolders() = %#v, want ok", check)
	}

	lazypath.ResetTestConfig()
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{Path: filepath.Join(root, "gone")}}})
	check := checkFolders()
	if check.Status != StatusWarn || !strings.Contains(check.Detail, "gone") {
		t.Errorf("checkFolders() = %#v, want it to name what is missing", check)
	}
}

func TestCheckTemplates(t *testing.T) {
	dir := t.TempDir()
	previous := tplutil.TemplateDir
	tplutil.TemplateDir = dir
	t.Setenv("PROJEKT_TEMPLATE_DIR", "")
	t.Cleanup(func() { tplutil.TemplateDir = previous })

	if check := checkTemplates(); check.Status != StatusWarn {
		t.Errorf("checkTemplates() = %#v, want a warning for an empty store", check)
	}

	if err := os.WriteFile(filepath.Join(dir, "note.tmpl"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if check := checkTemplates(); check.Status != StatusOK {
		t.Errorf("checkTemplates() = %#v, want ok", check)
	}
}

func TestCheckBoilerplates(t *testing.T) {
	templates := t.TempDir()
	recipes := t.TempDir()

	previousTemplates, previousRecipes := tplutil.TemplateDir, bplutil.RecipeDir
	tplutil.TemplateDir, bplutil.RecipeDir = templates, recipes
	t.Setenv("PROJEKT_TEMPLATE_DIR", "")
	t.Setenv("PROJEKT_BOILERPLATE_DIR", "")
	t.Cleanup(func() { tplutil.TemplateDir, bplutil.RecipeDir = previousTemplates, previousRecipes })

	if check := checkBoilerplates(); check.Status != StatusWarn {
		t.Errorf("checkBoilerplates() = %#v, want a warning for an empty store", check)
	}

	// A recipe whose template was renamed only fails when it is used, which is
	// the wrong moment to find out.
	if err := os.WriteFile(filepath.Join(recipes, "app.yaml"),
		[]byte("source:\n  template: not-there\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	check := checkBoilerplates()
	if check.Status != StatusFail || !strings.Contains(check.Detail, "app") {
		t.Errorf("checkBoilerplates() = %#v, want a failure naming the recipe", check)
	}

	if err := os.WriteFile(filepath.Join(templates, "not-there.tmpl"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if check := checkBoilerplates(); check.Status != StatusOK {
		t.Errorf("checkBoilerplates() = %#v, want ok once the template is there", check)
	}
}

func TestWorst(t *testing.T) {
	cases := []struct {
		checks []Check
		want   Status
	}{
		{nil, StatusOK},
		{[]Check{{Status: StatusOK}}, StatusOK},
		{[]Check{{Status: StatusOK}, {Status: StatusWarn}}, StatusWarn},
		{[]Check{{Status: StatusWarn}, {Status: StatusFail}}, StatusFail},
		{[]Check{{Status: StatusFail}, {Status: StatusOK}}, StatusFail},
	}
	for _, tt := range cases {
		if got := Worst(tt.checks); got != tt.want {
			t.Errorf("Worst(%v) = %q, want %q", tt.checks, got, tt.want)
		}
	}
}
