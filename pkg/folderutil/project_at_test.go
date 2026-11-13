package folderutil

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

func useConfig(t *testing.T, folders ...lazypath.Folder) {
	t.Helper()
	lazypath.SetTestConfig(lazypath.Config{Folders: folders})
	t.Cleanup(lazypath.ResetTestConfig)
}

func TestProjectAt_ExistingProject(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "pkg", "x")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	useConfig(t, lazypath.Folder{Path: dir, Name: "api"})

	got, ok := ProjectAt(sub)
	if !ok || got.ShortName != "api" || got.Path != dir {
		t.Errorf("ProjectAt() = %+v, %v, want api at %s", got, ok, dir)
	}
}

func TestProjectAt_WorkspaceChildThatDoesNotExistYet(t *testing.T) {
	ws := t.TempDir()
	useConfig(t, lazypath.Folder{Path: ws, IsWorkspace: true, Prefix: "w", Tags: []string{"go"}, Description: "Work"})

	for _, path := range []string{filepath.Join(ws, "newapp"), filepath.Join(ws, "newapp", "cmd")} {
		got, ok := ProjectAt(path)
		want := ParsedFolder{ShortName: "w-newapp", Path: filepath.Join(ws, "newapp"), Workspace: ws, Tags: []string{"go"}, Description: "Work"}
		if !ok || !reflect.DeepEqual(got, want) {
			t.Errorf("ProjectAt(%s) = %+v, %v, want %+v", path, got, ok, want)
		}
	}
}

func TestProjectAt_WorkspacePatternIsRespected(t *testing.T) {
	ws := t.TempDir()
	useConfig(t, lazypath.Folder{Path: ws, IsWorkspace: true, RegexMatch: "^app-"})

	if got, ok := ProjectAt(filepath.Join(ws, "other")); ok {
		t.Errorf("ProjectAt() = %+v, true, want false", got)
	}
	if _, ok := ProjectAt(filepath.Join(ws, "app-new")); !ok {
		t.Error("ProjectAt() of a name the pattern accepts = false, want true")
	}
}

func TestProjectAt_Nowhere(t *testing.T) {
	ws := t.TempDir()
	useConfig(t, lazypath.Folder{Path: ws, IsWorkspace: true})

	if got, ok := ProjectAt(t.TempDir()); ok {
		t.Errorf("ProjectAt(elsewhere) = %+v, true, want false", got)
	}
	if got, ok := ProjectAt(ws); ok {
		t.Errorf("ProjectAt(workspace) = %+v, true, want false", got)
	}
}

func loadConfigFile(t *testing.T, content string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := lazypath.CfgFile
	lazypath.CfgFile = file
	t.Cleanup(func() {
		lazypath.CfgFile = previous
		lazypath.ResetTestConfig()
		viper.Reset()
	})
	lazypath.InitConfigReadOnly()
}

func TestProjectAt_BrokenConfig(t *testing.T) {
	loadConfigFile(t, "folders: [ this is not: valid: yaml\n")
	if lazypath.LoadError() == nil {
		t.Fatal("LoadError() = nil, want the broken file reported")
	}

	if got, ok := ProjectAt(t.TempDir()); ok {
		t.Errorf("ProjectAt() = %+v, true, want false", got)
	}
}

// A file that parses but does not decode is only noticed on first use.
func TestProjectAt_ConfigThatDoesNotDecode(t *testing.T) {
	loadConfigFile(t, "folders: 5\n")

	if got, ok := ProjectAt(t.TempDir()); ok {
		t.Errorf("ProjectAt() = %+v, true, want false", got)
	}
	if lazypath.LoadError() == nil {
		t.Error("LoadError() = nil after the decode failed, want it recorded")
	}
}

// warnings runs fn with the logger at the level a plain command runs at, and
// returns what it logged.
func warnings(t *testing.T, fn func()) string {
	t.Helper()
	// The logger writes to the stderr it was built with, so rebuild it over
	// a pipe.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	realStderr := os.Stderr
	os.Stderr = w
	// Cleanups run last-registered first: the logger is rebuilt over the
	// real stderr, not over the closed pipe.
	t.Cleanup(cli.InitLogging)
	t.Cleanup(func() { os.Stderr = realStderr })
	cli.GetEnv().LogLevel = "warn"
	t.Cleanup(func() { cli.GetEnv().LogLevel = "" })
	cli.InitLogging()

	fn()
	os.Stderr = realStderr
	_ = w.Close()
	logged, _ := io.ReadAll(r)
	return string(logged)
}

// A workspace that is not on this machine, as with a configuration shared
// through dotfiles.
func missingWorkspaceConfig(t *testing.T) string {
	return "folders:\n  - path: " + filepath.Join(t.TempDir(), "gone") + "\n    is_workspace: true\n"
}

func TestProjectAt_ReadOnlyConfigIsQuiet(t *testing.T) {
	config := missingWorkspaceConfig(t)
	logged := warnings(t, func() {
		loadConfigFile(t, config)
		ProjectAt(t.TempDir())
	})
	if logged != "" {
		t.Errorf("logged at warning level or above: %s", logged)
	}
}

func TestParseConfig_StillWarnsForProjekt(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(file, []byte(missingWorkspaceConfig(t)), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := lazypath.CfgFile
	lazypath.CfgFile = file
	t.Cleanup(func() {
		lazypath.CfgFile = previous
		lazypath.ResetTestConfig()
		viper.Reset()
	})

	logged := warnings(t, func() {
		lazypath.InitConfig()
		_, _ = ParseConfig(lazypath.GetConfig())
	})
	if !strings.Contains(logged, "Cannot read folder") {
		t.Errorf("logged %q, want the unreadable workspace warned about", logged)
	}
}

// A workspace inside a configured project names the workspace's child, also
// before the child exists.
func TestProjectAt_WorkspaceNestedInProject(t *testing.T) {
	outer := t.TempDir()
	ws := filepath.Join(outer, "ws")
	if err := os.Mkdir(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	useConfig(t,
		lazypath.Folder{Path: outer, Name: "outer"},
		lazypath.Folder{Path: ws, IsWorkspace: true, Prefix: "w"},
	)
	newapp := filepath.Join(ws, "newapp")

	if got, ok := ProjectAt(newapp); !ok || got.ShortName != "w-newapp" {
		t.Errorf("ProjectAt() before the folder exists = %+v, %v, want w-newapp", got, ok)
	}
	if err := os.Mkdir(newapp, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, ok := ProjectAt(newapp); !ok || got.ShortName != "w-newapp" {
		t.Errorf("ProjectAt() once the folder exists = %+v, %v, want w-newapp", got, ok)
	}
}
