package folder

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewFolderCmd(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewFolderCmd(&buf)

	if cmd == nil {
		t.Fatal("NewFolderCmd() returned nil")
	}

	if cmd.Use != "folder" {
		t.Errorf("NewFolderCmd() Use = %v, want folder", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("NewFolderCmd() Short description is empty")
	}

	// Check aliases
	expectedAliases := []string{"f", "fd", "fol"}
	if len(cmd.Aliases) != len(expectedAliases) {
		t.Errorf("NewFolderCmd() has %d aliases, want %d", len(cmd.Aliases), len(expectedAliases))
	}
}

func TestNewFolderCmd_Subcommands(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewFolderCmd(&buf)

	expectedCommands := []string{
		"add",
		"list",
		"get",
		"remove",
		"sync",
		"check",
	}

	commands := cmd.Commands()
	commandNames := make(map[string]bool)
	for _, c := range commands {
		commandNames[c.Name()] = true
	}

	for _, expected := range expectedCommands {
		if !commandNames[expected] {
			t.Errorf("NewFolderCmd() missing subcommand: %s", expected)
		}
	}
}

func TestNewFolderCmd_Help(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewFolderCmd(&buf)
	cmd.SetArgs([]string{"--help"})
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err != nil {
		t.Errorf("NewFolderCmd().Execute() with --help error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "folder") {
		t.Error("Help output does not contain command name")
	}
}

func TestNewFolderAddCmd(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewFolderAddCmd(&buf)

	if cmd == nil {
		t.Fatal("NewFolderAddCmd() returned nil")
	}

	if cmd.Use != "add [folder path]" {
		t.Errorf("NewFolderAddCmd() Use = %v, want 'add [folder path]'", cmd.Use)
	}

	// Check aliases
	if len(cmd.Aliases) == 0 || cmd.Aliases[0] != "a" {
		t.Error("NewFolderAddCmd() missing alias 'a'")
	}

	// Check flags
	prefixFlag := cmd.Flags().Lookup("prefix")
	if prefixFlag == nil {
		t.Error("NewFolderAddCmd() missing --prefix flag")
	}

	workspaceFlag := cmd.Flags().Lookup("as-workspace")
	if workspaceFlag == nil {
		t.Error("NewFolderAddCmd() missing --as-workspace flag")
	}

	regexFlag := cmd.Flags().Lookup("regex")
	if regexFlag == nil {
		t.Error("NewFolderAddCmd() missing --regex flag")
	}

	priorityFlag := cmd.Flags().Lookup("priority")
	if priorityFlag == nil {
		t.Error("NewFolderAddCmd() missing --priority flag")
	}
}

func TestNewFolderAddCmd_Args(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "no args",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "too many args",
			args:    []string{"/tmp/folder1", "/tmp/folder2"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := NewFolderAddCmd(&buf)
			cmd.SetArgs(tt.args)
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("NewFolderAddCmd().Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewFolderListCmd(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewFolderListCmd(&buf)

	if cmd == nil {
		t.Fatal("NewFolderListCmd() returned nil")
	}

	if cmd.Use != "list" {
		t.Errorf("NewFolderListCmd() Use = %v, want list", cmd.Use)
	}

	// Check aliases
	if len(cmd.Aliases) == 0 || cmd.Aliases[0] != "l" {
		t.Error("NewFolderListCmd() missing alias 'l'")
	}

	// Check flags
	plainFlag := cmd.Flags().Lookup("plain")
	if plainFlag == nil {
		t.Error("NewFolderListCmd() missing --plain flag")
	}

	shortOnlyFlag := cmd.Flags().Lookup("short-only")
	if shortOnlyFlag == nil {
		t.Error("NewFolderListCmd() missing --short-only flag")
	}

	noHeadersFlag := cmd.Flags().Lookup("no-headers")
	if noHeadersFlag == nil {
		t.Error("NewFolderListCmd() missing --no-headers flag")
	}

	noColorFlag := cmd.Flags().Lookup("no-color")
	if noColorFlag == nil {
		t.Error("NewFolderListCmd() missing --no-color flag")
	}
}

func TestNewFolderListCmd_NoArgs(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewFolderListCmd(&buf)
	cmd.SetArgs([]string{})
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Should not error with no arguments
	err := cmd.Execute()
	if err != nil {
		t.Errorf("NewFolderListCmd().Execute() with no args error = %v", err)
	}
}

func TestNewFolderListCmd_WithArgs(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewFolderListCmd(&buf)
	cmd.SetArgs([]string{"unexpected"})
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Should error with arguments
	err := cmd.Execute()
	if err == nil {
		t.Error("NewFolderListCmd().Execute() with args should error")
	}
}

func TestNewFolderGetCmd(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewFolderGetCmd(&buf)

	if cmd == nil {
		t.Fatal("NewFolderGetCmd() returned nil")
	}

	if cmd.Use != "get [short name]" {
		t.Errorf("NewFolderGetCmd() Use = %v, want 'get [short name]'", cmd.Use)
	}

	// Check aliases
	if len(cmd.Aliases) == 0 || cmd.Aliases[0] != "g" {
		t.Error("NewFolderGetCmd() missing alias 'g'")
	}

	// Check if ValidArgsFunction is set
	if cmd.ValidArgsFunction == nil {
		t.Error("NewFolderGetCmd() missing ValidArgsFunction")
	}
}

func TestNewFolderGetCmd_Args(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "no args",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "too many args",
			args:    []string{"name1", "name2"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := NewFolderGetCmd(&buf)
			cmd.SetArgs(tt.args)
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("NewFolderGetCmd().Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewFolderRemoveCmd(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewFolderRemoveCmd(&buf)

	if cmd == nil {
		t.Fatal("NewFolderRemoveCmd() returned nil")
	}

	if !strings.Contains(cmd.Use, "remove") {
		t.Errorf("NewFolderRemoveCmd() Use = %v, should contain 'remove'", cmd.Use)
	}

	// Check aliases
	aliasesFound := false
	for _, alias := range cmd.Aliases {
		if alias == "rm" || alias == "r" {
			aliasesFound = true
			break
		}
	}
	if !aliasesFound {
		t.Error("NewFolderRemoveCmd() missing expected aliases")
	}
}

func TestNewFolderSyncCmd(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewFolderSyncCmd(&buf)

	if cmd == nil {
		t.Fatal("NewFolderSyncCmd() returned nil")
	}

	if cmd.Use != "sync" {
		t.Errorf("NewFolderSyncCmd() Use = %v, want sync", cmd.Use)
	}

	// Check dry-run flag
	dryRunFlag := cmd.Flags().Lookup("dry-run")
	if dryRunFlag == nil {
		t.Error("NewFolderSyncCmd() missing --dry-run flag")
	}
}

func TestNewFolderCheckCmd(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewFolderCheckCmd(&buf)

	if cmd == nil {
		t.Fatal("NewFolderCheckCmd() returned nil")
	}

	if cmd.Use != "check" {
		t.Errorf("NewFolderCheckCmd() Use = %v, want check", cmd.Use)
	}
}

func TestNewFolderCmd_AllSubcommandsHaveHelp(t *testing.T) {
	var buf bytes.Buffer
	parentCmd := NewFolderCmd(&buf)

	for _, subCmd := range parentCmd.Commands() {
		t.Run(subCmd.Name()+"_help", func(t *testing.T) {
			// Create a fresh buffer for each subcommand
			var subBuf bytes.Buffer

			// Create a fresh parent command to avoid state issues
			freshParent := NewFolderCmd(&subBuf)
			freshParent.SetOut(&subBuf)
			freshParent.SetErr(&subBuf)
			freshParent.SetArgs([]string{subCmd.Name(), "--help"})

			err := freshParent.Execute()
			// Help returns nil error after showing help
			if err != nil {
				t.Errorf("%s --help error = %v", subCmd.Name(), err)
			}

			output := subBuf.String()
			if len(output) == 0 {
				t.Errorf("%s help output is empty", subCmd.Name())
			}
		})
	}
}
