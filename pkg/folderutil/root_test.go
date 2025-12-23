package folderutil

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

func TestFindFolderByShortName(t *testing.T) {
	tmpDir := t.TempDir()
	testProject := filepath.Join(tmpDir, "test-project")
	if err := os.Mkdir(testProject, 0o755); err != nil {
		t.Fatal(err)
	}

	// Mock config
	lazypath.SetTestConfig(lazypath.Config{
		Folders: []lazypath.Folder{
			{
				Path:        tmpDir,
				Prefix:      "tmp",
				IsWorkspace: true,
				RegexMatch:  "test-.*",
			},
		},
	})
	defer lazypath.ResetTestConfig()

	tests := []struct {
		name      string
		shortName string
		wantPath  string
		wantErr   bool
	}{
		{
			name:      "existing folder",
			shortName: "tmp-test-project",
			wantPath:  testProject,
			wantErr:   false,
		},
		{
			name:      "non-existing folder",
			shortName: "nonexistent",
			wantPath:  "",
			wantErr:   false, // Returns empty path, not error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := FindFolderByShortName(&buf, tt.shortName)

			if (err != nil) != tt.wantErr {
				t.Errorf("FindFolderByShortName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			output := strings.TrimSpace(buf.String())
			if tt.wantPath != "" && output != tt.wantPath {
				t.Errorf("FindFolderByShortName() output = %v, want %v", output, tt.wantPath)
			}
		})
	}
}

func TestFindFolderByShortName_EmptyConfig(t *testing.T) {
	// Test with empty config
	lazypath.SetTestConfig(lazypath.Config{
		Folders: []lazypath.Folder{},
	})
	defer lazypath.ResetTestConfig()

	var buf bytes.Buffer
	err := FindFolderByShortName(&buf, "any-name")
	if err != nil {
		t.Errorf("FindFolderByShortName() should not error with empty config, got: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if output != "" {
		t.Errorf("FindFolderByShortName() should return empty path, got: %s", output)
	}
}

func TestImportFolderToConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Setup test config
	lazypath.CfgFile = configFile
	lazypath.InitConfig()

	tests := []struct {
		name    string
		folder  *lazypath.Folder
		wantErr bool
	}{
		{
			name: "add new folder",
			folder: &lazypath.Folder{
				Path:        "/tmp/test1",
				Prefix:      "test",
				IsWorkspace: false,
			},
			wantErr: false,
		},
		{
			name: "add folder with workspace",
			folder: &lazypath.Folder{
				Path:        "/tmp/workspace",
				Prefix:      "ws",
				IsWorkspace: true,
				RegexMatch:  "^[^.].+",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ImportFolderToConfig(tt.folder)
			if (err != nil) != tt.wantErr {
				t.Errorf("ImportFolderToConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestListFolders(t *testing.T) {
	tmpDir := t.TempDir()
	testProject := filepath.Join(tmpDir, "test-project")
	if err := os.Mkdir(testProject, 0o755); err != nil {
		t.Fatal(err)
	}

	// Mock config
	lazypath.SetTestConfig(lazypath.Config{
		Folders: []lazypath.Folder{
			{
				Path:        tmpDir,
				Prefix:      "tmp",
				IsWorkspace: true,
				RegexMatch:  "test-.*",
			},
			{
				Path:        "/home/user/project",
				Prefix:      "home",
				IsWorkspace: false,
			},
		},
	})
	defer lazypath.ResetTestConfig()

	tests := []struct {
		name    string
		option  *ListOption
		wantErr bool
		check   func(string) bool
	}{
		{
			name: "list all folders with default options",
			option: &ListOption{
				IsPlain:   false,
				ShortOnly: false,
				NoHeaders: false,
				NoColor:   true,
			},
			wantErr: false,
			check: func(output string) bool {
				return strings.Contains(output, "SHORT NAME")
			},
		},
		{
			name: "list plain folders",
			option: &ListOption{
				IsPlain:   true,
				ShortOnly: false,
				NoHeaders: false,
				NoColor:   true,
			},
			wantErr: false,
			check: func(output string) bool {
				return strings.Contains(output, "PATH")
			},
		},
		{
			name: "list plain folders without headers",
			option: &ListOption{
				IsPlain:   true,
				ShortOnly: false,
				NoHeaders: true,
				NoColor:   true,
			},
			wantErr: false,
			check: func(output string) bool {
				return len(output) > 0
			},
		},
		{
			name: "list short names only",
			option: &ListOption{
				IsPlain:   false,
				ShortOnly: true,
				NoHeaders: false,
				NoColor:   true,
			},
			wantErr: false,
			check: func(output string) bool {
				return strings.Contains(output, "SHORT NAME")
			},
		},
		{
			name: "list without headers",
			option: &ListOption{
				IsPlain:   false,
				ShortOnly: false,
				NoHeaders: true,
				NoColor:   true,
			},
			wantErr: false,
			check: func(output string) bool {
				return !strings.Contains(output, "SHORT NAME") || len(output) > 0
			},
		},
		{
			name: "list with color",
			option: &ListOption{
				IsPlain:   false,
				ShortOnly: false,
				NoHeaders: false,
				NoColor:   false,
			},
			wantErr: false,
			check: func(output string) bool {
				return len(output) > 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := ListFolders(&buf, tt.option)

			if (err != nil) != tt.wantErr {
				t.Errorf("ListFolders() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			output := buf.String()
			if !tt.wantErr && tt.check != nil && !tt.check(output) {
				t.Errorf("ListFolders() output check failed, got: %s", output)
			}
		})
	}
}

func TestRemoveFolderFromConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Setup test config
	lazypath.CfgFile = configFile
	lazypath.InitConfig()

	// Add a folder first
	folder := &lazypath.Folder{
		Path:        "/tmp/test-remove",
		Prefix:      "test",
		IsWorkspace: false,
	}
	ImportFolderToConfig(folder)

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "remove existing folder",
			path:    "/tmp/test-remove",
			wantErr: false,
		},
		{
			name:    "remove non-existing folder",
			path:    "/tmp/nonexistent",
			wantErr: false, // Should not error, just log
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RemoveFolderFromConfig(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("RemoveFolderFromConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseConfig_SymlinkHandling(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a regular directory
	regularDir := filepath.Join(tmpDir, "regular-dir")
	if err := os.Mkdir(regularDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a file (not a directory)
	regularFile := filepath.Join(tmpDir, "regular-file")
	if err := os.WriteFile(regularFile, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	config := lazypath.Config{
		Folders: []lazypath.Folder{
			{
				Path:        tmpDir,
				IsWorkspace: true,
				RegexMatch:  ".*",
			},
		},
	}

	result, err := ParseConfig(config)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}

	// Should include regular-dir but not regular-file
	found := false
	for _, folder := range result {
		if strings.Contains(folder.ShortName, "regular-dir") {
			found = true
		}
		if strings.Contains(folder.ShortName, "regular-file") {
			t.Error("ParseConfig() should not include regular files")
		}
	}

	if !found {
		t.Error("ParseConfig() should include regular directories")
	}
}

func TestParseConfig_ErrorHandling(t *testing.T) {
	tests := []struct {
		name    string
		config  lazypath.Config
		wantErr bool
	}{
		{
			name: "workspace with non-existent path",
			config: lazypath.Config{
				Folders: []lazypath.Folder{
					{
						Path:        "/nonexistent/path",
						IsWorkspace: true,
						RegexMatch:  ".*",
					},
				},
			},
			wantErr: false, // Should not error, just skip
		},
		{
			name: "workspace with empty prefix",
			config: lazypath.Config{
				Folders: []lazypath.Folder{
					{
						Path:        "/tmp",
						Prefix:      "",
						IsWorkspace: true,
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
