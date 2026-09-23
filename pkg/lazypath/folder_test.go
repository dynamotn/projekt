package lazypath

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestGetRegexMatch(t *testing.T) {
	tests := []struct {
		name   string
		folder Folder
		want   string
	}{
		{
			name: "non-workspace folder",
			folder: Folder{
				Path:        "/tmp/test",
				IsWorkspace: false,
			},
			want: "",
		},
		{
			name: "workspace with custom regex",
			folder: Folder{
				Path:        "/tmp/test",
				IsWorkspace: true,
				RegexMatch:  "^project-.*",
			},
			want: "^project-.*",
		},
		{
			name: "workspace with default regex",
			folder: Folder{
				Path:        "/tmp/test",
				IsWorkspace: true,
				RegexMatch:  "",
			},
			want: defaultRegexWorkspace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.folder.GetRegexMatch(); got != tt.want {
				t.Errorf("GetRegexMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckFolderExist(t *testing.T) {
	// Reset config for testing
	c = Config{
		Folders: []Folder{
			{Path: "/tmp/test1"},
			{Path: "/tmp/test2/"},
			{Path: "/home/user/projects"},
		},
	}

	tests := []struct {
		name      string
		path      string
		wantExist bool
		wantIndex int
	}{
		{
			name:      "exact match",
			path:      "/tmp/test1",
			wantExist: true,
			wantIndex: 0,
		},
		{
			name:      "match with trailing slash",
			path:      "/tmp/test2",
			wantExist: true,
			wantIndex: 1,
		},
		{
			name:      "match without trailing slash when config has it",
			path:      "/tmp/test2/",
			wantExist: true,
			wantIndex: 1,
		},
		{
			name:      "non-existent path",
			path:      "/tmp/nonexistent",
			wantExist: false,
			wantIndex: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotExist, gotIndex := CheckFolderExist(tt.path)
			if gotExist != tt.wantExist {
				t.Errorf("CheckFolderExist() gotExist = %v, want %v", gotExist, tt.wantExist)
			}
			if tt.wantExist && gotIndex != tt.wantIndex {
				t.Errorf("CheckFolderExist() gotIndex = %v, want %v", gotIndex, tt.wantIndex)
			}
		})
	}
}

func TestAddToConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Save and restore original state
	originalCfgFile := CfgFile
	defer func() {
		CfgFile = originalCfgFile
		c = Config{}
	}()

	// Setup test config
	c = Config{}
	CfgFile = configFile
	InitConfig()

	tests := []struct {
		name    string
		folder  Folder
		wantErr bool
	}{
		{
			name: "add new folder",
			folder: Folder{
				Path:        "/tmp/new-folder",
				Prefix:      "new",
				IsWorkspace: false,
			},
			wantErr: false,
		},
		{
			name: "add duplicate folder",
			folder: Folder{
				Path:        "/tmp/new-folder",
				Prefix:      "new",
				IsWorkspace: false,
			},
			wantErr: false, // Should not error, just log
		},
		{
			name: "add workspace folder",
			folder: Folder{
				Path:        "/tmp/workspace",
				Prefix:      "ws",
				IsWorkspace: true,
				RegexMatch:  "^[^.].+",
				Priority:    10,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.folder.AddToConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("AddToConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRemoveFromConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Save and restore original state
	originalCfgFile := CfgFile
	defer func() {
		CfgFile = originalCfgFile
		c = Config{}
	}()

	// Setup test config
	c = Config{}
	CfgFile = configFile
	InitConfig()

	// Add a folder first
	folder := Folder{
		Path:   "/tmp/to-remove",
		Prefix: "remove",
	}
	folder.AddToConfig()

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "remove existing folder",
			path:    "/tmp/to-remove",
			wantErr: false,
		},
		{
			name:    "remove non-existing folder",
			path:    "/tmp/nonexistent",
			wantErr: false, // Should not error, just log
		},
		{
			name:    "remove with trailing slash",
			path:    "/tmp/to-remove/",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RemoveFromConfig(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("RemoveFromConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAddToConfig_WithGitConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Save and restore
	originalCfgFile := CfgFile
	defer func() {
		CfgFile = originalCfgFile
		// Force full reset
		c = Config{}
	}()

	c = Config{}
	CfgFile = configFile
	InitConfig()

	folder := Folder{
		Path:        "/tmp/git-project",
		Prefix:      "git",
		IsWorkspace: true,
		Git: &GitConfig{
			Host:  "github",
			Group: "myorg",
			Repos: []GitRepo{
				{Name: "repo1", Path: "repo1"},
				{Name: "repo2", Path: "repo2"},
			},
		},
	}

	err := folder.AddToConfig()
	if err != nil {
		t.Errorf("AddToConfig() with GitConfig error = %v", err)
	}

	// Verify by checking c directly after unmarshal
	c = Config{} // Reset to force unmarshal
	config := GetConfig()
	found := false
	for _, f := range config.Folders {
		if f.Path == "/tmp/git-project" {
			found = true
			if f.Git == nil {
				t.Error("AddToConfig() did not preserve GitConfig")
			}
			if f.Git != nil && len(f.Git.Repos) != 2 {
				t.Errorf("AddToConfig() GitConfig repos count = %d, want 2", len(f.Git.Repos))
			}
		}
	}
	if !found {
		t.Error("AddToConfig() did not add folder with GitConfig")
	}
}

func TestCheckFolderExist_EdgeCases(t *testing.T) {
	c = Config{
		Folders: []Folder{
			{Path: "/"},
			{Path: "/home/"},
			{Path: ""},
		},
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "root path",
			path: "/",
			want: true,
		},
		{
			name: "empty path matches empty config path",
			path: "",
			want: true,
		},
		{
			name: "path with multiple slashes",
			path: "/home//",
			want: true, // TrimRight will normalize this to /home which matches /home/
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := CheckFolderExist(tt.path)
			if got != tt.want {
				t.Errorf("CheckFolderExist(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestRemoveFromConfig_MultipleRemoves(t *testing.T) {
	t.Skip("Skipping due to global state interference in test suite")
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Save original state
	originalCfgFile := CfgFile
	defer func() {
		CfgFile = originalCfgFile
		c = Config{}
	}()

	// Reset config and setup fresh
	c = Config{}
	CfgFile = configFile
	InitConfig()

	// Add multiple folders
	folders := []Folder{
		{Path: "/tmp/folder1", Prefix: "f1"},
		{Path: "/tmp/folder2", Prefix: "f2"},
		{Path: "/tmp/folder3", Prefix: "f3"},
	}

	for _, f := range folders {
		f.AddToConfig()
	}

	// Remove middle folder
	err := RemoveFromConfig("/tmp/folder2")
	if err != nil {
		t.Errorf("RemoveFromConfig() error = %v", err)
	}

	// Force re-read config
	c = Config{}
	config := GetConfig()
	if len(config.Folders) != 2 {
		t.Errorf("After RemoveFromConfig(), got %d folders, want 2", len(config.Folders))
	}

	// Verify correct folders remain
	paths := make(map[string]bool)
	for _, f := range config.Folders {
		paths[f.Path] = true
	}

	if !paths["/tmp/folder1"] || !paths["/tmp/folder3"] {
		t.Error("RemoveFromConfig() removed wrong folders")
	}
	if paths["/tmp/folder2"] {
		t.Error("RemoveFromConfig() did not remove the target folder")
	}
}

func TestFolderShortName(t *testing.T) {
	tests := []struct {
		name   string
		folder Folder
		want   string
	}{
		{
			name:   "configured name wins",
			folder: Folder{Path: "/home/me/Dotfiles/home/..", Name: "dot"},
			want:   "dot",
		},
		{
			name:   "path with dot segments is cleaned",
			folder: Folder{Path: "/home/me/Dotfiles/home/.."},
			want:   "Dotfiles",
		},
		{
			name:   "trailing slash",
			folder: Folder{Path: "/home/me/projects/myapp/"},
			want:   "myapp",
		},
		{
			name:   "plain path",
			folder: Folder{Path: "/home/me/projects/myapp"},
			want:   "myapp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.folder.ShortName(); got != tt.want {
				t.Errorf("ShortName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeTags(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want []string
	}{
		{name: "nil stays nil", tags: nil, want: nil},
		{name: "empty stays nil", tags: []string{}, want: nil},
		{name: "kept in the given order", tags: []string{"work", "go"}, want: []string{"work", "go"}},
		{name: "trimmed", tags: []string{"  work  "}, want: []string{"work"}},
		{name: "blanks dropped", tags: []string{"work", "", "   "}, want: []string{"work"}},
		{name: "only blanks become nil", tags: []string{"", "  "}, want: nil},
		{name: "duplicates dropped", tags: []string{"go", "work", "go"}, want: []string{"go", "work"}},
		{name: "duplicates after trimming", tags: []string{"go", " go "}, want: []string{"go"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeTags(tt.tags); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NormalizeTags(%q) = %q, want %q", tt.tags, got, tt.want)
			}
		})
	}
}

func TestHasTags(t *testing.T) {
	tests := []struct {
		name string
		have []string
		want []string
		ok   bool
	}{
		{name: "no wanted tags matches anything", have: nil, want: nil, ok: true},
		{name: "no wanted tags matches a tagged folder", have: []string{"go"}, want: nil, ok: true},
		{name: "only blank wanted tags is no filter", have: nil, want: []string{" ", ""}, ok: true},
		{name: "single match", have: []string{"go", "work"}, want: []string{"work"}, ok: true},
		{name: "every wanted tag must be present", have: []string{"go", "work"}, want: []string{"go", "work"}, ok: true},
		{name: "one missing tag fails the match", have: []string{"go"}, want: []string{"go", "work"}, ok: false},
		{name: "an untagged folder matches nothing", have: nil, want: []string{"go"}, ok: false},
		{name: "matching is exact, not a prefix", have: []string{"golang"}, want: []string{"go"}, ok: false},
		{name: "whitespace is forgiven on both sides", have: []string{" go "}, want: []string{"go "}, ok: true},
		{name: "matching is case sensitive", have: []string{"Go"}, want: []string{"go"}, ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasTags(tt.have, tt.want); got != tt.ok {
				t.Errorf("HasTags(%q, %q) = %v, want %v", tt.have, tt.want, got, tt.ok)
			}
		})
	}
}

func TestFolderGetTags(t *testing.T) {
	f := Folder{Tags: []string{" work ", "", "go", "work"}}
	if got := f.GetTags(); !reflect.DeepEqual(got, []string{"work", "go"}) {
		t.Errorf("GetTags() = %q, want [work go]", got)
	}
}
