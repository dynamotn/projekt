package folderutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseRepoRef(t *testing.T) {
	cases := map[string]RepoRef{
		"github:me/starter":              {Host: "github", Group: "me", Name: "starter"},
		"github:me/starter.git":          {Host: "github", Group: "me", Name: "starter"},
		"git@github.com:me/starter.git":  {URL: "git@github.com:me/starter.git"},
		"https://github.com/me/starter":  {URL: "https://github.com/me/starter"},
		"ssh://git@host:2222/me/starter": {URL: "ssh://git@host:2222/me/starter"},
		// A repository on this machine is somewhere git can clone from too.
		"/srv/starters/go.git": {URL: "/srv/starters/go.git"},
		"~/starters/go":        {URL: "~/starters/go"},
		"./starters/go":        {URL: "./starters/go"},
	}
	for input, want := range cases {
		got, err := ParseRepoRef(input)
		if err != nil {
			t.Errorf("ParseRepoRef(%q) error = %v", input, err)
			continue
		}
		if got != want {
			t.Errorf("ParseRepoRef(%q) = %#v, want %#v", input, got, want)
		}
	}

	for _, bad := range []string{"", "   ", "github:justaname", ":group/name", "github:/name", "github:group/"} {
		if _, err := ParseRepoRef(bad); err == nil {
			t.Errorf("ParseRepoRef(%q) error = nil, want an error", bad)
		}
	}
}

func TestIsRepoRoot(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git to make a repository with")
	}

	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main", ".")

	nested := filepath.Join(repo, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	if !IsRepoRoot(repo) {
		t.Error("IsRepoRoot(repo) = false, want the repository itself recognised")
	}
	if IsRepoRoot(nested) {
		t.Error("IsRepoRoot(nested) = true, want a folder inside a repository to be told apart from one")
	}
	if !IsGitRepo(nested) {
		t.Error("IsGitRepo(nested) = false, want it still to be inside a repository")
	}
	if IsRepoRoot(t.TempDir()) {
		t.Error("IsRepoRoot(plain folder) = true")
	}
}
