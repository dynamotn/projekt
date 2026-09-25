package root

import (
	"bytes"
	"strings"
	"testing"

	"gitlab.com/dynamo-tools/projekt/pkg/templates"
)

func TestNewInitCmd(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewInitCmd(&buf)

	if cmd == nil {
		t.Fatal("NewInitCmd() returned nil")
	}

	if cmd.Use != "init [shell]" {
		t.Errorf("NewInitCmd() Use = %v, want init [shell]", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("NewInitCmd() Short description is empty")
	}
}

func TestNewInitCmd_ValidArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "bash shell",
			args:    []string{"bash"},
			wantErr: false,
		},
		{
			name:    "fish shell",
			args:    []string{"fish"},
			wantErr: false,
		},
		{
			name:    "zsh shell",
			args:    []string{"zsh"},
			wantErr: false,
		},
		{
			name:    "invalid shell",
			args:    []string{"invalid"},
			wantErr: true,
		},
		{
			name:    "no args",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "too many args",
			args:    []string{"bash", "extra"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := NewInitCmd(&buf)
			cmd.SetArgs(tt.args)
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("NewInitCmd().Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewInitCmd_BashOutput(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewInitCmd(&buf)
	cmd.SetArgs([]string{"bash"})
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err != nil {
		t.Errorf("NewInitCmd().Execute() error = %v", err)
	}

	if !strings.Contains(buf.String(), "pj") {
		t.Errorf("NewInitCmd() did not write the bash script to the provided writer, got: %q", buf.String())
	}
}

func TestNewInitCmd_FishOutput(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewInitCmd(&buf)
	cmd.SetArgs([]string{"fish"})
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err != nil {
		t.Errorf("NewInitCmd().Execute() error = %v", err)
	}

	if !strings.Contains(buf.String(), "pj") {
		t.Errorf("NewInitCmd() did not write the fish script to the provided writer, got: %q", buf.String())
	}
}

func TestNewTemplateCmd(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewTemplateCmd(&buf)

	if cmd == nil {
		t.Fatal("NewTemplateCmd() returned nil")
	}

	// Should delegate to template root command
	if cmd.Use == "" {
		t.Error("NewTemplateCmd() has empty Use")
	}
}

func TestNewTemplateCmd_Help(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewTemplateCmd(&buf)
	cmd.SetArgs([]string{"--help"})
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err != nil {
		t.Errorf("NewTemplateCmd().Execute() with --help error = %v", err)
	}

	output := buf.String()
	if len(output) == 0 {
		t.Error("NewTemplateCmd() help output is empty")
	}
}

func TestNewBoilerplateCmd(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewBoilerplateCmd(&buf)

	if cmd == nil {
		t.Fatal("NewBoilerplateCmd() returned nil")
	}

	// Should delegate to boilerplate root command
	if cmd.Use == "" {
		t.Error("NewBoilerplateCmd() has empty Use")
	}
}

func TestNewBoilerplateCmd_Help(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewBoilerplateCmd(&buf)
	cmd.SetArgs([]string{"--help"})
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err != nil {
		t.Errorf("NewBoilerplateCmd().Execute() with --help error = %v", err)
	}

	output := buf.String()
	if len(output) == 0 {
		t.Error("NewBoilerplateCmd() help output is empty")
	}
}

func TestNewInitCmd_ValidArgsCheck(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewInitCmd(&buf)

	// What the command accepts is what a script is shipped for, so the two
	// cannot drift apart.
	want := templates.Shells()
	if len(cmd.ValidArgs) != len(want) {
		t.Errorf("NewInitCmd() ValidArgs = %v, want %v", cmd.ValidArgs, want)
	}

	validArgs := make(map[string]bool)
	for _, arg := range cmd.ValidArgs {
		validArgs[arg] = true
	}

	for _, shell := range append([]string{"bash", "fish", "zsh"}, want...) {
		if !validArgs[shell] {
			t.Errorf("NewInitCmd() ValidArgs missing %q", shell)
		}
	}
}

func TestNewInitCmd_EveryShellPrintsItsScript(t *testing.T) {
	for _, shell := range templates.Shells() {
		t.Run(shell, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := NewInitCmd(&buf)
			cmd.SetArgs([]string{shell})
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute(%s) error = %v", shell, err)
			}
			if !strings.Contains(buf.String(), "pj") {
				t.Errorf("init %s did not print the script, got: %q", shell, buf.String())
			}
		})
	}
}

func TestNewInitCmd_Help(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewInitCmd(&buf)
	cmd.SetArgs([]string{"--help"})
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err != nil {
		t.Errorf("NewInitCmd().Execute() with --help error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "init") {
		t.Error("Help output does not contain command name")
	}
}
