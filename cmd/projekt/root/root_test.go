package root

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

func TestNewRootCmd(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewRootCmd(&buf)

	if cmd == nil {
		t.Fatal("NewRootCmd() returned nil")
	}

	if cmd.Use != "projekt" {
		t.Errorf("NewRootCmd() Use = %v, want projekt", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("NewRootCmd() Short description is empty")
	}

	// Check persistent flags
	configFlag := cmd.PersistentFlags().Lookup("config")
	if configFlag == nil {
		t.Error("NewRootCmd() missing --config flag")
	}

	verboseFlag := cmd.PersistentFlags().Lookup("verbose")
	if verboseFlag == nil {
		t.Error("NewRootCmd() missing --verbose flag")
	}
}

func TestNewRootCmd_Subcommands(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewRootCmd(&buf)

	expectedCommands := []string{
		"init",
		"folder",
		"template",
		"boilerplate",
		"version",
	}

	commands := cmd.Commands()
	commandNames := make(map[string]bool)
	for _, c := range commands {
		commandNames[c.Name()] = true
	}

	for _, expected := range expectedCommands {
		if !commandNames[expected] {
			t.Errorf("NewRootCmd() missing subcommand: %s", expected)
		}
	}
}

func TestNewRootCmd_Help(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewRootCmd(&buf)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("NewRootCmd().Execute() with --help error = %v", err)
	}

	output := buf.String()
	// Help command should produce output
	if len(output) == 0 {
		t.Error("Help output is empty")
	}
}

func TestNewRootCmd_InvalidFlag(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewRootCmd(&buf)
	cmd.SetArgs([]string{"--invalid-flag"})
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for invalid flag")
	}
}

func TestNewRootCmd_ConfigFlag(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewRootCmd(&buf)
	cmd.SetArgs([]string{"--config", "/tmp/test-config.yaml", "version"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("NewRootCmd().Execute() with --config error = %v", err)
	}
}

func TestNewRootCmd_VerboseFlag(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewRootCmd(&buf)
	cmd.SetArgs([]string{"--verbose", "debug", "version"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("NewRootCmd().Execute() with --verbose error = %v", err)
	}
}

// brokenConfig installs a config file lazypath cannot use.
func brokenConfig(t *testing.T, content string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	lazypath.CfgFile = path
	viper.Reset()
	t.Cleanup(func() {
		lazypath.CfgFile = ""
		viper.Reset()
		lazypath.ResetTestConfig()
	})
	lazypath.ResetTestConfig()
	lazypath.InitConfig()
}

func TestNewRootCmd_ConfigGate(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		args    []string
		wantErr bool
	}{
		// None of these needs a folder, so a broken config must not stop them.
		{"version", "folders: [unclosed\n", []string{"version"}, false},
		{"completion", "folders: [unclosed\n", []string{"completion", "bash"}, false},
		{"help", "folders: [unclosed\n", []string{"help"}, false},
		{"template", "folders: [unclosed\n", []string{"template", "list"}, false},
		{"unreadable", "folders: [unclosed\n", []string{"folder", "list"}, true},
		// YAML that does not decode is only found out when the config is
		// first used, which has to be before the gate rather than after it.
		{"undecodable", "folders: [1]\n", []string{"folder", "list"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			brokenConfig(t, tt.config)
			cmd := NewRootCmd(&bytes.Buffer{})
			// Cobra adds these on Execute; Find needs them now.
			cmd.InitDefaultHelpCmd()
			cmd.InitDefaultCompletionCmd()

			found, _, err := cmd.Find(tt.args)
			if err != nil {
				t.Fatalf("Find(%v) error = %v", tt.args, err)
			}
			err = cmd.PersistentPreRunE(found, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("gate on %v = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
		})
	}
}
