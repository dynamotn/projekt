package root

import (
	"bytes"
	"strings"
	"testing"
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

func TestNewRootCmd_NoExtraCommands(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewRootCmd(&buf)

	// Should only have version command and help
	commands := cmd.Commands()

	// Filter out help command (automatically added by cobra)
	actualCommands := []string{}
	for _, c := range commands {
		if c.Name() != "help" && c.Name() != "completion" {
			actualCommands = append(actualCommands, c.Name())
		}
	}

	if len(actualCommands) != 1 || actualCommands[0] != "version" {
		t.Errorf("NewRootCmd() should only have version command, got: %v", actualCommands)
	}
}
