package lazypath

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// savedConfig points the package at a configuration file holding content,
// runs change against it and returns what the file says afterwards.
func savedConfig(t *testing.T, path, content string, change func() error) string {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := CfgFile
	CfgFile = path
	ResetTestConfig()
	InitConfig()
	t.Cleanup(func() {
		CfgFile = previous
		ResetTestConfig()
		viper.Reset()
	})

	if err := change(); err != nil {
		t.Fatalf("change: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

const commentedConfig = `# my projects
gitServers:
  - name: gh # main
    type: github
    ssh: git@github.com
    preferGitSSH: true

folders:
  # the one I work on
  - path: /work/api # line comment
    name: api
    tags: [go, work]
  - path: "/work/web"
    is_workspace: false
`

func TestSaveConfig_AddKeepsTheRestOfTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	got := savedConfig(t, path, commentedConfig, func() error {
		return (&Folder{Path: "/work/new", Tags: []string{"go"}}).AddToConfig()
	})

	want := commentedConfig + "  - path: /work/new\n    tags:\n      - go\n"
	if got != want {
		t.Errorf("file after add:\n%s\nwant:\n%s", got, want)
	}
}

func TestSaveConfig_KeepsKeyCaseAndSkipsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	got := savedConfig(t, path, commentedConfig, func() error {
		return (&Folder{Path: "/work/new"}).AddToConfig()
	})

	for _, want := range []string{"gitServers:", "preferGitSSH: true", "# my projects", "# line comment", "# the one I work on", `"/work/web"`, "is_workspace: false"} {
		if !strings.Contains(got, want) {
			t.Errorf("file lost %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"gitservers", "prefergitssh", `prefix: ""`, `regex: ""`, "priority: 0"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("file gained %q:\n%s", unwanted, got)
		}
	}
}

func TestSaveConfig_RemoveDropsOnlyThatEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	got := savedConfig(t, path, commentedConfig, func() error {
		return RemoveFromConfig("/work/web")
	})

	if strings.Contains(got, "/work/web") {
		t.Errorf("entry still there:\n%s", got)
	}
	if !strings.HasPrefix(got, strings.SplitN(commentedConfig, `  - path: "/work/web"`, 2)[0]) {
		t.Errorf("the rest of the file changed:\n%s", got)
	}
}

func TestSaveConfig_ChangedEntryKeepsItsComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	got := savedConfig(t, path, commentedConfig, func() error {
		if err := SetFolderTags("/work/api", []string{"go"}); err != nil {
			return err
		}
		return MoveFolder("/work/api", "/work/api-v2")
	})

	for _, want := range []string{"# the one I work on", "- path: /work/api-v2 # line comment", "name: api"} {
		if !strings.Contains(got, want) {
			t.Errorf("file lost %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "work]") {
		t.Errorf("the removed tag is still there:\n%s", got)
	}
}

func TestSaveConfig_EmptySectionIsRemoved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := commentedConfig + "worktrees:\n  - project: api\n    name: next\n    branch: next\n    path: /work/api/.worktrees/next\n"
	got := savedConfig(t, path, content, func() error {
		return RemoveWorktreeFromConfig("api", "next")
	})

	if got != commentedConfig {
		t.Errorf("file after removing the last worktree:\n%s\nwant:\n%s", got, commentedConfig)
	}
}

func TestSaveConfig_KeepsTheFileIndentation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "folders:\n    - path: /work/api\n      is_workspace: false\n      prefix: \"\"\n"
	got := savedConfig(t, path, content, func() error {
		return (&Folder{Path: "/work/new"}).AddToConfig()
	})

	want := content + "    - path: /work/new\n"
	if got != want {
		t.Errorf("file:\n%s\nwant (indentation kept):\n%s", got, want)
	}
}

func TestSaveConfig_EmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	got := savedConfig(t, path, "", func() error {
		return (&Folder{Path: "/work/new", Prefix: "w"}).AddToConfig()
	})

	if want := "folders:\n  - path: /work/new\n    prefix: w\n"; got != want {
		t.Errorf("file:\n%s\nwant:\n%s", got, want)
	}
}

func TestSaveConfig_WritesThroughASymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(commentedConfig), 0o640); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "config.yaml")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	previous := CfgFile
	CfgFile = link
	ResetTestConfig()
	InitConfig()
	t.Cleanup(func() {
		CfgFile = previous
		ResetTestConfig()
		viper.Reset()
	})
	if err := (&Folder{Path: "/work/new"}).AddToConfig(); err != nil {
		t.Fatal(err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("the link was replaced by a regular file")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "/work/new") {
		t.Errorf("the target was not written:\n%s", data)
	}
	if stat, _ := os.Stat(target); stat.Mode().Perm() != 0o640 {
		t.Errorf("permissions = %v, want 0640", stat.Mode().Perm())
	}
}

func TestSaveConfig_ReadBackInTheSameProcess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	savedConfig(t, path, commentedConfig, func() error {
		return (&Folder{Path: "/work/new"}).AddToConfig()
	})

	ResetTestConfig()
	var paths []string
	for _, folder := range GetConfig().Folders {
		paths = append(paths, folder.Path)
	}
	if got := strings.Join(paths, ","); got != "/work/api,/work/web,/work/new" {
		t.Errorf("folders read back = %s", got)
	}
}
