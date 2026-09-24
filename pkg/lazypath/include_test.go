package lazypath

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// writeConfig writes a configuration file and returns its path.
func writeConfig(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// loadFrom points the package at a configuration file and reads it.
func loadFrom(t *testing.T, path string) {
	t.Helper()

	previous := CfgFile
	CfgFile = path
	ResetTestConfig()
	viper.Reset()
	InitConfig()
	t.Cleanup(func() {
		CfgFile = previous
		ResetTestConfig()
		viper.Reset()
	})
}

func TestInclude_MergesInOrder(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "work.yaml", `
folders:
  - path: /tmp/work-one
  - path: /tmp/work-two
gitServers:
  - name: work-server
    https: https://git.example.com
`)
	main := writeConfig(t, dir, "config.yaml", `
include:
  - work.yaml
folders:
  - path: /tmp/mine
`)
	loadFrom(t, main)

	config := GetConfig()
	if len(config.Folders) != 3 {
		t.Fatalf("folders = %#v, want this file's and the included ones", config.Folders)
	}
	// This file first: it is the one you are looking at.
	if config.Folders[0].Path != "/tmp/mine" {
		t.Errorf("first folder = %v, want the local one", config.Folders[0].Path)
	}
	if len(config.GitServers) != 1 {
		t.Errorf("gitServers = %#v, want the included one", config.GitServers)
	}

	// And what gets written stays this machine's own.
	own := OwnConfig()
	if len(own.Folders) != 1 || own.Folders[0].Path != "/tmp/mine" {
		t.Errorf("OwnConfig() = %#v, want only the local folder", own.Folders)
	}
}

func TestInclude_AddDoesNotCopyIncludedFolders(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "shared.yaml", "folders:\n  - path: /tmp/shared\n")
	main := writeConfig(t, dir, "config.yaml", "include:\n  - shared.yaml\nfolders: []\n")
	loadFrom(t, main)

	folder := &Folder{Path: "/tmp/added"}
	if err := folder.AddToConfig(); err != nil {
		t.Fatalf("AddToConfig() error = %v", err)
	}

	// The written file must not have grown a copy of the included folder:
	// that is the whole reason reading and writing are kept apart.
	data, err := os.ReadFile(main)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "/tmp/shared") {
		t.Errorf("the local config absorbed an included folder:\n%s", data)
	}
	if !strings.Contains(string(data), "/tmp/added") {
		t.Errorf("the local config did not get the new folder:\n%s", data)
	}

	// And a reader sees both.
	config := GetConfig()
	if len(config.Folders) != 2 {
		t.Errorf("folders = %#v, want the added one and the included one", config.Folders)
	}
}

func TestInclude_RelativeToTheFileThatNamesIt(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "nested/deep.yaml", "folders:\n  - path: /tmp/deep\n")
	writeConfig(t, dir, "nested/middle.yaml", "include:\n  - deep.yaml\nfolders:\n  - path: /tmp/middle\n")
	main := writeConfig(t, dir, "config.yaml", "include:\n  - nested/middle.yaml\n")
	loadFrom(t, main)

	var paths []string
	for _, folder := range GetConfig().Folders {
		paths = append(paths, folder.Path)
	}
	// deep.yaml is named relative to middle.yaml, not to config.yaml.
	if len(paths) != 2 || paths[0] != "/tmp/middle" || paths[1] != "/tmp/deep" {
		t.Errorf("folders = %v, want the whole chain in order", paths)
	}
}

func TestInclude_ACycleIsReadOnce(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "other.yaml", "include:\n  - config.yaml\nfolders:\n  - path: /tmp/other\n")
	main := writeConfig(t, dir, "config.yaml", "include:\n  - other.yaml\nfolders:\n  - path: /tmp/mine\n")
	loadFrom(t, main)

	config := GetConfig()
	if len(config.Folders) != 2 {
		t.Errorf("folders = %#v, want each file read once despite the cycle", config.Folders)
	}
}

func TestInclude_TheSameFileTwiceIsReadOnce(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "shared.yaml", "folders:\n  - path: /tmp/shared\n")
	writeConfig(t, dir, "a.yaml", "include:\n  - shared.yaml\n")
	writeConfig(t, dir, "b.yaml", "include:\n  - shared.yaml\n")
	main := writeConfig(t, dir, "config.yaml", "include:\n  - a.yaml\n  - b.yaml\n")
	loadFrom(t, main)

	config := GetConfig()
	if len(config.Folders) != 1 {
		t.Errorf("folders = %#v, want the shared file read once", config.Folders)
	}
}

func TestInclude_MissingOrBrokenIsAWarningNotAStop(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "broken.yaml", "folders: [this is not a list of folders\n")
	main := writeConfig(t, dir, "config.yaml", `
include:
  - not-there.yaml
  - broken.yaml
folders:
  - path: /tmp/mine
`)
	loadFrom(t, main)

	// One unreadable include must not cost the rest of the configuration.
	config := GetConfig()
	if len(config.Folders) != 1 || config.Folders[0].Path != "/tmp/mine" {
		t.Errorf("folders = %#v, want the local one to survive", config.Folders)
	}
	if LoadError() != nil {
		t.Errorf("LoadError() = %v, want a broken include not to stop everything", LoadError())
	}
}

func TestIncludedFiles(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "deep.yaml", "folders: []\n")
	writeConfig(t, dir, "middle.yaml", "include:\n  - deep.yaml\n")
	main := writeConfig(t, dir, "config.yaml", "include:\n  - middle.yaml\n")
	loadFrom(t, main)

	files := IncludedFiles()
	if len(files) != 2 {
		t.Fatalf("IncludedFiles() = %v, want both", files)
	}
	if filepath.Base(files[0]) != "middle.yaml" || filepath.Base(files[1]) != "deep.yaml" {
		t.Errorf("IncludedFiles() = %v, want them in reading order", files)
	}
}

func TestHostnameOverride(t *testing.T) {
	host, err := os.Hostname()
	if err != nil {
		t.Skip("no hostname")
	}
	short := strings.Split(host, ".")[0]

	got := HostnameOverride("/home/me/.config/projekt/config.yaml")
	want := "/home/me/.config/projekt/config." + short + ".yaml"
	if got != want {
		t.Errorf("HostnameOverride() = %v, want %v", got, want)
	}
}

func TestHostnameOverride_IsMergedWhenItExists(t *testing.T) {
	host, err := os.Hostname()
	if err != nil {
		t.Skip("no hostname")
	}
	short := strings.Split(host, ".")[0]

	dir := t.TempDir()
	writeConfig(t, dir, "config."+short+".yaml", "folders:\n  - path: /tmp/this-machine\n")
	main := writeConfig(t, dir, "config.yaml", "folders:\n  - path: /tmp/everywhere\n")
	loadFrom(t, main)

	var paths []string
	for _, folder := range GetConfig().Folders {
		paths = append(paths, folder.Path)
	}
	// One dotfiles repository, and the folders that only exist here.
	if len(paths) != 2 || paths[0] != "/tmp/everywhere" || paths[1] != "/tmp/this-machine" {
		t.Errorf("folders = %v, want the shared ones then this machine's", paths)
	}

	if files := IncludedFiles(); len(files) != 1 || !strings.Contains(files[0], short) {
		t.Errorf("IncludedFiles() = %v, want the hostname file listed", files)
	}
}

func TestIncludePaths(t *testing.T) {
	got := includePaths([]string{"rel.yaml", "/abs.yaml", "  ", "nested/x.yaml"}, "/home/me/.config/projekt/config.yaml")
	want := []string{
		"/home/me/.config/projekt/rel.yaml",
		"/abs.yaml",
		"/home/me/.config/projekt/nested/x.yaml",
	}
	if len(got) != len(want) {
		t.Fatalf("includePaths() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("includePaths()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}
