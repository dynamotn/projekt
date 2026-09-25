package folderutil

import (
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
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
		name      string
		list      []ParsedFolder
		shortName string
		path      string
		workspace string
		wantLen   int
	}{
		{
			name:      "add new folder",
			list:      []ParsedFolder{},
			shortName: "test-project1",
			path:      "/tmp/workspace/project1",
			workspace: "/tmp/workspace",
			wantLen:   1,
		},
		{
			name: "skip duplicate short name",
			list: []ParsedFolder{
				{ShortName: "test-project1", Path: "/tmp/other", Workspace: "/tmp"},
			},
			shortName: "test-project1",
			path:      "/tmp/workspace/project1",
			workspace: "/tmp/workspace",
			wantLen:   1, // Should not add
		},
		{
			name:      "folder without workspace",
			list:      []ParsedFolder{},
			shortName: "test-myproject",
			path:      "/tmp/workspace/myproject",
			workspace: "/tmp/workspace/myproject",
			wantLen:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendToParsedFolder(tt.list, shortNameSet(tt.list), tt.shortName, tt.path, tt.workspace, nil)
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

func TestParseConfig_PriorityWinsDuplicateShortName(t *testing.T) {
	tmpDir := t.TempDir()
	low := filepath.Join(tmpDir, "low", "myapp")
	high := filepath.Join(tmpDir, "high", "myapp")
	for _, dir := range []string{low, high} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	config := lazypath.Config{
		Folders: []lazypath.Folder{
			{Path: filepath.Join(tmpDir, "low"), IsWorkspace: true, Priority: 1},
			{Path: filepath.Join(tmpDir, "high"), IsWorkspace: true, Priority: 10},
		},
	}

	result, err := ParseConfig(config)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("ParseConfig() returned %d folders, want 1", len(result))
	}
	if result[0].Path != high {
		t.Errorf("ParseConfig() kept %s, want the higher priority %s", result[0].Path, high)
	}
}

func TestParseConfig_SymlinkToFileIsSkipped(t *testing.T) {
	tmpDir := t.TempDir()

	regular := filepath.Join(tmpDir, "project")
	if err := os.Mkdir(regular, 0o755); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(tmpDir, "target.txt")
	if err := os.WriteFile(target, []byte("not a folder"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(tmpDir, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	linkedDir := filepath.Join(tmpDir, "linked-dir")
	if err := os.Symlink(regular, linkedDir); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	config := lazypath.Config{
		Folders: []lazypath.Folder{{Path: tmpDir, IsWorkspace: true, RegexMatch: ".*"}},
	}

	result, err := ParseConfig(config)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}

	names := make(map[string]bool, len(result))
	for _, folder := range result {
		names[folder.ShortName] = true
	}

	if !names["project"] || !names["linked-dir"] {
		t.Errorf("ParseConfig() = %v, want the directory and the symlink to a directory", names)
	}
	if names["linked"] {
		t.Error("ParseConfig() included a symlink pointing at a regular file")
	}
	if names["target.txt"] {
		t.Error("ParseConfig() included a regular file")
	}
}

func TestParseConfig_UsesConfiguredName(t *testing.T) {
	tmpDir := t.TempDir()
	dotfiles := filepath.Join(tmpDir, "Dotfiles")
	if err := os.MkdirAll(filepath.Join(dotfiles, "home"), 0o755); err != nil {
		t.Fatal(err)
	}

	config := lazypath.Config{
		Folders: []lazypath.Folder{
			// The path deliberately walks back out of "home", the way a
			// chezmoi-style configuration does.
			{Path: filepath.Join(dotfiles, "home", ".."), Name: "dot"},
			{Path: filepath.Join(dotfiles, "home"), Prefix: "x"},
		},
	}

	result, err := ParseConfig(config)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("ParseConfig() returned %d folders, want 2", len(result))
	}

	if result[0].ShortName != "dot" {
		t.Errorf("ShortName = %q, want dot", result[0].ShortName)
	}
	if result[0].Path != dotfiles {
		t.Errorf("Path = %q, want the cleaned %q", result[0].Path, dotfiles)
	}
	// Without a name the folder falls back to the last element of the path,
	// and the prefix still applies.
	if result[1].ShortName != "x-home" {
		t.Errorf("ShortName = %q, want x-home", result[1].ShortName)
	}
}
