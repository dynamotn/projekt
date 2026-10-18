package folderutil

import (
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// pullable sets up an origin, a clone that will be pulled and a second clone
// that pushes a commit the first one does not have yet.
func pullable(t *testing.T) (clone, other string) {
	t.Helper()

	root := t.TempDir()
	seed := filepath.Join(root, "seed")
	initRepo(t, seed)
	pushable(t, seed)
	origin := seed + ".git"

	clone = filepath.Join(root, "clone")
	mustGit(t, root, "clone", "--quiet", "--", origin, clone)
	other = filepath.Join(root, "other")
	mustGit(t, root, "clone", "--quiet", "--", origin, other)
	for _, repo := range []string{clone, other} {
		mustGit(t, repo, "config", "user.email", "test@example.com")
		mustGit(t, repo, "config", "user.name", "Test")
	}

	return clone, other
}

// pushCommit adds a commit to a clone and pushes it, so another clone falls
// behind by one.
func pushCommit(t *testing.T, repo, file string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(repo, file), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "add", file)
	mustGit(t, repo, "commit", "--quiet", "-m", "add "+file)
	mustGit(t, repo, "push", "--quiet")
}

func head(t *testing.T, repo string) string {
	t.Helper()

	out, err := gitOutput(repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestPullRepo_FastForwards(t *testing.T) {
	clone, other := pullable(t)
	pushCommit(t, other, "new")

	result, err := pullRepo(clone, false)
	if err != nil {
		t.Fatalf("pullRepo() error = %v", err)
	}
	if result.Outcome != pullUpdated {
		t.Errorf("Outcome = %q, want %q", result.Outcome, pullUpdated)
	}
	if result.Commits != 1 {
		t.Errorf("Commits = %d, want 1", result.Commits)
	}
	if head(t, clone) != head(t, other) {
		t.Error("clone was not fast-forwarded to the pushed commit")
	}
}

func TestPullRepo_UpToDate(t *testing.T) {
	clone, _ := pullable(t)

	result, err := pullRepo(clone, false)
	if err != nil {
		t.Fatalf("pullRepo() error = %v", err)
	}
	if result.Outcome != pullUpToDate {
		t.Errorf("Outcome = %q, want %q", result.Outcome, pullUpToDate)
	}
}

func TestPullRepo_SkipsDirtyTree(t *testing.T) {
	clone, other := pullable(t)
	pushCommit(t, other, "new")
	if err := os.WriteFile(filepath.Join(clone, "README"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := head(t, clone)

	result, err := pullRepo(clone, false)
	if err != nil {
		t.Fatalf("pullRepo() error = %v", err)
	}
	if result.Outcome != pullSkipped {
		t.Errorf("Outcome = %q, want %q", result.Outcome, pullSkipped)
	}
	if result.Reason != "uncommitted changes" {
		t.Errorf("Reason = %q, want %q", result.Reason, "uncommitted changes")
	}
	if head(t, clone) != before {
		t.Error("a dirty clone must not move")
	}
	// The fetch still happened, so the branch knows it is behind.
	if out, _ := gitOutput(clone, "rev-list", "--count", "HEAD..@{u}"); out != "1\n" {
		t.Errorf("behind after fetch = %q, want 1", out)
	}
}

func TestPullRepo_SkipsDetachedHead(t *testing.T) {
	clone, other := pullable(t)
	pushCommit(t, other, "new")
	mustGit(t, clone, "checkout", "--quiet", "--detach")

	result, err := pullRepo(clone, false)
	if err != nil {
		t.Fatalf("pullRepo() error = %v", err)
	}
	if result.Outcome != pullSkipped || result.Reason != "detached HEAD" {
		t.Errorf("got %+v, want skipped for a detached HEAD", result)
	}
}

func TestPullRepo_SkipsNoUpstream(t *testing.T) {
	clone, _ := pullable(t)
	mustGit(t, clone, "checkout", "--quiet", "-b", "local-only")

	result, err := pullRepo(clone, false)
	if err != nil {
		t.Fatalf("pullRepo() error = %v", err)
	}
	if result.Outcome != pullSkipped || result.Reason != "no upstream" {
		t.Errorf("got %+v, want skipped for no upstream", result)
	}
}

func TestPullRepo_SkipsDiverged(t *testing.T) {
	clone, other := pullable(t)
	pushCommit(t, other, "theirs")
	if err := os.WriteFile(filepath.Join(clone, "mine"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, clone, "add", "mine")
	mustGit(t, clone, "commit", "--quiet", "-m", "mine")
	before := head(t, clone)

	result, err := pullRepo(clone, false)
	if err != nil {
		t.Fatalf("pullRepo() error = %v", err)
	}
	if result.Outcome != pullSkipped || result.Reason != "diverged" {
		t.Errorf("got %+v, want skipped for a diverged branch", result)
	}
	if head(t, clone) != before {
		t.Error("a diverged clone must not move")
	}
}

func TestPullRepo_AheadOnlyIsUpToDate(t *testing.T) {
	clone, _ := pullable(t)
	if err := os.WriteFile(filepath.Join(clone, "mine"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, clone, "add", "mine")
	mustGit(t, clone, "commit", "--quiet", "-m", "mine")

	result, err := pullRepo(clone, false)
	if err != nil {
		t.Fatalf("pullRepo() error = %v", err)
	}
	if result.Outcome != pullUpToDate {
		t.Errorf("Outcome = %q, want %q: nothing to pull when only ahead", result.Outcome, pullUpToDate)
	}
}

func TestPullRepo_FetchFailureIsAnError(t *testing.T) {
	clone, _ := pullable(t)
	mustGit(t, clone, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "gone.git"))

	if _, err := pullRepo(clone, true); err == nil {
		t.Error("pullRepo() with an unreachable remote must fail")
	}
}

func TestSyncRepo_PullCoversRepoAndWorktrees(t *testing.T) {
	clone, other := pullable(t)
	mustGit(t, other, "checkout", "--quiet", "-b", "next")
	mustGit(t, other, "push", "--quiet", "--set-upstream", "origin", "next")
	worktree := filepath.Join(filepath.Dir(clone), "clone-next")
	mustGit(t, clone, "fetch", "--quiet")
	mustGit(t, clone, "worktree", "add", "--quiet", "--track", "-b", "next", worktree, "origin/next")

	pushCommit(t, other, "on-next")
	mustGit(t, other, "checkout", "--quiet", "main")
	pushCommit(t, other, "on-main")

	job := syncJob{
		repo: lazypath.GitRepo{Name: "clone", Worktrees: []lazypath.GitWorktree{
			{Path: "clone-next", Branch: "next"},
		}},
		repoPath:      clone,
		worktreePaths: []string{worktree},
		pull:          true,
	}
	if err := syncRepo(job, false); err != nil {
		t.Fatalf("syncRepo() error = %v", err)
	}
	if out, _ := gitOutput(clone, "rev-list", "--count", "HEAD..@{u}"); out != "0\n" {
		t.Errorf("main clone still behind by %q", out)
	}
	if out, _ := gitOutput(worktree, "rev-list", "--count", "HEAD..@{u}"); out != "0\n" {
		t.Errorf("worktree still behind by %q", out)
	}
}

func TestSyncRepo_WithoutPullLeavesClonesAlone(t *testing.T) {
	clone, other := pullable(t)
	pushCommit(t, other, "new")
	before := head(t, clone)

	job := syncJob{repo: lazypath.GitRepo{Name: "clone"}, repoPath: clone}
	if err := syncRepo(job, false); err != nil {
		t.Fatalf("syncRepo() error = %v", err)
	}
	if head(t, clone) != before {
		t.Error("sync without --pull must not move an existing clone")
	}
}
