package config

import (
	"bytes"
	"strings"
	"testing"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

func TestRunConfigCheck(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name string
		// config is installed before the check runs.
		config lazypath.Config
		strict bool
		// wantErr says whether the command should fail, which is what a CI job
		// keys off.
		wantErr bool
		// wantOutput are substrings the report must contain.
		wantOutput []string
	}{
		{
			name:       "a clean config passes",
			config:     lazypath.Config{Folders: []lazypath.Folder{{Path: tmpDir}}},
			wantErr:    false,
			wantOutput: []string{"0 error(s), 0 warning(s)"},
		},
		{
			name: "an error fails the check",
			config: lazypath.Config{
				Folders: []lazypath.Folder{{Path: tmpDir, IsWorkspace: true, RegexMatch: "([a-z"}},
			},
			wantErr:    true,
			wantOutput: []string{"[ERROR]", "invalid regex"},
		},
		{
			name: "a warning alone passes",
			config: lazypath.Config{
				Folders: []lazypath.Folder{{Path: tmpDir + "/missing"}},
			},
			wantErr:    false,
			wantOutput: []string{"[WARNING]", "0 error(s), 1 warning(s)"},
		},
		{
			name: "a warning fails under --strict",
			config: lazypath.Config{
				Folders: []lazypath.Folder{{Path: tmpDir + "/missing"}},
			},
			strict:     true,
			wantErr:    true,
			wantOutput: []string{"[WARNING]"},
		},
		{
			name: "every problem is reported, not just the first",
			config: lazypath.Config{
				Folders: []lazypath.Folder{
					{Path: tmpDir, IsWorkspace: true, RegexMatch: "([a-z"},
					{Path: "", Prefix: "broken"},
				},
			},
			wantErr:    true,
			wantOutput: []string{"invalid regex", "empty path", "2 error(s)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lazypath.SetTestConfig(tt.config)
			t.Cleanup(lazypath.ResetTestConfig)

			var buf bytes.Buffer
			err := runConfigCheck(&buf, tt.strict)

			if (err != nil) != tt.wantErr {
				t.Errorf("runConfigCheck() error = %v, wantErr %v", err, tt.wantErr)
			}

			output := buf.String()
			for _, want := range tt.wantOutput {
				if !strings.Contains(output, want) {
					t.Errorf("report does not contain %q:\n%s", want, output)
				}
			}
		})
	}
}

func TestNewConfigCheckCmd_RunsOnAnUnreadableConfig(t *testing.T) {
	// The root command refuses to run on a config it could not read. Reporting
	// that is this command's whole purpose, so it must override that guard.
	cmd := NewConfigCheckCmd(&bytes.Buffer{})

	if cmd.PersistentPreRunE == nil {
		t.Fatal("config check does not override the root PersistentPreRunE")
	}
	if err := cmd.PersistentPreRunE(cmd, nil); err != nil {
		t.Errorf("config check PersistentPreRunE = %v, want nil", err)
	}
}
