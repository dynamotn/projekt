package root

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

func TestNewProjektTemplateCmd(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewProjektTemplateCmd(&buf)

	if cmd == nil {
		t.Fatal("NewProjektTemplateCmd() returned nil")
	}

	if cmd.Use != "template" {
		t.Errorf("NewProjektTemplateCmd() Use = %v, want template", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("NewProjektTemplateCmd() Short description is empty")
	}

	// Check aliases
	expectedAliases := []string{"t", "tpl"}
	if len(cmd.Aliases) != len(expectedAliases) {
		t.Errorf("NewProjektTemplateCmd() has %d aliases, want %d", len(cmd.Aliases), len(expectedAliases))
	}
}

func TestNewRootCmd(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewRootCmd(&buf)

	if cmd == nil {
		t.Fatal("NewRootCmd() returned nil")
	}

	if cmd.Use != "t" {
		t.Errorf("NewRootCmd() Use = %v, want t", cmd.Use)
	}

	if !cmd.SilenceUsage {
		t.Error("NewRootCmd() SilenceUsage should be true")
	}

	// Check persistent flags
	verboseFlag := cmd.PersistentFlags().Lookup("verbose")
	if verboseFlag == nil {
		t.Error("NewRootCmd() missing --verbose flag")
	}
}

func TestNewRootCmd_Subcommands(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewRootCmd(&buf)

	commands := cmd.Commands()
	foundVersion := false
	for _, c := range commands {
		if c.Name() == "version" {
			foundVersion = true
			break
		}
	}

	if !foundVersion {
		t.Error("NewRootCmd() missing version subcommand")
	}
}

func TestNewRootCmd_Help(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewRootCmd(&buf)
	cmd.SetArgs([]string{"--help"})
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err != nil {
		t.Errorf("NewRootCmd().Execute() with --help error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "template") {
		t.Error("Help output does not contain expected text")
	}
}

func TestNewRootCmd_Version(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewRootCmd(&buf)
	cmd.SetArgs([]string{"version"})
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err != nil {
		t.Errorf("NewRootCmd().Execute() with version error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Projekt CLI") {
		t.Error("Version output does not contain expected text")
	}
}

func TestNewRootCmd_VerboseFlag(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewRootCmd(&buf)
	cmd.SetArgs([]string{"--verbose", "debug", "version"})
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err != nil {
		t.Errorf("NewRootCmd().Execute() with --verbose error = %v", err)
	}
}

func TestNewRootCmd_Aliases(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewRootCmd(&buf)

	// Root command should have no aliases for standalone 't' command
	if len(cmd.Aliases) != 0 {
		t.Errorf("NewRootCmd() should have no aliases, got %d", len(cmd.Aliases))
	}
}

func TestNewProjektTemplateCmd_Aliases(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewProjektTemplateCmd(&buf)

	aliasesMap := make(map[string]bool)
	for _, alias := range cmd.Aliases {
		aliasesMap[alias] = true
	}

	if !aliasesMap["t"] {
		t.Error("NewProjektTemplateCmd() missing alias 't'")
	}

	if !aliasesMap["tpl"] {
		t.Error("NewProjektTemplateCmd() missing alias 'tpl'")
	}
}

func TestNewRootCmd_TemplateCommands(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewRootCmd(&buf)

	// Filter out the commands cobra adds on its own
	actualCommands := map[string]bool{}
	for _, c := range cmd.Commands() {
		if c.Name() != "help" && c.Name() != "completion" {
			actualCommands[c.Name()] = true
		}
	}

	for _, name := range []string{"version", "list", "new", "add", "show", "path"} {
		if !actualCommands[name] {
			t.Errorf("NewRootCmd() missing %q subcommand, got: %v", name, actualCommands)
		}
	}
}

func TestNewRootCmd_TemplateDirFlag(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewRootCmd(&buf)

	if cmd.PersistentFlags().Lookup("template-dir") == nil {
		t.Error("NewRootCmd() missing --template-dir flag")
	}
}

func TestNewRootCmd_UpdateMeansApply(t *testing.T) {
	cmd := NewRootCmd(&bytes.Buffer{})

	// Two commands claiming one alias leave it to cobra to pick, and it picked
	// sync while apply's help promised update.
	found, _, err := cmd.Find([]string{"update"})
	if err != nil {
		t.Fatalf("Find(update) error = %v", err)
	}
	if found.Name() != "apply" {
		t.Errorf("t update runs %q, want apply", found.Name())
	}
}

func runT(t *testing.T, args ...string) error {
	t.Helper()
	// An earlier test that rendered a template read the configuration, which
	// is then cached; the command under test must load its own.
	lazypath.ResetTestConfig()
	t.Cleanup(func() {
		lazypath.CfgFile = ""
		lazypath.ResetTestConfig()
		viper.Reset()
	})
	var out bytes.Buffer
	cmd := NewRootCmd(&out)
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	return cmd.Execute()
}

func TestRootCmd_ConfigFlagIsRead(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.yaml")
	conf := "gitServers:\n  - name: gl\n    type: gitlab\n    https: https://gitlab.example.com\n"
	if err := os.WriteFile(file, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := runT(t, "--config", file, "--template-dir", t.TempDir(), "list"); err != nil {
		t.Fatalf("t list: %v", err)
	}

	servers := lazypath.GetConfig().GitServers
	if len(servers) != 1 || servers[0].Name != "gl" {
		t.Errorf("GitServers = %v, want the gl entry", servers)
	}
}

func TestRootCmd_BrokenConfigDoesNotStopT(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(file, []byte("folders: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := runT(t, "--config", file, "--template-dir", t.TempDir(), "list"); err != nil {
		t.Errorf("t list with a broken config: %v", err)
	}
}

// suggestFixture is a project that says "billing" and a fake AI that names it.
func suggestFixture(t *testing.T) (src, store string) {
	t.Helper()
	src = filepath.Join(t.TempDir(), "billing")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "README.md"), []byte("# billing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ai := filepath.Join(t.TempDir(), "ai.sh")
	if err := os.WriteFile(ai, []byte("#!/bin/sh\ncat >/dev/null\necho 'billing: name'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PROJEKT_AI", ai)
	return src, t.TempDir()
}

func TestAddCmd_SuggestWritesNothingWithoutYes(t *testing.T) {
	src, store := suggestFixture(t)
	if err := runT(t, "--template-dir", store, "add", src, "--suggest"); err != nil {
		t.Fatalf("t add --suggest: %v", err)
	}
	if entries, _ := os.ReadDir(store); len(entries) != 0 {
		t.Errorf("the store holds %v, want nothing", entries)
	}
}

func TestAddCmd_SuggestWithYesAdds(t *testing.T) {
	src, store := suggestFixture(t)
	if err := runT(t, "--template-dir", store, "add", src, "--suggest", "--yes"); err != nil {
		t.Fatalf("t add --suggest --yes: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(store, "billing", "README.md"))
	if err != nil || !strings.Contains(string(got), "{{ .Values.name }}") {
		t.Errorf("README = %q, %v, want {{ .Values.name }}", got, err)
	}
}

func TestAddCmd_ShowPromptNeedsSuggest(t *testing.T) {
	src, store := suggestFixture(t)
	if err := runT(t, "--template-dir", store, "add", src, "--show-prompt"); err == nil {
		t.Error("--show-prompt without --suggest = nil, want an error")
	}
}
