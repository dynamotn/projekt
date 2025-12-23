package folderutil

import (
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  lazypath.Config
		wantLen int
		wantErr bool
	}{
		{
			name: "empty config",
			config: lazypath.Config{
				Folders: []lazypath.Folder{},
			},
			wantLen: 0,
			wantErr: false,
		},
		{
			name: "single non-workspace folder",
			config: lazypath.Config{
				Folders: []lazypath.Folder{
					{
						Path:        "/tmp/test",
						Prefix:      "test",
						IsWorkspace: false,
					},
				},
			},
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "invalid regex pattern",
			config: lazypath.Config{
				Folders: []lazypath.Folder{
					{
						Path:        "/tmp/test",
						IsWorkspace: true,
						RegexMatch:  "[invalid",
					},
				},
			},
			wantLen: 0,
			wantErr: false, // Should not error, just skip
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != tt.wantLen {
				t.Errorf("ParseConfig() got %d folders, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestAppendToParsedFolder(t *testing.T) {
	tests := []struct {
		name            string
		list            []ParsedFolder
		prefix          string
		folderPath      string
		childFolderName string
		wantLen         int
	}{
		{
			name:            "add new folder",
			list:            []ParsedFolder{},
			prefix:          "test-",
			folderPath:      "/tmp/workspace",
			childFolderName: "project1",
			wantLen:         1,
		},
		{
			name: "skip duplicate short name",
			list: []ParsedFolder{
				{ShortName: "test-project1", Path: "/tmp/other", Workspace: "/tmp"},
			},
			prefix:          "test-",
			folderPath:      "/tmp/workspace",
			childFolderName: "project1",
			wantLen:         1, // Should not add
		},
		{
			name:            "folder without child name",
			list:            []ParsedFolder{},
			prefix:          "test-",
			folderPath:      "/tmp/workspace/myproject",
			childFolderName: "",
			wantLen:         1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendToParsedFolder(tt.list, tt.prefix, tt.folderPath, tt.childFolderName)
			if len(got) != tt.wantLen {
				t.Errorf("appendToParsedFolder() got %d folders, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestParsedFolderPaths(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test directory structure
	testProject := filepath.Join(tmpDir, "test-project")
	if err := os.Mkdir(testProject, 0o755); err != nil {
		t.Fatal(err)
	}

	config := lazypath.Config{
		Folders: []lazypath.Folder{
			{
				Path:        tmpDir,
				Prefix:      "tmp",
				IsWorkspace: true,
				RegexMatch:  "test-.*",
			},
		},
	}

	result, err := ParseConfig(config)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}

	if len(result) != 1 {
		t.Errorf("Expected 1 folder, got %d", len(result))
	}

	if len(result) > 0 {
		if result[0].ShortName != "tmp-test-project" {
			t.Errorf("Expected short name 'tmp-test-project', got '%s'", result[0].ShortName)
		}
		if result[0].Workspace != tmpDir {
			t.Errorf("Expected workspace '%s', got '%s'", tmpDir, result[0].Workspace)
		}
	}
}
