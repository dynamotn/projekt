package root

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/dynamo-tools/projekt/pkg/tplutil"
)

func TestTrustCmd_LetsAClonedTemplateRunItsCommands(t *testing.T) {
	store := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"remote", "add", "origin", "https://example.invalid/templates.git"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", store}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(store, "conf.tmpl"), []byte(`{{ output "echo" "hi" }}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("PROJEKT_TEMPLATE_DIR", store)
	previous := tplutil.AskTrust
	tplutil.AskTrust = func(tplutil.Origin, string) (bool, error) { return false, nil }
	t.Cleanup(func() { tplutil.AskTrust = previous })

	run := func(args ...string) (string, error) {
		var out bytes.Buffer
		cmd := NewRootCmd(&out)
		cmd.SetArgs(args)
		cmd.SetErr(&out)
		err := cmd.Execute()
		return out.String(), err
	}

	dest := filepath.Join(t.TempDir(), "conf.txt")
	if _, err := run("new", "conf", dest); err == nil || !strings.Contains(err.Error(), "t trust conf") {
		t.Fatalf("t new before trusting: error = %v, want a refusal naming `t trust conf`", err)
	}

	out, err := run("trust", "conf")
	if err != nil {
		t.Fatalf("t trust: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Trusted conf") {
		t.Errorf("t trust printed %q", out)
	}

	if out, err := run("new", "conf", dest); err != nil {
		t.Fatalf("t new after trusting: %v\n%s", err, out)
	}
}

// trustStore sets up a cloned store with one template and an empty trust file.
func trustStore(t *testing.T) (state string) {
	t.Helper()
	store := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"remote", "add", "origin", "https://example.invalid/templates.git"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", store}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(store, "conf.tmpl"), []byte(`{{ output "echo" "hi" }}`), 0o644); err != nil {
		t.Fatal(err)
	}
	state = t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	t.Setenv("PROJEKT_TEMPLATE_DIR", store)
	return state
}

func runTrust(args ...string) (string, error) {
	var out bytes.Buffer
	cmd := NewTemplateTrustCmd(&out)
	cmd.SetArgs(args)
	cmd.SetErr(&out)
	err := cmd.Execute()
	return out.String(), err
}

func TestTrustCmd_BareTrustsNothing(t *testing.T) {
	trustStore(t)

	out, err := runTrust()
	if err == nil || !strings.Contains(err.Error(), "--all") {
		t.Fatalf("t trust: error = %v, want one that mentions --all", err)
	}
	if strings.Contains(out, "Trusted") {
		t.Errorf("t trust trusted something: %q", out)
	}
	if _, err := os.Stat(tplutil.TrustFile()); !os.IsNotExist(err) {
		t.Errorf("t trust wrote the trust file: %v", err)
	}
}

func TestTrustCmd_AllTrustsEveryTemplate(t *testing.T) {
	trustStore(t)

	out, err := runTrust("--all")
	if err != nil {
		t.Fatalf("t trust --all: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Trusted conf") {
		t.Errorf("t trust --all printed %q", out)
	}
}

func TestTrustCmd_NamesWithAllIsAnError(t *testing.T) {
	trustStore(t)

	out, err := runTrust("conf", "--all")
	if err == nil || !strings.Contains(err.Error(), "drop the names") {
		t.Fatalf("t trust conf --all: want an error, got %q", out)
	}
	if strings.Contains(out, "Trusted") {
		t.Errorf("t trust conf --all trusted something: %q", out)
	}
}
