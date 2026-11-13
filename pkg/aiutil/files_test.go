package aiutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

// secretNames are the files of project that must never reach a prompt.
var secretNames = []string{
	".env", ".env.local", "key.pem", "id.key", "a.age", "secrets/token",
	".projekt/notes.md", "config.local.yaml", "big.txt", "link", "inner",
	".git/config",
}

// project writes a folder holding README.md next to every kind of file the
// rules forbid. As a repository, config.local.yaml is git-ignored.
func project(t *testing.T, repository bool) string {
	t.Helper()

	dir := t.TempDir()
	if repository {
		git := exec.Command("git", "init", "-q", dir)
		if out, err := git.CombinedOutput(); err != nil {
			t.Fatalf("git init: %v: %s", err, out)
		}
	}
	files := map[string]string{
		"README.md":         "# Project\n",
		".env":              "TOKEN=x\n",
		".env.local":        "TOKEN=x\n",
		"key.pem":           "pem\n",
		"id.key":            "key\n",
		"a.age":             "age\n",
		"secrets/token":     "token\n",
		".projekt/notes.md": "notes\n",
		"config.local.yaml": "password: x\n",
		".gitignore":        "config.local.yaml\n",
		"big.txt":           strings.Repeat("x", 40<<10),
	}
	for name, content := range files {
		write(t, filepath.Join(dir, name), content)
	}
	outside := t.TempDir()
	write(t, filepath.Join(outside, "secret.txt"), "secret\n")
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	// A link that stays inside but lands on a forbidden file.
	if err := os.Symlink(".env", filepath.Join(dir, "inner")); err != nil {
		t.Fatal(err)
	}
	return dir
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func paths(files []File) []string {
	var out []string
	for _, file := range files {
		out = append(out, file.Path)
	}
	return out
}

func TestFiles_LeavesOutWhatMustNotBeSent(t *testing.T) {
	dir := project(t, true)

	files, err := Files(dir, append([]string{"README.md", "../outside", "/etc/passwd", "missing"}, secretNames...)...)
	if err != nil {
		t.Fatal(err)
	}
	if got := paths(files); !slices.Equal(got, []string{"README.md"}) {
		t.Errorf("Files() = %v, want only README.md", got)
	}
}

func TestFiles_QuotesTheStart(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "README.md"), strings.Repeat("r", 10<<10))

	files, err := Files(dir, "README.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Content != strings.Repeat("r", 4<<10) {
		t.Errorf("Files() = %d files, want README.md with its first 4 KiB", len(files))
	}
}

func TestFiles_NotARepository(t *testing.T) {
	dir := project(t, false)

	files, err := Files(dir, append([]string{"README.md"}, secretNames...)...)
	if err != nil {
		t.Fatal(err)
	}
	// Without git nothing says config.local.yaml is ignored: it is an
	// ordinary file then.
	if got := paths(files); !slices.Equal(got, []string{"README.md", "config.local.yaml"}) {
		t.Errorf("Files() = %v, want README.md and config.local.yaml", got)
	}
}

func TestTree_SameRules(t *testing.T) {
	dir := project(t, true)
	write(t, filepath.Join(dir, "src", "main.go"), "package main\n")

	tree, err := Tree(dir, 100)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".gitignore", "README.md", "src/", "src/main.go"}
	if !slices.Equal(tree, want) {
		t.Errorf("Tree() = %v, want %v", tree, want)
	}

	if tree, _ := Tree(dir, 2); !slices.Equal(tree, want[:2]) {
		t.Errorf("Tree(2) = %v, want %v", tree, want[:2])
	}
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
}

// linkedRepository is a repository holding sub/a.md and docs -> sub.
func linkedRepository(t *testing.T, gitignore string) string {
	t.Helper()
	dir := t.TempDir()
	gitInit(t, dir)
	write(t, filepath.Join(dir, "sub", "a.md"), "a\n")
	write(t, filepath.Join(dir, ".gitignore"), gitignore)
	if err := os.Symlink("sub", filepath.Join(dir, "docs")); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestFiles_LinkToAFolderInside(t *testing.T) {
	dir := linkedRepository(t, "")

	files, err := Files(dir, "docs/a.md")
	if err != nil {
		t.Fatal(err)
	}
	if got := paths(files); !slices.Equal(got, []string{"docs/a.md"}) {
		t.Errorf("Files() = %v, want docs/a.md", got)
	}
	tree, err := Tree(dir, 100)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{".gitignore", "docs/", "sub/", "sub/a.md"}; !slices.Equal(tree, want) {
		t.Errorf("Tree() = %v, want %v", tree, want)
	}
}

func TestFiles_LinkIsJudgedByItsTarget(t *testing.T) {
	dir := linkedRepository(t, "sub/\n")

	files, err := Files(dir, "docs/a.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("Files() = %v, want nothing: sub/ is ignored", paths(files))
	}
	tree, err := Tree(dir, 100)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{".gitignore"}; !slices.Equal(tree, want) {
		t.Errorf("Tree() = %v, want %v", tree, want)
	}
}

func TestFiles_ProjectInsideARepository(t *testing.T) {
	repo := t.TempDir()
	gitInit(t, repo)
	write(t, filepath.Join(repo, ".gitignore"), "services/api/config.local.yaml\n")
	project := filepath.Join(repo, "services", "api")
	write(t, filepath.Join(project, "README.md"), "# API\n")
	write(t, filepath.Join(project, "config.local.yaml"), "password: x\n")

	files, err := Files(project, "config.local.yaml", "README.md")
	if err != nil {
		t.Fatal(err)
	}
	if got := paths(files); !slices.Equal(got, []string{"README.md"}) {
		t.Errorf("Files() = %v, want only README.md", got)
	}
	tree, err := Tree(project, 100)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"README.md"}; !slices.Equal(tree, want) {
		t.Errorf("Tree() = %v, want %v", tree, want)
	}
}

func TestFiles_RefusesAFIFO(t *testing.T) {
	dir := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(dir, "README.md"), 0o644); err != nil {
		t.Fatal(err)
	}

	done := make(chan []File, 1)
	go func() {
		files, err := Files(dir, "README.md")
		if err != nil {
			t.Error(err)
		}
		done <- files
	}()
	select {
	case files := <-done:
		if len(files) != 0 {
			t.Errorf("Files() = %v, want the FIFO refused", paths(files))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Files() blocked on a FIFO")
	}
}

func TestTree_SkipsAnUnreadableFolder(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root reads every folder")
	}
	dir := t.TempDir()
	write(t, filepath.Join(dir, "README.md"), "# Project\n")
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	tree, err := Tree(dir, 100)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(tree, "README.md") {
		t.Errorf("Tree() = %v, want README.md", tree)
	}
}
