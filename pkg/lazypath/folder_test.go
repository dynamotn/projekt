package lazypath

import (
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
