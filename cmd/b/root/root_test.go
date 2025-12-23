package root

import (
	"bytes"
	"strings"
	"testing"
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
