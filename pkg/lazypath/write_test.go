package lazypath

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

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

func TestSaveConfig_KeepsKeysItDoesNotKnow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `x-defaults: &defaults
  priority: 3
folders:
  - path: /work/api
    <<: *defaults
    color: red
    description: old
`
	got := savedConfig(t, path, content, func() error {
		return SetFolderDescription("/work/api", "")
	})

	for _, want := range []string{"<<: *defaults", "color: red"} {
		if !strings.Contains(got, want) {
			t.Errorf("file lost %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "description") {
		t.Errorf("the cleared description is still there:\n%s", got)
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

// loadedConfig points the package at a configuration file holding content and
// reads it, the way a command does when it starts.
func loadedConfig(t *testing.T, path, content string) {
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
	OwnConfig()
}

func TestSaveConfig_KeepsAChangeMadeAfterLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	loadedConfig(t, path, commentedConfig)

	// Another `projekt folder add`, or the user in an editor, while this
	// process was busy.
	if err := os.WriteFile(path, []byte(commentedConfig+"  - path: /work/other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (&Folder{Path: "/work/new"}).AddToConfig(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := commentedConfig + "  - path: /work/other\n  - path: /work/new\n"; string(data) != want {
		t.Errorf("file:\n%s\nwant both additions:\n%s", data, want)
	}
	if got := len(OwnConfig().Folders); got != 4 {
		t.Errorf("in-memory folders = %d, want 4", got)
	}
}

func TestSaveConfig_FailedSaveLeavesTheConfigAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	loadedConfig(t, path, commentedConfig)
	before := OwnConfig()

	if err := os.WriteFile(path, []byte("folders: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (&Folder{Path: "/work/new"}).AddToConfig(); err == nil {
		t.Fatal("saving over an unreadable file succeeded")
	}
	if err := RemoveFromConfig("/work/api"); err == nil {
		t.Fatal("saving over an unreadable file succeeded")
	}

	if after := OwnConfig(); !reflect.DeepEqual(after, before) {
		t.Errorf("in-memory config changed by failed saves:\n%+v\nwant:\n%+v", after, before)
	}
	if data, _ := os.ReadFile(path); string(data) != "folders: [\n" {
		t.Errorf("file was rewritten:\n%s", data)
	}
}

func TestSaveConfig_WaitsForTheLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	loadedConfig(t, path, commentedConfig)

	unlock, err := lockConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- (&Folder{Path: "/work/new"}).AddToConfig() }()

	select {
	case err := <-done:
		unlock()
		t.Fatalf("saved while another holder had the lock (err %v)", err)
	case <-time.After(300 * time.Millisecond):
	}
	// What the other holder wrote under the lock is what the save starts from.
	if err := os.WriteFile(path, []byte(commentedConfig+"  - path: /work/other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unlock()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("save still waiting after the lock was released")
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "/work/other") || !strings.Contains(string(data), "/work/new") {
		t.Errorf("file lost a change:\n%s", data)
	}
}

// TestSaveConfig_ConcurrentProcesses runs several copies of this test binary
// that each add a folder to the same file at once: every one must be kept.
func TestSaveConfig_ConcurrentProcesses(t *testing.T) {
	if os.Getenv("LAZYPATH_TEST_ADD") != "" {
		CfgFile = os.Getenv("LAZYPATH_TEST_CONFIG")
		InitConfig()
		OwnConfig()
		// Widen the window between load and save.
		time.Sleep(100 * time.Millisecond)
		if err := (&Folder{Path: os.Getenv("LAZYPATH_TEST_ADD")}).AddToConfig(); err != nil {
			t.Fatal(err)
		}
		return
	}

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(commentedConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	const n = 6
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestSaveConfig_ConcurrentProcesses$")
			cmd.Env = append(os.Environ(),
				"LAZYPATH_TEST_CONFIG="+path,
				"LAZYPATH_TEST_ADD=/work/p"+strconv.Itoa(i))
			if out, err := cmd.CombinedOutput(); err != nil {
				errs[i] = fmt.Errorf("%v: %s", err, out)
			}
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := range n {
		if !strings.Contains(string(data), "- path: /work/p"+strconv.Itoa(i)+"\n") {
			t.Errorf("folder /work/p%d was lost:\n%s", i, data)
		}
	}
}

func TestSaveConfig_EnvironmentOverrideSurvivesAWrite(t *testing.T) {
	extra := filepath.Join(t.TempDir(), "extra.yaml")
	t.Setenv("PROJEKT_INCLUDE", extra)
	path := filepath.Join(t.TempDir(), "config.yaml")
	loadedConfig(t, path, "include:\n  - /from/file.yaml\n"+commentedConfig)
	if got := OwnConfig().Include; len(got) != 1 || got[0] != extra {
		t.Fatalf("Include before = %v, want the override", got)
	}

	if err := (&Folder{Path: "/work/new"}).AddToConfig(); err != nil {
		t.Fatal(err)
	}
	if got := OwnConfig().Include; len(got) != 1 || got[0] != extra {
		t.Errorf("Include after a write = %v, want the override still on top", got)
	}
	if data, _ := os.ReadFile(path); strings.Contains(string(data), extra) {
		t.Errorf("the override was written to the file:\n%s", data)
	}
}

func TestSaveConfig_NothingToWriteStillCatchesUp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	loadedConfig(t, path, commentedConfig)

	// Somebody else removed every folder while this process ran.
	if err := os.WriteFile(path, []byte("folders: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = RemoveFromConfig("/work/api")
	if got := OwnConfig().Folders; len(got) != 0 {
		t.Errorf("in-memory folders = %v, want what the file now says", got)
	}
}
