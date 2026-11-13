package folderutil

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/spf13/viper"

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

func TestProjectAt_BrokenConfig(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(file, []byte("folders: [ this is not: valid: yaml\n"), 0o644); err != nil {
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
	if lazypath.LoadError() == nil {
		t.Fatal("LoadError() = nil, want the broken file reported")
	}

	if got, ok := ProjectAt(t.TempDir()); ok {
		t.Errorf("ProjectAt() = %+v, true, want false", got)
	}
}
