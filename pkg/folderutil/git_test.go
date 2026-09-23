package folderutil

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

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
			expected: "ssh://git@git.test.dev:2022/GROUP/SUBGROUP/myrepo.git",
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
			expectedFallback: "git@github.com:myorg/myteam/project.git",
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

	jobs, err := planFolderSync(folder)
	if err != nil {
		t.Fatalf("planFolderSync failed: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("planFolderSync returned %d jobs, want 1", len(jobs))
	}

	// Dry run should not clone repos
	if errs := runSyncJobs(jobs, SyncOptions{DryRun: true}); len(errs) != 0 {
		t.Errorf("runSyncJobs(dryRun=true) errors = %v", errs)
	}
	if _, err := os.Stat(jobs[0].repoPath); !os.IsNotExist(err) {
		t.Errorf("dry run created %s", jobs[0].repoPath)
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

	err := SyncGitRepos(SyncOptions{DryRun: true})
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

func TestSyncGitRepos_NoGitConfig(t *testing.T) {
	// A folder without a git section is skipped entirely, so syncing must
	// neither fail nor bring the folder into existence.
	folderPath := filepath.Join(t.TempDir(), "testfolder")

	lazypath.SetTestConfig(lazypath.Config{
		Folders: []lazypath.Folder{
			{
				Path:        folderPath,
				IsWorkspace: true,
				Git:         nil,
			},
		},
	})

	if err := SyncGitRepos(SyncOptions{}); err != nil {
		t.Errorf("SyncGitRepos() with no Git config should not error, got: %v", err)
	}
	if _, err := os.Stat(folderPath); !os.IsNotExist(err) {
		t.Errorf("SyncGitRepos() created %s for a folder with no Git config", folderPath)
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
			expected: "ssh://git@gitlab.com:2222/group/repo.git",
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

func TestRepoTargetPath(t *testing.T) {
	folder := lazypath.Folder{Path: "/tmp/workspace"}

	tests := []struct {
		name string
		repo lazypath.GitRepo
		want string
	}{
		{
			name: "explicit path",
			repo: lazypath.GitRepo{Name: "backend", Path: "api"},
			want: "/tmp/workspace/api",
		},
		{
			name: "path defaults to repo name",
			repo: lazypath.GitRepo{Name: "backend"},
			want: "/tmp/workspace/backend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := repoTargetPath(folder, tt.repo); got != tt.want {
				t.Errorf("repoTargetPath() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestEffectiveForks(t *testing.T) {
	tests := []struct {
		name     string
		opts     SyncOptions
		jobCount int
		want     int
	}{
		{
			name:     "unset falls back to the default",
			opts:     SyncOptions{},
			jobCount: 100,
			want:     DefaultSyncForks,
		},
		{
			name:     "negative falls back to the default",
			opts:     SyncOptions{Forks: -3},
			jobCount: 100,
			want:     DefaultSyncForks,
		},
		{
			name:     "one stays sequential",
			opts:     SyncOptions{Forks: 1},
			jobCount: 100,
			want:     1,
		},
		{
			name:     "explicit value is honoured",
			opts:     SyncOptions{Forks: 8},
			jobCount: 100,
			want:     8,
		},
		{
			name:     "clamped to the number of jobs",
			opts:     SyncOptions{Forks: 16},
			jobCount: 3,
			want:     3,
		},
		{
			name:     "a dry run stays on one worker",
			opts:     SyncOptions{DryRun: true, Forks: 16},
			jobCount: 100,
			want:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := effectiveForks(tt.opts, tt.jobCount); got != tt.want {
				t.Errorf("effectiveForks(%+v, %d) = %d, want %d", tt.opts, tt.jobCount, got, tt.want)
			}
		})
	}
}

func TestRunSyncJobs_ExistingReposRunConcurrently(t *testing.T) {
	// Repositories that are already checked out are left alone, which lets this
	// exercise the worker pool without cloning anything. Under -race it is also
	// what catches the workers sharing state they should not.
	tempDir := t.TempDir()
	server := &lazypath.GitServer{
		Name:  "github",
		Type:  "github",
		HTTPS: "https://github.com",
		SSH:   "git@github.com",
	}

	var jobs []syncJob
	for i := range 12 {
		name := fmt.Sprintf("repo%d", i)
		repoPath := filepath.Join(tempDir, name)
		if err := os.MkdirAll(repoPath, 0o755); err != nil {
			t.Fatalf("failed to create %s: %v", repoPath, err)
		}
		jobs = append(jobs, syncJob{
			group:    "test/group",
			server:   server,
			repo:     lazypath.GitRepo{Name: name},
			repoPath: repoPath,
		})
	}

	if errs := runSyncJobs(jobs, SyncOptions{Forks: 4}); len(errs) != 0 {
		t.Errorf("runSyncJobs() errors = %v, want none", errs)
	}
}

// namedJobs builds a job per name, with the fields the pool cares about.
func namedJobs(names ...string) []syncJob {
	jobs := make([]syncJob, 0, len(names))
	for _, name := range names {
		jobs = append(jobs, syncJob{
			group:    "test/group",
			server:   &lazypath.GitServer{Name: "github", HTTPS: "https://github.com", SSH: "git@github.com"},
			repo:     lazypath.GitRepo{Name: name},
			repoPath: filepath.Join("/nonexistent", name),
		})
	}
	return jobs
}

func TestRunSyncJobsWith_SingleForkKeepsConfigOrder(t *testing.T) {
	// A one-slot pool would not be enough: goroutines reach the semaphore in
	// whatever order the scheduler runs them, so the sequential path has to run
	// inline to report repositories in the order they are configured.
	names := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
	jobs := namedJobs(names...)

	// Repeat, because an ordering bug here shows up only sometimes.
	for attempt := range 50 {
		var order []string
		errs := runSyncJobsWith(jobs, SyncOptions{Forks: 1}, func(job syncJob, _ bool) error {
			order = append(order, job.repo.Name)
			return nil
		})
		if len(errs) != 0 {
			t.Fatalf("runSyncJobsWith() errors = %v", errs)
		}

		if len(order) != len(names) {
			t.Fatalf("attempt %d visited %d jobs, want %d", attempt, len(order), len(names))
		}
		for i, name := range names {
			if order[i] != name {
				t.Fatalf("attempt %d: job %d = %s, want %s", attempt, i, order[i], name)
			}
		}
	}
}

func TestRunSyncJobsWith_HonoursForkLimit(t *testing.T) {
	const (
		jobCount = 24
		forks    = 4
	)

	names := make([]string, 0, jobCount)
	for i := range jobCount {
		names = append(names, fmt.Sprintf("repo%d", i))
	}

	var mu sync.Mutex
	running, peak := 0, 0

	errs := runSyncJobsWith(namedJobs(names...), SyncOptions{Forks: forks}, func(syncJob, bool) error {
		mu.Lock()
		running++
		if running > peak {
			peak = running
		}
		mu.Unlock()

		// Hold the slot long enough that the other workers pile up behind it.
		time.Sleep(2 * time.Millisecond)

		mu.Lock()
		running--
		mu.Unlock()

		return nil
	})
	if len(errs) != 0 {
		t.Fatalf("runSyncJobsWith() errors = %v", errs)
	}

	if peak > forks {
		t.Errorf("ran %d jobs at once, want at most %d", peak, forks)
	}
	// Without real concurrency the whole point of the pool is lost, so make sure
	// more than one worker actually got going.
	if peak < 2 {
		t.Errorf("peak concurrency was %d, expected the pool to overlap jobs", peak)
	}
}

func TestRunSyncJobsWith_ErrorsKeepConfigOrder(t *testing.T) {
	// Errors are collected by job index, so which worker finishes first must not
	// change what the command reports.
	jobs := namedJobs("alpha", "beta", "gamma", "delta")

	errs := runSyncJobsWith(jobs, SyncOptions{Forks: 4}, func(job syncJob, _ bool) error {
		switch job.repo.Name {
		case "alpha":
			// Finish last despite being first in configuration.
			time.Sleep(10 * time.Millisecond)
			return fmt.Errorf("clone %s", job.repo.Name)
		case "gamma":
			return fmt.Errorf("clone %s", job.repo.Name)
		default:
			return nil
		}
	})

	want := []string{"clone alpha", "clone gamma"}
	if len(errs) != len(want) {
		t.Fatalf("runSyncJobsWith() returned %d errors (%v), want %d", len(errs), errs, len(want))
	}
	for i, w := range want {
		if errs[i].Error() != w {
			t.Errorf("error %d = %q, want %q", i, errs[i].Error(), w)
		}
	}
}

func TestRunSyncJobs_NoJobs(t *testing.T) {
	if errs := runSyncJobs(nil, SyncOptions{Forks: 4}); errs != nil {
		t.Errorf("runSyncJobs(nil) = %v, want nil", errs)
	}
}
