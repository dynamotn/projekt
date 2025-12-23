package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewVersionCmd(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewVersionCmd(&buf)

	if cmd == nil {
		t.Fatal("NewVersionCmd() returned nil")
	}

	if cmd.Use != "version" {
		t.Errorf("NewVersionCmd() Use = %v, want version", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("NewVersionCmd() Short description is empty")
	}

	// Check flags exist
	shortFlag := cmd.Flags().Lookup("short")
	if shortFlag == nil {
		t.Error("NewVersionCmd() missing --short flag")
	}

	templateFlag := cmd.Flags().Lookup("template")
	if templateFlag == nil {
		t.Error("NewVersionCmd() missing --template flag")
	}
}

func TestVersionCmd_Run(t *testing.T) {
	tests := []struct {
		name     string
		short    bool
		template string
		wantErr  bool
		contains string
	}{
		{
			name:     "default version output",
			short:    false,
			template: "",
			wantErr:  false,
			contains: "Projekt CLI",
		},
		{
			name:     "short version output",
			short:    true,
			template: "",
			wantErr:  false,
			contains: "v", // Short version always has 'v'
		},
		{
			name:     "custom template",
			template: "Version: {{.Version}}",
			wantErr:  false,
			contains: "Version:",
		},
		{
			name:     "invalid template",
			template: "{{.InvalidField",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := NewVersionCmd(&buf)

			if tt.short {
				cmd.Flags().Set("short", "true")
			}

			if tt.template != "" {
				cmd.Flags().Set("template", tt.template)
			}

			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("cmd.Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				output := buf.String()
				if !strings.Contains(output, tt.contains) {
					t.Errorf("output does not contain expected string %q, got: %s", tt.contains, output)
				}
			}
		})
	}
}

func TestFormatVersion(t *testing.T) {
	tests := []struct {
		name        string
		short       bool
		shouldCheck bool
	}{
		{
			name:        "short format",
			short:       true,
			shouldCheck: true,
		},
		{
			name:        "long format",
			short:       false,
			shouldCheck: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatVersion(tt.short)

			if result == "" {
				t.Error("formatVersion() returned empty string")
			}

			if tt.short {
				// Short version could be either "Projekt CLI vX.Y.Z+gitXXXXXXX" or just "vX.Y.Z"
				// depending on whether GitCommit is available
				if !strings.Contains(result, "Projekt CLI") && !strings.Contains(result, "v") {
					t.Errorf("formatVersion(true) expected version format, got: %s", result)
				}
			} else {
				// Long format should have "Projekt CLI" and more details
				if !strings.Contains(result, "Projekt CLI") {
					t.Errorf("formatVersion(false) expected long format with 'Projekt CLI', got: %s", result)
				}
			}
		})
	}
}

func TestFormatVersion_WithGitCommit(t *testing.T) {
	// This tests the path where GitCommit has a value
	// Since we can't easily modify version.GetBuildInfo(), we just verify
	// the logic for short format with various GitCommit lengths
	result := formatVersion(true)
	if result == "" {
		t.Error("formatVersion(true) should not return empty string")
	}

	// The result should either be "Projekt CLI X+gitY" or "vX.Y.Z"
	hasVersion := strings.Contains(result, "v") || strings.Contains(result, "Projekt CLI")
	if !hasVersion {
		t.Errorf("formatVersion(true) should contain version info, got: %s", result)
	}
}

func TestVersionCmd_NoArgs(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewVersionCmd(&buf)

	// Version command should not accept arguments
	cmd.SetArgs([]string{"extra", "args"})
	err := cmd.Execute()

	if err == nil {
		t.Error("Expected error when passing arguments to version command, got nil")
	}
}

func TestVersionOptions_Run(t *testing.T) {
	tests := []struct {
		name     string
		opts     versionOptions
		wantErr  bool
		contains string
	}{
		{
			name:     "default options",
			opts:     versionOptions{},
			wantErr:  false,
			contains: "Projekt CLI",
		},
		{
			name:     "short option",
			opts:     versionOptions{short: true},
			wantErr:  false,
			contains: "v", // Version string always starts with 'v'
		},
		{
			name:    "valid template",
			opts:    versionOptions{template: "{{.Version}}"},
			wantErr: false,
		},
		{
			name:    "invalid template",
			opts:    versionOptions{template: "{{.Invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := tt.opts.run(&buf)

			if (err != nil) != tt.wantErr {
				t.Errorf("run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.contains != "" {
				output := buf.String()
				if !strings.Contains(output, tt.contains) {
					t.Errorf("output does not contain expected string %q, got: %s", tt.contains, output)
				}
			}
		})
	}
}
