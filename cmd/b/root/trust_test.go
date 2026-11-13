package root

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"gitlab.com/dynamo-tools/projekt/pkg/tplutil"
)

func runTrust(args ...string) (string, error) {
	var out bytes.Buffer
	cmd := NewBoilerplateTrustCmd(&out)
	cmd.SetArgs(args)
	cmd.SetErr(&out)
	err := cmd.Execute()
	return out.String(), err
}

func TestTrustCmd_BareTrustsNothing(t *testing.T) {
	stores(t, "source:\n  template: app\n")
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	out, err := runTrust()
	if err == nil || !strings.Contains(err.Error(), "--all") {
		t.Fatalf("b trust: error = %v, want one that mentions --all", err)
	}
	if strings.Contains(out, "Trusted") {
		t.Errorf("b trust trusted something: %q", out)
	}
	if _, err := os.Stat(tplutil.TrustFile()); !os.IsNotExist(err) {
		t.Errorf("b trust wrote the trust file: %v", err)
	}
}

func TestTrustCmd_AllTrustsEveryRecipe(t *testing.T) {
	stores(t, "source:\n  template: app\n")
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	out, err := runTrust("--all")
	if err != nil {
		t.Fatalf("b trust --all: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Trusted app") {
		t.Errorf("b trust --all printed %q", out)
	}
}

func TestTrustCmd_NamesWithAllIsAnError(t *testing.T) {
	stores(t, "source:\n  template: app\n")
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	out, err := runTrust("app", "--all")
	if err == nil || !strings.Contains(err.Error(), "drop the names") {
		t.Fatalf("b trust app --all: want an error, got %q", out)
	}
	if strings.Contains(out, "Trusted") {
		t.Errorf("b trust app --all trusted something: %q", out)
	}
}
