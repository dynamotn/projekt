package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestSetColorAndStyles(t *testing.T) {
	tests := []struct {
		name    string
		cmdName string
	}{
		{
			name:    "basic command",
			cmdName: "test",
		},
		{
			name:    "command with subcommands",
			cmdName: "parent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{
				Use:   tt.cmdName,
				Short: "Test command",
			}

			// Should not panic
			SetColorAndStyles(cmd)

			// Verify template was modified
			template := cmd.UsageTemplate()
			if template == "" {
				t.Error("SetColorAndStyles() did not set usage template")
			}

			// Check if StyleHeading was applied
			if !contains(template, "StyleHeading") {
				t.Error("SetColorAndStyles() did not apply StyleHeading template function")
			}
		})
	}
}

func TestSetColorAndStyles_TemplateReplacements(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	originalTemplate := cmd.UsageTemplate()
	SetColorAndStyles(cmd)
	newTemplate := cmd.UsageTemplate()

	// Template should be different after applying styles
	if originalTemplate == newTemplate {
		t.Error("SetColorAndStyles() did not modify the usage template")
	}

	// Check for expected replacements
	expectedStrings := []string{
		"StyleHeading",
		"USAGE:",
		"FLAGS:",
	}

	for _, expected := range expectedStrings {
		if !contains(newTemplate, expected) {
			t.Errorf("SetColorAndStyles() template does not contain expected string: %s", expected)
		}
	}
}

func TestSetColorAndStyles_RegexReplacement(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
		Long: `This is a long description
with multiple lines
Flags:
  --flag1  Description 1
  --flag2  Description 2
`,
	}

	SetColorAndStyles(cmd)
	template := cmd.UsageTemplate()

	// The regex should replace "Flags:" at the beginning of a line
	if !contains(template, "StyleHeading") {
		t.Error("SetColorAndStyles() did not apply regex replacement for Flags:")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
