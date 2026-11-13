package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
	"gitlab.com/dynamo-tools/projekt/pkg/tplutil"
)

// includingConfig points the configuration at a file that includes one more,
// and returns the included file.
func includingConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	included := filepath.Join(dir, "team.yaml")
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(included, []byte("folders: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("include:\n  - "+included+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	// viper and the cached config are process-wide: put them back afterwards.
	lazypath.CfgFile = cfgPath
	viper.Reset()
	t.Cleanup(func() {
		lazypath.CfgFile = ""
		viper.Reset()
		lazypath.ResetTestConfig()
	})
	lazypath.InitConfig()
	lazypath.ResetTestConfig()
	return included
}

func runTrust(args ...string) (string, error) {
	var out bytes.Buffer
	cmd := NewConfigTrustCmd(&out)
	cmd.SetArgs(args)
	cmd.SetErr(&out)
	err := cmd.Execute()
	return out.String(), err
}

func TestConfigTrust_BareTrustsNothing(t *testing.T) {
	includingConfig(t)

	out, err := runTrust()
	if err == nil || !strings.Contains(err.Error(), "--all") {
		t.Fatalf("config trust: error = %v, want one that mentions --all", err)
	}
	if strings.Contains(out, "Trusted") {
		t.Errorf("config trust trusted something: %q", out)
	}
	if _, err := os.Stat(tplutil.TrustFile()); !os.IsNotExist(err) {
		t.Errorf("config trust wrote the trust file: %v", err)
	}
}

func TestConfigTrust_AllTrustsEveryIncludedFile(t *testing.T) {
	included := includingConfig(t)

	out, err := runTrust("--all")
	if err != nil {
		t.Fatalf("config trust --all: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Trusted "+included) {
		t.Errorf("config trust --all printed %q", out)
	}
}

func TestConfigTrust_NamesWithAllIsAnError(t *testing.T) {
	included := includingConfig(t)

	out, err := runTrust(included, "--all")
	if err == nil || !strings.Contains(err.Error(), "drop the names") {
		t.Fatalf("config trust <file> --all: want an error, got %q", out)
	}
	if strings.Contains(out, "Trusted") {
		t.Errorf("config trust <file> --all trusted something: %q", out)
	}
}
