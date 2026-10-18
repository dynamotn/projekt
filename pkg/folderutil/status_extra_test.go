package folderutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

func gitCommand(repo string, args ...string) *exec.Cmd {
	return exec.Command("git", append([]string{"-C", repo}, args...)...)
}

func TestParseSince(t *testing.T) {
	cases := map[string]time.Duration{
		"":    0,
		"12h": 12 * time.Hour,
		"7d":  7 * 24 * time.Hour,
		"2w":  14 * 24 * time.Hour,
		"3mo": 90 * 24 * time.Hour,
		"1y":  365 * 24 * time.Hour,
		"90m": 90 * time.Minute,
	}
	for input, want := range cases {
		got, err := ParseSince(input)
		if err != nil || got != want {
			t.Errorf("ParseSince(%q) = %v, %v; want %v", input, got, err, want)
		}
	}
	for _, bad := range []string{"soon", "-1d", "7"} {
		if _, err := ParseSince(bad); err == nil {
			t.Errorf("ParseSince(%q) error = nil, want one", bad)
		}
	}
}

// commitAt makes a commit dated as given, so that --since has something old
// and something new to tell apart.
func commitAt(t *testing.T, repo, file string, at time.Time) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, file), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "add", file)
	cmd := gitCommand(repo, "commit", "--quiet", "-m", "add "+file)
	stamp := at.Format(time.RFC3339)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+stamp, "GIT_COMMITTER_DATE="+stamp)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
}

func TestStatusFolders_Since(t *testing.T) {
	useHistory(t)
	root := t.TempDir()
	stale := filepath.Join(root, "stale")
	fresh := filepath.Join(root, "fresh")
	visited := filepath.Join(root, "visited")
	for _, repo := range []string{stale, fresh, visited} {
		initRepo(t, repo)
		commitAt(t, repo, "old", time.Now().Add(-60*24*time.Hour))
	}
	commitAt(t, fresh, "new", time.Now().Add(-time.Hour))
	if err := lazypath.RecordVisit("visited", visited); err != nil {
		t.Fatal(err)
	}

	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{Path: stale}, {Path: fresh}, {Path: visited}}})
	t.Cleanup(lazypath.ResetTestConfig)

	rows := statusOf(t, &StatusOptions{Since: 7 * 24 * time.Hour})
	if _, ok := rows["stale"]; ok {
		t.Error("a folder with nothing this week must be left out")
	}
	if _, ok := rows["fresh"]; !ok {
		t.Error("a folder with a commit this week must be kept")
	}
	if _, ok := rows["visited"]; !ok {
		t.Error("a folder jumped to this week must be kept")
	}
}

func TestStatusFolders_Branches(t *testing.T) {
	clone, other := pullable(t)
	pushCommit(t, other, "on-main")
	mustGit(t, clone, "fetch", "--quiet")
	mustGit(t, clone, "checkout", "--quiet", "-b", "feature")
	if err := os.WriteFile(filepath.Join(clone, "mine"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, clone, "add", "mine")
	mustGit(t, clone, "commit", "--quiet", "-m", "mine")

	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{Path: clone}, {Path: other}}})
	t.Cleanup(lazypath.ResetTestConfig)

	rows := statusOf(t, &StatusOptions{Branches: true})
	if rows["clone"]["defaultBranch"] != "main" {
		t.Errorf("defaultBranch = %v, want main", rows["clone"]["defaultBranch"])
	}
	if rows["clone"]["behindDefault"].(float64) != 1 {
		t.Errorf("behindDefault = %v, want 1", rows["clone"]["behindDefault"])
	}
	if rows["clone"]["branchAge"] == "" {
		t.Error("branchAge is empty for a branch that parted from main")
	}
	// On the default branch itself there is nothing to compare.
	if rows["other"]["behindDefault"].(float64) != 0 || rows["other"]["branchAge"] != "" {
		t.Errorf("other = %#v, want nothing behind and no age on main", rows["other"])
	}
}

func TestStatusFolders_Requests(t *testing.T) {
	clone, _ := pullable(t)
	mustGit(t, clone, "remote", "set-url", "origin", "git@github.com:me/clone.git")

	// A stand-in for gh that answers two open pull requests.
	bin := t.TempDir()
	script := "#!/bin/sh\nprintf '[{\"number\":1},{\"number\":2}]\\n'\n"
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	original := os.Getenv("PATH")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+original)

	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{Path: clone}}})
	t.Cleanup(lazypath.ResetTestConfig)

	rows := statusOf(t, &StatusOptions{Requests: true})
	if rows["clone"]["requests"] != "2" {
		t.Errorf("requests = %v, want 2", rows["clone"]["requests"])
	}

	// Without the tool, the column says so rather than failing the listing.
	// Git has to stay reachable, or there is no status to add a column to, so
	// the original PATH comes back — and with it any real gh, which would
	// answer instead.
	t.Setenv("PATH", original)
	if _, err := exec.LookPath("gh"); err == nil {
		t.Skip("a real gh is on PATH")
	}
	rows = statusOf(t, &StatusOptions{Requests: true})
	if rows["clone"]["requests"] != requestsNoTool {
		t.Errorf("requests = %v, want %q without gh", rows["clone"]["requests"], requestsNoTool)
	}
}

func TestRequestTool(t *testing.T) {
	servers := []lazypath.GitServer{{Name: "corp", Type: "gitlab", HTTPS: "https://git.corp.example", SSH: "git@git.corp.example"}}
	cases := map[string]string{
		"git@git.corp.example:team/app.git": "gitlab",
		"https://github.com/me/app.git":     "github",
		"git@gitlab.com:me/app.git":         "gitlab",
		"ssh://git@bitbucket.org/me/app":    "",
	}
	for origin, want := range cases {
		if got := requestTool(origin, servers); got != want {
			t.Errorf("requestTool(%q) = %q, want %q", origin, got, want)
		}
	}
}
