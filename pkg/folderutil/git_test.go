package folderutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
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

	err := CheckGitReposStatus(CheckOptions{})
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

// initRepo creates a real repository with one commit, which is the least a
// worktree needs to be attachable.
func initRepo(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}

	mustGit(t, path, "init", "--initial-branch=main")
	// A commit needs an identity, and the one on the machine running the tests
	// is none of this test's business.
	mustGit(t, path, "config", "user.email", "test@example.com")
	mustGit(t, path, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(path, "README"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, path, "add", "README")
	mustGit(t, path, "commit", "-m", "initial")
}

func mustGit(t *testing.T, repoPath string, args ...string) {
	t.Helper()

	if err := runGit(repoPath, args...); err != nil {
		t.Fatalf("git %v in %s: %v", args, repoPath, err)
	}
}

func TestEnsureRemotes(t *testing.T) {
	repoPath := filepath.Join(t.TempDir(), "repo")
	initRepo(t, repoPath)

	job := syncJob{
		repoPath: repoPath,
		repo: lazypath.GitRepo{
			Name:    "repo",
			Remotes: map[string]string{"upstream": "https://example.com/up.git"},
		},
	}

	if err := ensureRemotes(job, false); err != nil {
		t.Fatalf("ensureRemotes() error = %v", err)
	}

	remotes, err := gitRemotes(repoPath)
	if err != nil {
		t.Fatalf("gitRemotes() error = %v", err)
	}
	if got := remotes["upstream"]; got != "https://example.com/up.git" {
		t.Fatalf("upstream = %q, want the configured URL", got)
	}

	// Running again with the same configuration must be a no-op rather than an
	// error: sync is expected to be safe to repeat.
	if err := ensureRemotes(job, false); err != nil {
		t.Fatalf("second ensureRemotes() error = %v", err)
	}

	// A changed URL is repointed, not duplicated or left stale.
	job.repo.Remotes["upstream"] = "https://example.com/moved.git"
	if err := ensureRemotes(job, false); err != nil {
		t.Fatalf("ensureRemotes() after URL change error = %v", err)
	}

	remotes, err = gitRemotes(repoPath)
	if err != nil {
		t.Fatalf("gitRemotes() error = %v", err)
	}
	if got := remotes["upstream"]; got != "https://example.com/moved.git" {
		t.Errorf("upstream = %q, want the updated URL", got)
	}
}

func TestEnsureRemotes_DryRunChangesNothing(t *testing.T) {
	repoPath := filepath.Join(t.TempDir(), "repo")
	initRepo(t, repoPath)

	job := syncJob{
		repoPath: repoPath,
		repo: lazypath.GitRepo{
			Name:    "repo",
			Remotes: map[string]string{"upstream": "https://example.com/up.git"},
		},
	}

	if err := ensureRemotes(job, true); err != nil {
		t.Fatalf("ensureRemotes(dryRun) error = %v", err)
	}

	remotes, err := gitRemotes(repoPath)
	if err != nil {
		t.Fatalf("gitRemotes() error = %v", err)
	}
	if _, present := remotes["upstream"]; present {
		t.Error("a dry run added the remote")
	}
}

func TestEnsureRemotes_NoRemotesConfigured(t *testing.T) {
	// Nothing configured means nothing to do, and in particular no git call
	// against a path that need not even be a repository.
	job := syncJob{repoPath: filepath.Join(t.TempDir(), "absent"), repo: lazypath.GitRepo{Name: "repo"}}

	if err := ensureRemotes(job, false); err != nil {
		t.Errorf("ensureRemotes() with no remotes = %v, want nil", err)
	}
}

func TestEnsureWorktrees(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")
	initRepo(t, repoPath)

	worktreePath := filepath.Join(tmpDir, "repo-next")
	job := syncJob{
		repoPath: repoPath,
		repo: lazypath.GitRepo{
			Name:      "repo",
			Worktrees: []lazypath.GitWorktree{{Path: "repo-next", Branch: "next"}},
		},
		worktreePaths: []string{worktreePath},
	}

	if err := ensureWorktrees(job, false); err != nil {
		t.Fatalf("ensureWorktrees() error = %v", err)
	}

	if _, err := os.Stat(worktreePath); err != nil {
		t.Fatalf("worktree was not created: %v", err)
	}

	// The branch has to be the configured one, not one git invented from the
	// directory name.
	branch, err := gitOutputForTest(worktreePath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("cannot read worktree branch: %v", err)
	}
	if branch != "next" {
		t.Errorf("worktree is on %q, want next", branch)
	}

	// Repeating must not fail on the worktree that is already there.
	if err := ensureWorktrees(job, false); err != nil {
		t.Errorf("second ensureWorktrees() error = %v", err)
	}
}

func TestEnsureWorktrees_ExistingBranchKeepsHistory(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")
	initRepo(t, repoPath)

	// A branch that already exists must be checked out, not recreated: -b on an
	// existing branch fails outright.
	mustGit(t, repoPath, "branch", "existing")

	worktreePath := filepath.Join(tmpDir, "repo-existing")
	job := syncJob{
		repoPath: repoPath,
		repo: lazypath.GitRepo{
			Name:      "repo",
			Worktrees: []lazypath.GitWorktree{{Path: "repo-existing", Branch: "existing"}},
		},
		worktreePaths: []string{worktreePath},
	}

	if err := ensureWorktrees(job, false); err != nil {
		t.Fatalf("ensureWorktrees() error = %v", err)
	}

	branch, err := gitOutputForTest(worktreePath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("cannot read worktree branch: %v", err)
	}
	if branch != "existing" {
		t.Errorf("worktree is on %q, want existing", branch)
	}
}

func TestEnsureWorktrees_DryRunChangesNothing(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")
	initRepo(t, repoPath)

	worktreePath := filepath.Join(tmpDir, "repo-next")
	job := syncJob{
		repoPath: repoPath,
		repo: lazypath.GitRepo{
			Name:      "repo",
			Worktrees: []lazypath.GitWorktree{{Path: "repo-next", Branch: "next"}},
		},
		worktreePaths: []string{worktreePath},
	}

	if err := ensureWorktrees(job, true); err != nil {
		t.Fatalf("ensureWorktrees(dryRun) error = %v", err)
	}
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Error("a dry run created the worktree")
	}
}

func TestEnsureWorktrees_BranchlessIsSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")
	initRepo(t, repoPath)

	worktreePath := filepath.Join(tmpDir, "repo-next")
	job := syncJob{
		repoPath: repoPath,
		repo: lazypath.GitRepo{
			Name:      "repo",
			Worktrees: []lazypath.GitWorktree{{Path: "repo-next"}},
		},
		worktreePaths: []string{worktreePath},
	}

	// Skipped with a warning rather than failing the whole sync: the config
	// check is where that mistake gets reported as an error.
	if err := ensureWorktrees(job, false); err != nil {
		t.Errorf("ensureWorktrees() = %v, want the branchless worktree skipped", err)
	}
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Error("a worktree with no branch was created anyway")
	}
}

func TestWorktreeTargetPath(t *testing.T) {
	folder := lazypath.Folder{Path: "/home/me/work"}

	tests := []struct {
		name     string
		worktree lazypath.GitWorktree
		want     string
	}{
		{
			name:     "relative to the folder, beside the repositories",
			worktree: lazypath.GitWorktree{Path: "api-next"},
			want:     "/home/me/work/api-next",
		},
		{
			name:     "an absolute path is left alone",
			worktree: lazypath.GitWorktree{Path: "/elsewhere/api-next"},
			want:     "/elsewhere/api-next",
		},
		{
			name:     "a nested relative path still resolves under the folder",
			worktree: lazypath.GitWorktree{Path: "trees/api-next"},
			want:     "/home/me/work/trees/api-next",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := worktreeTargetPath(folder, tt.worktree); got != tt.want {
				t.Errorf("worktreeTargetPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGitRemotes(t *testing.T) {
	repoPath := filepath.Join(t.TempDir(), "repo")
	initRepo(t, repoPath)

	mustGit(t, repoPath, "remote", "add", "origin", "https://example.com/origin.git")
	mustGit(t, repoPath, "remote", "add", "upstream", "https://example.com/upstream.git")

	remotes, err := gitRemotes(repoPath)
	if err != nil {
		t.Fatalf("gitRemotes() error = %v", err)
	}

	want := map[string]string{
		"origin":   "https://example.com/origin.git",
		"upstream": "https://example.com/upstream.git",
	}
	if !reflect.DeepEqual(remotes, want) {
		t.Errorf("gitRemotes() = %v, want %v", remotes, want)
	}
}

func TestGitRemotes_NotARepo(t *testing.T) {
	if _, err := gitRemotes(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Error("gitRemotes() on a missing repository returned no error")
	}
}

// gitOutputForTest reads a single-line git result from a repository.
func gitOutputForTest(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
