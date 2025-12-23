package lazypath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	tmpDir := t.TempDir()
	existingPath := filepath.Join(tmpDir, "exists")
	os.Mkdir(existingPath, 0o755)

	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config with existing paths",
			config: Config{
				Folders: []Folder{
					{Path: existingPath},
				},
			},
			wantErr: false,
		},
		{
			name: "config with empty path",
			config: Config{
				Folders: []Folder{
					{Path: ""},
				},
			},
			wantErr: true,
		},
		{
			name: "config with non-existent path",
			config: Config{
				Folders: []Folder{
					{Path: "/this/path/does/not/exist"},
				},
			},
			wantErr: false, // Just logs debug, doesn't error
		},
		{
			name: "empty config",
			config: Config{
				Folders: []Folder{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
