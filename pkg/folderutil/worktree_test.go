package folderutil

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// worktreeRepo sets up a workspace with one real repository in it, and points
// the configuration at a file a test may write to.
func worktreeRepo(t *testing.T) (workspace, repoPath string) {
	t.Helper()

	workspace = t.TempDir()
	repoPath = filepath.Join(workspace, "mytool")
	initRepo(t, repoPath)

	configFile := filepath.Join(t.TempDir(), "config.yaml")
	previous := lazypath.CfgFile
	lazypath.CfgFile = configFile
	lazypath.ResetTestConfig()
	lazypath.InitConfig()
	t.Cleanup(func() {
		lazypath.CfgFile = previous
		lazypath.ResetTestConfig()
	})

	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{
		Path:        workspace,
		Prefix:      "oss",
		IsWorkspace: true,
		Tags:        []string{"oss"},
	}}})

	return workspace, repoPath
}

func TestWorktreeNameFromBranch(t *testing.T) {
	cases := map[string]string{
		"feature/PROJ-123":      "PROJ-123",
		"main":                  "main",
		"release/1.0":           "1.0",
		"user/me/fix things":    "fix-things",
		"feature/a//b":          "b",
		"weird~^:name":          "weird-name",
		"--leading-and-tailing": "leading-and-tailing",
		"///":                   "",
	}

	for branch, want := range cases {
		if got := WorktreeNameFromBranch(branch); got != want {
			t.Errorf("WorktreeNameFromBranch(%q) = %q, want %q", branch, got, want)
		}
	}
}

func TestAddWorktree(t *testing.T) {
	_, repoPath := worktreeRepo(t)
	var out bytes.Buffer

	worktree, err := AddWorktree(&out, AddWorktreeOptions{
		Project: "oss-mytool",
		Branch:  "feature/PROJ-123",
	})
	if err != nil {
		t.Fatalf("AddWorktree() error = %v", err)
	}

	want := filepath.Join(repoPath, WorktreeFolder, "PROJ-123")
	if worktree.Path != want {
		t.Errorf("AddWorktree().Path = %v, want %v", worktree.Path, want)
	}
	if worktree.ShortName() != "oss-mytool@PROJ-123" {
		t.Errorf("ShortName() = %v", worktree.ShortName())
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("the worktree was not created: %v", err)
	}

	// The branch is created when it does not exist, and checked out there.
	branch, err := gitOutputForTest(want, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse error = %v", err)
	}
	if branch != "feature/PROJ-123" {
		t.Errorf("checked out %q, want feature/PROJ-123", branch)
	}

	// And it resolves as a project, which is the whole point.
	parsed, err := ParseConfig(lazypath.GetConfig())
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	var found bool
	for _, folder := range parsed {
		if folder.ShortName != "oss-mytool@PROJ-123" {
			continue
		}
		found = true
		if folder.Path != want {
			t.Errorf("resolved path = %v, want %v", folder.Path, want)
		}
		// It carries the project's tags, so --tags reaches it too.
		if len(folder.Tags) != 1 || folder.Tags[0] != "oss" {
			t.Errorf("tags = %v, want the project's", folder.Tags)
		}
	}
	if !found {
		t.Error("the worktree does not resolve as a project")
	}
}

func TestAddWorktree_ExistingBranchKeepsItsHistory(t *testing.T) {
	_, repoPath := worktreeRepo(t)
	mustGit(t, repoPath, "branch", "already-here")

	var out bytes.Buffer
	worktree, err := AddWorktree(&out, AddWorktreeOptions{Project: "oss-mytool", Branch: "already-here"})
	if err != nil {
		t.Fatalf("AddWorktree() error = %v", err)
	}

	branch, err := gitOutputForTest(worktree.Path, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse error = %v", err)
	}
	if branch != "already-here" {
		t.Errorf("checked out %q, want already-here", branch)
	}
}

func TestAddWorktree_NameAndPath(t *testing.T) {
	workspace, _ := worktreeRepo(t)
	var out bytes.Buffer

	worktree, err := AddWorktree(&out, AddWorktreeOptions{
		Project: "oss-mytool",
		Branch:  "release/1.0",
		Name:    "trunk",
		Path:    filepath.Join(workspace, "elsewhere", "mytool-trunk"),
	})
	if err != nil {
		t.Fatalf("AddWorktree() error = %v", err)
	}
	if worktree.Name != "trunk" {
		t.Errorf("Name = %v, want trunk", worktree.Name)
	}
	if worktree.Path != filepath.Join(workspace, "elsewhere", "mytool-trunk") {
		t.Errorf("Path = %v", worktree.Path)
	}
}

func TestAddWorktree_DryRunTouchesNothing(t *testing.T) {
	_, _ = worktreeRepo(t)
	var out bytes.Buffer

	worktree, err := AddWorktree(&out, AddWorktreeOptions{
		Project: "oss-mytool",
		Branch:  "feature/PROJ-123",
		DryRun:  true,
	})
	if err != nil {
		t.Fatalf("AddWorktree() error = %v", err)
	}
	if _, err := os.Stat(worktree.Path); !os.IsNotExist(err) {
		t.Error("the dry run created the worktree")
	}
	if len(lazypath.GetConfig().Worktrees) != 0 {
		t.Error("the dry run wrote to the configuration")
	}
	if !strings.Contains(out.String(), "DRY RUN") {
		t.Errorf("output = %q, want the plan", out.String())
	}
}

func TestAddWorktree_Errors(t *testing.T) {
	workspace, _ := worktreeRepo(t)
	var out bytes.Buffer

	if _, err := AddWorktree(&out, AddWorktreeOptions{Project: "nope", Branch: "x"}); err == nil {
		t.Error("AddWorktree() with an unknown project error = nil, want an error")
	}
	if _, err := AddWorktree(&out, AddWorktreeOptions{Project: "oss-mytool", Branch: " "}); err == nil {
		t.Error("AddWorktree() with no branch error = nil, want an error")
	}
	// A branch whose last element is only punctuation cannot name a folder.
	if _, err := AddWorktree(&out, AddWorktreeOptions{Project: "oss-mytool", Branch: "///"}); err == nil {
		t.Error("AddWorktree() with an unusable branch name error = nil, want an error")
	}

	// A folder that is not a repository has nothing to hang a worktree off.
	plain := filepath.Join(workspace, "not-a-repo")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := AddWorktree(&out, AddWorktreeOptions{Project: "oss-not-a-repo", Branch: "x"}); err == nil {
		t.Error("AddWorktree() on a plain folder error = nil, want an error")
	}

	// Twice is refused, and so is a worktree of a worktree.
	if _, err := AddWorktree(&out, AddWorktreeOptions{Project: "oss-mytool", Branch: "feature/x"}); err != nil {
		t.Fatalf("AddWorktree() error = %v", err)
	}
	if _, err := AddWorktree(&out, AddWorktreeOptions{Project: "oss-mytool", Branch: "feature/x"}); err == nil {
		t.Error("AddWorktree() twice error = nil, want an error")
	}
	if _, err := AddWorktree(&out, AddWorktreeOptions{Project: "oss-mytool@x", Branch: "other"}); err == nil {
		t.Error("AddWorktree() on a worktree error = nil, want an error")
	}
}

func TestRemoveWorktree(t *testing.T) {
	_, _ = worktreeRepo(t)
	var out bytes.Buffer

	worktree, err := AddWorktree(&out, AddWorktreeOptions{Project: "oss-mytool", Branch: "feature/PROJ-123"})
	if err != nil {
		t.Fatalf("AddWorktree() error = %v", err)
	}

	if err := RemoveWorktree(&out, RemoveWorktreeOptions{Project: "oss-mytool", Name: "PROJ-123"}); err != nil {
		t.Fatalf("RemoveWorktree() error = %v", err)
	}
	if _, err := os.Stat(worktree.Path); !os.IsNotExist(err) {
		t.Error("RemoveWorktree() left the folder behind")
	}
	if len(lazypath.GetConfig().Worktrees) != 0 {
		t.Error("RemoveWorktree() left the entry behind")
	}
	// The branch is the work; removing the folder is not removing it.
	if !branchExists(filepath.Dir(filepath.Dir(worktree.Path)), "feature/PROJ-123") {
		t.Error("RemoveWorktree() deleted the branch")
	}
}

func TestRemoveWorktree_Keep(t *testing.T) {
	_, _ = worktreeRepo(t)
	var out bytes.Buffer

	worktree, err := AddWorktree(&out, AddWorktreeOptions{Project: "oss-mytool", Branch: "feature/PROJ-123"})
	if err != nil {
		t.Fatalf("AddWorktree() error = %v", err)
	}

	if err := RemoveWorktree(&out, RemoveWorktreeOptions{Project: "oss-mytool", Name: "PROJ-123", Keep: true}); err != nil {
		t.Fatalf("RemoveWorktree() error = %v", err)
	}
	if _, err := os.Stat(worktree.Path); err != nil {
		t.Error("RemoveWorktree() with Keep removed the folder")
	}
	if len(lazypath.GetConfig().Worktrees) != 0 {
		t.Error("RemoveWorktree() with Keep left the entry behind")
	}
}

func TestRemoveWorktree_Unknown(t *testing.T) {
	_, _ = worktreeRepo(t)
	var out bytes.Buffer

	if err := RemoveWorktree(&out, RemoveWorktreeOptions{Project: "oss-mytool", Name: "nope"}); err == nil {
		t.Error("RemoveWorktree() of an unknown worktree error = nil, want an error")
	}
}

func TestListWorktrees_Status(t *testing.T) {
	_, _ = worktreeRepo(t)
	var out bytes.Buffer

	worktree, err := AddWorktree(&out, AddWorktreeOptions{Project: "oss-mytool", Branch: "feature/PROJ-123"})
	if err != nil {
		t.Fatalf("AddWorktree() error = %v", err)
	}

	out.Reset()
	o := &ListWorktreeOption{}
	o.Output = "tsv"
	o.NoHeaders = true
	if err := ListWorktrees(&out, o); err != nil {
		t.Fatalf("ListWorktrees() error = %v", err)
	}
	if !strings.Contains(out.String(), worktreeOK) {
		t.Errorf("listing = %q, want it ok", out.String())
	}

	// Remove the folder behind its back: the listing has to say so.
	if err := os.RemoveAll(worktree.Path); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := ListWorktrees(&out, o); err != nil {
		t.Fatalf("ListWorktrees() error = %v", err)
	}
	if !strings.Contains(out.String(), worktreeMissing) {
		t.Errorf("listing = %q, want it missing", out.String())
	}
}

func TestListWorktrees_FiltersAndFormats(t *testing.T) {
	_, _ = worktreeRepo(t)
	var out bytes.Buffer

	if _, err := AddWorktree(&out, AddWorktreeOptions{Project: "oss-mytool", Branch: "feature/PROJ-123"}); err != nil {
		t.Fatalf("AddWorktree() error = %v", err)
	}

	out.Reset()
	o := &ListWorktreeOption{NamesOnly: true, NoStatus: true}
	o.Output = "tsv"
	o.NoHeaders = true
	if err := ListWorktrees(&out, o); err != nil {
		t.Fatalf("ListWorktrees() error = %v", err)
	}
	if strings.TrimSpace(out.String()) != "oss-mytool@PROJ-123" {
		t.Errorf("names only = %q", out.String())
	}

	// A project filter that matches nothing lists nothing.
	out.Reset()
	other := &ListWorktreeOption{Project: "somewhere-else"}
	other.Output = "json"
	if err := ListWorktrees(&out, other); err != nil {
		t.Fatalf("ListWorktrees() error = %v", err)
	}
	if strings.TrimSpace(out.String()) != "[]" {
		t.Errorf("filtered json = %q, want []", out.String())
	}

	// The project's tags reach its worktrees.
	out.Reset()
	tagged := &ListWorktreeOption{Tags: []string{"oss"}, NoStatus: true, NamesOnly: true}
	tagged.Output = "tsv"
	tagged.NoHeaders = true
	if err := ListWorktrees(&out, tagged); err != nil {
		t.Fatalf("ListWorktrees() error = %v", err)
	}
	if !strings.Contains(out.String(), "PROJ-123") {
		t.Errorf("tagged listing = %q", out.String())
	}
}

func TestDiagnoseWorktrees(t *testing.T) {
	workspace, _ := worktreeRepo(t)

	config := lazypath.Config{
		Folders: []lazypath.Folder{{Path: workspace, Prefix: "oss", IsWorkspace: true}},
		Worktrees: []lazypath.Worktree{
			{Project: "oss-mytool", Name: "ok", Branch: "main", Path: filepath.Join(workspace, "a")},
			{Project: "renamed-away", Name: "x", Branch: "main", Path: filepath.Join(workspace, "b")},
		},
	}

	diags := DiagnoseWorktrees(config)
	if len(diags) != 1 {
		t.Fatalf("DiagnoseWorktrees() = %#v, want one problem", diags)
	}
	if diags[0].Severity != lazypath.SeverityError {
		t.Errorf("severity = %v, want an error", diags[0].Severity)
	}
	if !strings.Contains(diags[0].Message, "renamed-away") {
		t.Errorf("message = %q", diags[0].Message)
	}
}

func TestCoveringWorkspace(t *testing.T) {
	workspace, _ := worktreeRepo(t)

	if _, covered := CoveringWorkspace(filepath.Join(workspace, "anything")); !covered {
		t.Error("CoveringWorkspace() = false for a child of a workspace")
	}
	// A dotted folder is skipped by the default regex, which is what keeps
	// .worktrees out of the listing.
	if _, covered := CoveringWorkspace(filepath.Join(workspace, ".worktrees")); covered {
		t.Error("CoveringWorkspace() = true for a dotted child")
	}
	// Not a direct child.
	if _, covered := CoveringWorkspace(filepath.Join(workspace, "a", "b")); covered {
		t.Error("CoveringWorkspace() = true for a grandchild")
	}
}

func TestFindFolder(t *testing.T) {
	_, repoPath := worktreeRepo(t)

	folder, err := FindFolder("oss-mytool")
	if err != nil {
		t.Fatalf("FindFolder() error = %v", err)
	}
	if folder.Path != repoPath {
		t.Errorf("FindFolder().Path = %v, want %v", folder.Path, repoPath)
	}

	if _, err := FindFolder("nope"); err == nil {
		t.Error("FindFolder() of an unknown name error = nil, want an error")
	}
}
