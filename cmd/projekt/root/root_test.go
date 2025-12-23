package root

import (
	"bytes"
	"testing"
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
