package folderutil

import (
	"path/filepath"
	"testing"

	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

func TestGitConfig(t *testing.T) {
	tests := []struct {
		name     string
		folder   lazypath.Folder
		wantNil  bool
		wantHost string
	}{
		{
			name: "folder without git config",
			folder: lazypath.Folder{
				Path:        "/tmp/test",
				IsWorkspace: true,
			},
			wantNil: true,
		},
		{
			name: "folder with git config",
			folder: lazypath.Folder{
				Path:        "/tmp/test",
				IsWorkspace: true,
				Git: &lazypath.GitConfig{
					Host:  "git3",
					Group: "test/group",
					Repos: []lazypath.GitRepo{
						{Name: "repo1", Path: "repo1"},
					},
				},
			},
			wantNil:  false,
			wantHost: "git3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantNil && tt.folder.Git != nil {
				t.Errorf("expected nil Git config, got %v", tt.folder.Git)
			}
			if !tt.wantNil && tt.folder.Git == nil {
				t.Errorf("expected non-nil Git config, got nil")
			}
			if !tt.wantNil && tt.folder.Git.Host != tt.wantHost {
				t.Errorf("expected host %s, got %s", tt.wantHost, tt.folder.Git.Host)
			}
		})
	}
}

func TestGitServer(t *testing.T) {
	tests := []struct {
		name     string
		server   lazypath.GitServer
		expected string
	}{
		{
			name: "gitlab server with ssh format",
			server: lazypath.GitServer{
				Name:  "git3",
				Type:  "gitlab",
				HTTPS: "https://git.test.dev",
				SSH:   "ssh://git@git.test.dev:2022",
			},
			expected: "git3",
		},
		{
			name: "github server",
			server: lazypath.GitServer{
				Name:  "github",
				Type:  "github",
				HTTPS: "https://github.com",
				SSH:   "git@github.com",
			},
			expected: "github",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.server.Name != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.server.Name)
			}
		})
	}
}

func TestBuildGitURL(t *testing.T) {
	tests := []struct {
		name     string
		server   lazypath.GitServer
		group    string
		repoName string
		expected string
	}{
		{
			name: "ssh with port format",
			server: lazypath.GitServer{
				SSH: "ssh://git@git.test.dev:2022",
			},
			group:    "GROUP/SUBGROUP",
			repoName: "myrepo",
			expected: "git@git.test.dev:2022/GROUP/SUBGROUP/myrepo.git",
		},
		{
			name: "standard git format",
			server: lazypath.GitServer{
				SSH: "git@github.com",
			},
			group:    "myorg/myteam",
			repoName: "project",
			expected: "git@github.com:myorg/myteam/project.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildGitURL(&tt.server, tt.group, tt.repoName)
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestBuildHTTPSURL(t *testing.T) {
	tests := []struct {
		name     string
		server   lazypath.GitServer
		group    string
		repoName string
		expected string
	}{
		{
			name: "gitlab https",
			server: lazypath.GitServer{
				HTTPS: "https://git.test.dev",
			},
			group:    "GROUP/SUBGROUP",
			repoName: "myrepo",
			expected: "https://git.test.dev/GROUP/SUBGROUP/myrepo.git",
		},
		{
			name: "github https",
			server: lazypath.GitServer{
				HTTPS: "https://github.com",
			},
			group:    "myorg/myteam",
			repoName: "project",
			expected: "https://github.com/myorg/myteam/project.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildHTTPSURL(&tt.server, tt.group, tt.repoName)
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestGetGitURLs(t *testing.T) {
	tests := []struct {
		name             string
		server           lazypath.GitServer
		group            string
		repoName         string
		expectedPrimary  string
		expectedFallback string
	}{
		{
			name: "preferGitSSH true",
			server: lazypath.GitServer{
				SSH:          "git@github.com",
				HTTPS:        "https://github.com",
				PreferGitSSH: true,
			},
			group:            "myorg/myteam",
			repoName:         "project",
			expectedPrimary:  "git@github.com:myorg/myteam/project.git",
			expectedFallback: "https://github.com/myorg/myteam/project.git",
		},
		{
			name: "preferGitSSH false",
			server: lazypath.GitServer{
				SSH:          "git@github.com",
				HTTPS:        "https://github.com",
				PreferGitSSH: false,
			},
			group:            "myorg/myteam",
			repoName:         "project",
			expectedPrimary:  "https://github.com/myorg/myteam/project.git",
			expectedFallback: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			primary, fallback := getGitURLs(&tt.server, tt.group, tt.repoName)
			if primary != tt.expectedPrimary {
				t.Errorf("expected primary %s, got %s", tt.expectedPrimary, primary)
			}
			if fallback != tt.expectedFallback {
				t.Errorf("expected fallback %s, got %s", tt.expectedFallback, fallback)
			}
		})
	}
}

func TestGetGitServer(t *testing.T) {
	// Mock config with git servers
	lazypath.SetTestConfig(lazypath.Config{
		GitServers: []lazypath.GitServer{
			{
				Name:  "git3",
				Type:  "gitlab",
				HTTPS: "https://test.git.dev",
				SSH:   "ssh://git@test.git.dev:2022",
			},
			{
				Name:  "github",
				Type:  "github",
				HTTPS: "https://github.com",
				SSH:   "git@github.com",
			},
		},
	})

	tests := []struct {
		name     string
		hostName string
		wantNil  bool
	}{
		{
			name:     "existing server git3",
			hostName: "git3",
			wantNil:  false,
		},
		{
			name:     "existing server github",
			hostName: "github",
			wantNil:  false,
		},
		{
			name:     "non-existing server",
			hostName: "unknown",
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getGitServer(tt.hostName)
			if tt.wantNil && got != nil {
				t.Errorf("expected nil, got %v", got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("expected non-nil lazypath.GitServer, got nil")
			}
			if !tt.wantNil && got.Name != tt.hostName {
				t.Errorf("expected name %s, got %s", tt.hostName, got.Name)
			}
		})
	}
}

func TestSyncFolderGitRepos_DryRun(t *testing.T) {
	tempDir := t.TempDir()

	folder := lazypath.Folder{
		Path:        filepath.Join(tempDir, "testfolder"),
		IsWorkspace: true,
		Git: &lazypath.GitConfig{
			Host:  "github",
			Group: "test/group",
			Repos: []lazypath.GitRepo{
				{Name: "repo1", Path: "repo1"},
			},
		},
	}

	// Mock config with git servers
	lazypath.SetTestConfig(lazypath.Config{
		GitServers: []lazypath.GitServer{
			{
				Name:  "github",
				Type:  "github",
				HTTPS: "https://github.com",
				SSH:   "git@github.com",
			},
		},
	})

	// Dry run should not clone repos
	err := syncFolderGitRepos(folder, true)
	if err != nil {
		t.Errorf("syncFolderGitRepos failed: %v", err)
	}
}

func TestCheckFolderGitRepos(t *testing.T) {
	tempDir := t.TempDir()

	folder := lazypath.Folder{
		Path:        tempDir,
		IsWorkspace: true,
		Git: &lazypath.GitConfig{
			Host:  "github",
			Group: "test/group",
			Repos: []lazypath.GitRepo{
				{Name: "nonexistent", Path: "nonexistent"},
			},
		},
	}

	// Mock config with git servers
	lazypath.SetTestConfig(lazypath.Config{
		GitServers: []lazypath.GitServer{
			{
				Name:  "github",
				Type:  "github",
				HTTPS: "https://github.com",
				SSH:   "git@github.com",
			},
		},
	})

	// This should not fail even if repos don't exist
	err := checkFolderGitRepos(folder)
	if err != nil {
		t.Errorf("checkFolderGitRepos failed: %v", err)
	}
}

func TestGetURLType(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "https url",
			url:  "https://github.com/org/repo.git",
			want: "HTTPS",
		},
		{
			name: "http url",
			url:  "http://github.com/org/repo.git",
			want: "HTTPS",
		},
		{
			name: "ssh url",
			url:  "git@github.com:org/repo.git",
			want: "SSH",
		},
		{
			name: "ssh:// url",
			url:  "ssh://git@github.com/org/repo.git",
			want: "SSH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getURLType(tt.url)
			if got != tt.want {
				t.Errorf("getURLType(%s) = %s, want %s", tt.url, got, tt.want)
			}
		})
	}
}

func TestSyncGitRepos(t *testing.T) {
	tempDir := t.TempDir()

	lazypath.SetTestConfig(lazypath.Config{
		Folders: []lazypath.Folder{
			{
				Path:        tempDir,
				IsWorkspace: true,
				Git: &lazypath.GitConfig{
					Host:  "github",
					Group: "test",
					Repos: []lazypath.GitRepo{
						{Name: "repo1", Path: "repo1"},
					},
				},
			},
		},
		GitServers: []lazypath.GitServer{
			{
				Name:         "github",
				Type:         "github",
				HTTPS:        "https://github.com",
				SSH:          "git@github.com",
				PreferGitSSH: true,
			},
		},
	})

	err := SyncGitRepos(true)
	if err != nil {
		t.Errorf("SyncGitRepos(dryRun=true) error = %v", err)
	}
}

func TestCheckGitReposStatus(t *testing.T) {
	tempDir := t.TempDir()

	lazypath.SetTestConfig(lazypath.Config{
		Folders: []lazypath.Folder{
			{
				Path:        tempDir,
				IsWorkspace: true,
				Git: &lazypath.GitConfig{
					Host:  "github",
					Group: "test",
					Repos: []lazypath.GitRepo{
						{Name: "repo1", Path: "repo1"},
					},
				},
			},
		},
		GitServers: []lazypath.GitServer{
			{
				Name:  "github",
				Type:  "github",
				HTTPS: "https://github.com",
				SSH:   "git@github.com",
			},
		},
	})

	err := CheckGitReposStatus()
	if err != nil {
		t.Errorf("CheckGitReposStatus() error = %v", err)
	}
}

func TestSyncFolderGitRepos_NoGitConfig(t *testing.T) {
	folder := lazypath.Folder{
		Path:        "/tmp/test",
		IsWorkspace: true,
		Git:         nil,
	}

	err := syncFolderGitRepos(folder, false)
	if err != nil {
		t.Errorf("syncFolderGitRepos() with no Git config should not error, got: %v", err)
	}
}

func TestCheckFolderGitRepos_NoGitConfig(t *testing.T) {
	folder := lazypath.Folder{
		Path:        "/tmp/test",
		IsWorkspace: true,
		Git:         nil,
	}

	err := checkFolderGitRepos(folder)
	if err != nil {
		t.Errorf("checkFolderGitRepos() with no Git config should not error, got: %v", err)
	}
}

func TestBuildGitURL_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		server   lazypath.GitServer
		group    string
		repoName string
		expected string
	}{
		{
			name: "ssh format without @ and ssh:// prefix",
			server: lazypath.GitServer{
				SSH: "gitlab.com",
			},
			group:    "group",
			repoName: "repo",
			expected: "git@gitlab.com:group/repo.git",
		},
		{
			name: "ssh with @ but no ssh:// prefix",
			server: lazypath.GitServer{
				SSH: "git@gitlab.com:2222",
			},
			group:    "group",
			repoName: "repo",
			expected: "git@gitlab.com:2222:group/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildGitURL(&tt.server, tt.group, tt.repoName)
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}
