package folderutil

import (
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

func TestMatchRemoteToServer(t *testing.T) {
	servers := []lazypath.GitServer{
		{Name: "github", HTTPS: "https://github.com", SSH: "git@github.com"},
		{Name: "ported", HTTPS: "https://test.git.dev", SSH: "ssh://git@test.git.dev:2022"},
	}

	tests := []struct {
		name      string
		remote    string
		wantHost  string
		wantGroup string
		wantName  string
		wantOK    bool
	}{
		{
			name:      "scp form",
			remote:    "git@github.com:myorg/backend.git",
			wantHost:  "github",
			wantGroup: "myorg",
			wantName:  "backend",
			wantOK:    true,
		},
		{
			name:      "https form",
			remote:    "https://github.com/myorg/backend.git",
			wantHost:  "github",
			wantGroup: "myorg",
			wantName:  "backend",
			wantOK:    true,
		},
		{
			name:      "without the .git suffix",
			remote:    "https://github.com/myorg/backend",
			wantHost:  "github",
			wantGroup: "myorg",
			wantName:  "backend",
			wantOK:    true,
		},
		{
			name:      "a nested group stays with the group",
			remote:    "git@github.com:myorg/myteam/sub/backend.git",
			wantHost:  "github",
			wantGroup: "myorg/myteam/sub",
			wantName:  "backend",
			wantOK:    true,
		},
		{
			name:      "ssh url with a port",
			remote:    "ssh://git@test.git.dev:2022/GROUP/SUB/api.git",
			wantHost:  "ported",
			wantGroup: "GROUP/SUB",
			wantName:  "api",
			wantOK:    true,
		},
		{
			name:      "scp spelling of a server configured with a port",
			remote:    "git@test.git.dev:GROUP/api.git",
			wantHost:  "ported",
			wantGroup: "GROUP",
			wantName:  "api",
			wantOK:    true,
		},
		{
			name:   "an unknown host matches nothing",
			remote: "git@gitlab.com:myorg/backend.git",
			wantOK: false,
		},
		{
			name:   "a URL with no group is not a repository we can place",
			remote: "https://github.com/backend.git",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, group, name, ok := matchRemoteToServer(servers, tt.remote)
			if ok != tt.wantOK {
				t.Fatalf("matchRemoteToServer(%q) ok = %v, want %v", tt.remote, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if host != tt.wantHost || group != tt.wantGroup || name != tt.wantName {
				t.Errorf("matchRemoteToServer(%q) = (%q, %q, %q), want (%q, %q, %q)",
					tt.remote, host, group, name, tt.wantHost, tt.wantGroup, tt.wantName)
			}
		})
	}
}

func TestMatchRemoteToServer_LongestPrefixWins(t *testing.T) {
	// A server hosted under another one's path must not be shadowed by it.
	servers := []lazypath.GitServer{
		{Name: "outer", HTTPS: "https://example.com"},
		{Name: "inner", HTTPS: "https://example.com/git"},
	}

	host, group, name, ok := matchRemoteToServer(servers, "https://example.com/git/myorg/backend.git")
	if !ok {
		t.Fatal("matchRemoteToServer() found no server")
	}
	if host != "inner" || group != "myorg" || name != "backend" {
		t.Errorf("matched (%q, %q, %q), want (inner, myorg, backend)", host, group, name)
	}
}

// initRepoWithRemote creates a real repository with one origin remote, so that
// discovery is exercised through git rather than a stub.
func initRepoWithRemote(t *testing.T, parent string, dirName string, remote string) {
	t.Helper()

	repoPath := filepath.Join(parent, dirName)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, output)
		}
	}

	run("init", "-q", repoPath)
	if remote != "" {
		run("-C", repoPath, "remote", "add", "origin", remote)
	}
}

func TestDiscoverRepos(t *testing.T) {
	workspace := t.TempDir()

	initRepoWithRemote(t, workspace, "backend", "git@github.com:myorg/backend.git")
	// Checked out under a different name than the repository, which the
	// discovered config has to record as a path.
	initRepoWithRemote(t, workspace, "web", "git@github.com:myorg/frontend.git")
	// Another group: it cannot be represented alongside the majority, so it is
	// reported and skipped rather than silently misfiled.
	initRepoWithRemote(t, workspace, "stray", "git@github.com:other/thing.git")
	// Not a repository at all.
	if err := exec.Command("mkdir", filepath.Join(workspace, "notrepo")).Run(); err != nil {
		t.Fatal(err)
	}

	servers := []lazypath.GitServer{{Name: "github", HTTPS: "https://github.com", SSH: "git@github.com"}}
	folder := lazypath.Folder{Path: workspace, IsWorkspace: true}

	got, err := DiscoverRepos(folder, servers)
	if err != nil {
		t.Fatalf("DiscoverRepos() error = %v", err)
	}

	if got.Host != "github" {
		t.Errorf("Host = %q, want github", got.Host)
	}
	if got.Group != "myorg" {
		t.Errorf("Group = %q, want myorg (the group most repositories share)", got.Group)
	}

	want := []lazypath.GitRepo{
		{Name: "backend"},
		{Name: "frontend", Path: "web"},
	}
	if !reflect.DeepEqual(got.Repos, want) {
		t.Errorf("Repos = %+v, want %+v", got.Repos, want)
	}
}

func TestDiscoverRepos_HonoursWorkspaceRegex(t *testing.T) {
	workspace := t.TempDir()
	initRepoWithRemote(t, workspace, "keep-me", "git@github.com:myorg/keep-me.git")
	initRepoWithRemote(t, workspace, "skip-me", "git@github.com:myorg/skip-me.git")

	servers := []lazypath.GitServer{{Name: "github", SSH: "git@github.com"}}
	folder := lazypath.Folder{Path: workspace, IsWorkspace: true, RegexMatch: "^keep-"}

	got, err := DiscoverRepos(folder, servers)
	if err != nil {
		t.Fatalf("DiscoverRepos() error = %v", err)
	}

	want := []lazypath.GitRepo{{Name: "keep-me"}}
	if !reflect.DeepEqual(got.Repos, want) {
		t.Errorf("Repos = %+v, want %+v", got.Repos, want)
	}
}

func TestDiscoverRepos_Errors(t *testing.T) {
	t.Run("refuses a plain folder", func(t *testing.T) {
		_, err := DiscoverRepos(
			lazypath.Folder{Path: t.TempDir()},
			[]lazypath.GitServer{{Name: "github", SSH: "git@github.com"}},
		)
		if err == nil {
			t.Fatal("DiscoverRepos() on a plain folder returned no error")
		}
	})

	t.Run("refuses without configured servers", func(t *testing.T) {
		_, err := DiscoverRepos(lazypath.Folder{Path: t.TempDir(), IsWorkspace: true}, nil)
		if err == nil {
			t.Fatal("DiscoverRepos() with no servers returned no error")
		}
	})

	t.Run("reports an empty workspace", func(t *testing.T) {
		_, err := DiscoverRepos(
			lazypath.Folder{Path: t.TempDir(), IsWorkspace: true},
			[]lazypath.GitServer{{Name: "github", SSH: "git@github.com"}},
		)
		if err == nil {
			t.Fatal("DiscoverRepos() on an empty workspace returned no error")
		}
	})

	t.Run("skips a repository with no remote", func(t *testing.T) {
		workspace := t.TempDir()
		initRepoWithRemote(t, workspace, "noremote", "")
		initRepoWithRemote(t, workspace, "fine", "git@github.com:myorg/fine.git")

		got, err := DiscoverRepos(
			lazypath.Folder{Path: workspace, IsWorkspace: true},
			[]lazypath.GitServer{{Name: "github", SSH: "git@github.com"}},
		)
		if err != nil {
			t.Fatalf("DiscoverRepos() error = %v", err)
		}
		if want := []lazypath.GitRepo{{Name: "fine"}}; !reflect.DeepEqual(got.Repos, want) {
			t.Errorf("Repos = %+v, want %+v", got.Repos, want)
		}
	})
}
