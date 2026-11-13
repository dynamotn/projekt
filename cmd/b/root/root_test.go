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

func TestNewProjektBoilerplateCmd(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewProjektBoilerplateCmd(&buf)

	if cmd == nil {
		t.Fatal("NewProjektBoilerplateCmd() returned nil")
	}

	if cmd.Use != "boilerplate" {
		t.Errorf("NewProjektBoilerplateCmd() Use = %v, want boilerplate", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("NewProjektBoilerplateCmd() Short description is empty")
	}

	// Check aliases
	expectedAliases := []string{"b", "bpl"}
	if len(cmd.Aliases) != len(expectedAliases) {
		t.Errorf("NewProjektBoilerplateCmd() has %d aliases, want %d", len(cmd.Aliases), len(expectedAliases))
	}
}

func TestNewRootCmd(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewRootCmd(&buf)

	if cmd == nil {
		t.Fatal("NewRootCmd() returned nil")
	}

	if cmd.Use != "b" {
		t.Errorf("NewRootCmd() Use = %v, want b", cmd.Use)
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
	if !strings.Contains(output, "boilerplate") {
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

	// Root command should have no aliases for standalone 'b' command
	if len(cmd.Aliases) != 0 {
		t.Errorf("NewRootCmd() should have no aliases, got %d", len(cmd.Aliases))
	}
}

func TestNewRootCmd_BoilerplateCommands(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewRootCmd(&buf)

	// Filter out the commands cobra adds on its own
	actualCommands := map[string]bool{}
	for _, c := range cmd.Commands() {
		if c.Name() != "help" && c.Name() != "completion" {
			actualCommands[c.Name()] = true
		}
	}

	for _, name := range []string{"version", "list", "new", "show", "path"} {
		if !actualCommands[name] {
			t.Errorf("NewRootCmd() missing %q subcommand, got: %v", name, actualCommands)
		}
	}
}

func TestNewRootCmd_Flags(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewRootCmd(&buf)

	for _, name := range []string{"boilerplate-dir", "template-dir", "config"} {
		if cmd.PersistentFlags().Lookup(name) == nil {
			t.Errorf("NewRootCmd() missing --%s flag", name)
		}
	}
}

func TestNewRootCmd_ConfigGate(t *testing.T) {
	const unreadable, undecodable = "folders: [unclosed\n", "folders: [1]\n"
	tests := []struct {
		args    string
		config  string
		wantErr bool
	}{
		// None of these needs the configuration.
		{"version", unreadable, false},
		{"completion bash", unreadable, false},
		{"help", unreadable, false},
		{"new", unreadable, true},
		// YAML that does not decode only shows once the configuration is
		// used: the gate has to use it first.
		{"new", undecodable, true},
	}
	for _, tt := range tests {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(tt.config), 0o644); err != nil {
			t.Fatal(err)
		}
		lazypath.CfgFile = path
		viper.Reset()
		lazypath.ResetTestConfig()
		lazypath.InitConfig()
		t.Cleanup(func() {
			lazypath.CfgFile = ""
			viper.Reset()
			lazypath.ResetTestConfig()
		})

		cmd := NewRootCmd(&bytes.Buffer{})
		// Cobra adds these on Execute; Find needs them now.
		cmd.InitDefaultHelpCmd()
		cmd.InitDefaultCompletionCmd()

		found, _, err := cmd.Find(strings.Fields(tt.args))
		if err != nil {
			t.Fatalf("Find(%s) error = %v", tt.args, err)
		}
		if err := cmd.PersistentPreRunE(found, nil); (err != nil) != tt.wantErr {
			t.Errorf("gate on %s with %q = %v, wantErr %v", tt.args, tt.config, err, tt.wantErr)
		}
	}
}
