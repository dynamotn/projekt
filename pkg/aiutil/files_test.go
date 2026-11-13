package aiutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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
