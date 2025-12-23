package templates

import (
	"bytes"
	"strings"
	"testing"
)

func TestGenCommands(t *testing.T) {
	tests := []struct {
		name    string
		shell   string
		wantErr bool
		check   func(string) bool
	}{
		{
			name:    "bash shell",
			shell:   "bash",
			wantErr: false,
			check: func(output string) bool {
				return len(output) > 0
			},
		},
		{
			name:    "fish shell",
			shell:   "fish",
			wantErr: false,
			check: func(output string) bool {
				return len(output) > 0
			},
		},
		{
			name:    "invalid shell",
			shell:   "invalid-shell",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := GenCommands(tt.shell, &buf)

			if (err != nil) != tt.wantErr {
				t.Errorf("GenCommands() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				output := buf.String()
				if tt.check != nil && !tt.check(output) {
					t.Errorf("GenCommands() output check failed for shell %s", tt.shell)
				}
			}
		})
	}
}

func TestGenCommands_TemplateExecution(t *testing.T) {
	shells := []string{"bash", "fish"}

	for _, shell := range shells {
		t.Run("test_"+shell, func(t *testing.T) {
			var buf bytes.Buffer
			err := GenCommands(shell, &buf)
			if err != nil {
				t.Errorf("GenCommands(%s) error = %v", shell, err)
				return
			}

			output := buf.String()
			if output == "" {
				t.Errorf("GenCommands(%s) produced empty output", shell)
			}

			// Check that template was executed (output should not contain template syntax)
			if strings.Contains(output, "{{") || strings.Contains(output, "}}") {
				t.Errorf("GenCommands(%s) output contains unprocessed template syntax", shell)
			}
		})
	}
}

func TestGenCommands_EmptyShell(t *testing.T) {
	var buf bytes.Buffer
	err := GenCommands("", &buf)

	if err == nil {
		t.Error("GenCommands() with empty shell should return error")
	}
}

func TestGenCommands_OutputWriter(t *testing.T) {
	// Test that output is written to the provided writer
	var buf bytes.Buffer
	err := GenCommands("bash", &buf)
	if err != nil {
		t.Errorf("GenCommands() error = %v", err)
		return
	}

	if buf.Len() == 0 {
		t.Error("GenCommands() did not write to output writer")
	}
}
