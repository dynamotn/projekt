package templates

import (
	"bytes"
	"sort"
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
			name:    "zsh shell",
			shell:   "zsh",
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
	shells := Shells()

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

func TestShells(t *testing.T) {
	shells := Shells()

	want := map[string]bool{"bash": false, "fish": false, "zsh": false}
	for _, shell := range shells {
		if _, known := want[shell]; !known {
			t.Errorf("Shells() = %v, which has an unexpected %q", shells, shell)
			continue
		}
		want[shell] = true
	}
	for shell, found := range want {
		if !found {
			t.Errorf("Shells() = %v, missing %q", shells, shell)
		}
	}

	// Sorted, because it is what the command offers for completion.
	if !sort.StringsAreSorted(shells) {
		t.Errorf("Shells() = %v, want it sorted", shells)
	}
}

func TestShells_EveryOneOfThemGenerates(t *testing.T) {
	for _, shell := range Shells() {
		t.Run(shell, func(t *testing.T) {
			var buf bytes.Buffer
			if err := GenCommands(shell, &buf); err != nil {
				t.Fatalf("GenCommands(%s) error = %v", shell, err)
			}
			// Every script defines the jump function; that is the whole point
			// of the integration.
			if !strings.Contains(buf.String(), "pj") {
				t.Errorf("GenCommands(%s) does not define pj: %q", shell, buf.String())
			}
		})
	}
}

// TestShells_CompletionIsReadWhenCompleting guards the bug where fish listed
// the projects once, at startup.
func TestShells_CompletionIsReadWhenCompleting(t *testing.T) {
	for _, shell := range Shells() {
		t.Run(shell, func(t *testing.T) {
			var buf bytes.Buffer
			if err := GenCommands(shell, &buf); err != nil {
				t.Fatalf("GenCommands(%s) error = %v", shell, err)
			}
			script := buf.String()

			// Whatever the shell, the candidates have to come from a `folder
			// list` that runs at completion time. A script that has no such
			// call cannot be offering anything but a stale list.
			if !strings.Contains(script, "folder list") {
				t.Errorf("%s never lists the folders, so completion cannot be current", shell)
			}
		})
	}
}

// TestFishCompletionIsDynamic pins the fix: fish used to register one
// completion per project when the script was sourced, and refresh them from a
// wrapper around `projekt`. Anything else that changed the configuration —
// `b new`, `projekt worktree add`, an editor — left the list stale until the
// next `projekt` command.
func TestFishCompletionIsDynamic(t *testing.T) {
	var buf bytes.Buffer
	if err := GenCommands("fish", &buf); err != nil {
		t.Fatalf("GenCommands(fish) error = %v", err)
	}
	script := buf.String()

	// The candidates come from a command substitution, which fish runs when
	// the completion is asked for.
	if !strings.Contains(script, "-a '(__pj_projects)'") {
		t.Error("fish completion is not a command substitution, so it is computed once")
	}
	// And nothing wraps `projekt` to refresh a cache that no longer exists.
	if strings.Contains(script, "function projekt") {
		t.Error("fish still wraps projekt, which only existed to refresh the stale completion")
	}
}
