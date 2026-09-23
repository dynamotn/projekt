package lazypath

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// prunableConfig writes a configuration with two folders and a worktree that
// are there, and one of each that is not.
func prunableConfig(t *testing.T) (configFile, here string) {
	t.Helper()

	root := t.TempDir()
	here = filepath.Join(root, "here")
	worktree := filepath.Join(here, "wt")
	for _, path := range []string{here, worktree} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	configFile = filepath.Join(root, "config.yaml")
	previous := CfgFile
	CfgFile = configFile
	c = Config{}
	loadErr = nil
	InitConfig()
	t.Cleanup(func() {
		CfgFile = previous
		c = Config{}
		loadErr = nil
	})

	c = Config{
		Folders: []Folder{
			{Path: here, Tags: []string{"work"}},
			{Path: filepath.Join(root, "gone")},
		},
		Worktrees: []Worktree{
			{Project: "here", Name: "wt", Branch: "main", Path: worktree},
			{Project: "here", Name: "vanished", Branch: "main", Path: filepath.Join(here, "vanished")},
		},
	}

	return configFile, here
}

func TestFindStale(t *testing.T) {
	prunableConfig(t)

	stale := FindStale()
	if len(stale) != 2 {
		t.Fatalf("FindStale() = %#v, want the two that are gone", stale)
	}

	kinds := map[string]string{}
	for _, entry := range stale {
		kinds[entry.Kind] = entry.Name
	}
	if kinds[StaleFolder] != "gone" {
		t.Errorf("stale folder = %q, want gone", kinds[StaleFolder])
	}
	if kinds[StaleWorktree] != "here@vanished" {
		t.Errorf("stale worktree = %q, want here@vanished", kinds[StaleWorktree])
	}
}

func TestPruneStale(t *testing.T) {
	configFile, here := prunableConfig(t)

	pruned, err := PruneStale()
	if err != nil {
		t.Fatalf("PruneStale() error = %v", err)
	}
	if len(pruned) != 2 {
		t.Fatalf("PruneStale() = %#v, want two", pruned)
	}

	config := GetConfig()
	if len(config.Folders) != 1 || config.Folders[0].Path != here {
		t.Errorf("folders = %#v, want only the one that is there", config.Folders)
	}
	if len(config.Worktrees) != 1 || config.Worktrees[0].Name != "wt" {
		t.Errorf("worktrees = %#v, want only the one that is there", config.Worktrees)
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(data), "vanished") || strings.Contains(string(data), "/gone") {
		t.Errorf("the config file still has what was pruned: %q", data)
	}
}

func TestPruneStale_NothingToDo(t *testing.T) {
	prunableConfig(t)

	if _, err := PruneStale(); err != nil {
		t.Fatalf("PruneStale() error = %v", err)
	}
	// A second run has nothing left to remove and says so by removing nothing.
	pruned, err := PruneStale()
	if err != nil {
		t.Fatalf("PruneStale() twice error = %v", err)
	}
	if len(pruned) != 0 {
		t.Errorf("PruneStale() = %#v, want nothing left", pruned)
	}
}

func TestFindStale_LeavesWhatIsOnlyUnreachable(t *testing.T) {
	root := t.TempDir()
	unreadable := filepath.Join(root, "locked")
	inside := filepath.Join(unreadable, "project")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Skipf("cannot make a folder unreadable here: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o755) })

	SetTestConfig(Config{Folders: []Folder{{Path: inside}}})
	t.Cleanup(ResetTestConfig)

	if os.Geteuid() == 0 {
		t.Skip("root can read it anyway")
	}
	// Not readable is not the same as not there: removing it would throw away
	// a configuration that is still wanted.
	if stale := FindStale(); len(stale) != 0 {
		t.Errorf("FindStale() = %#v, want an unreadable path left alone", stale)
	}
}

func TestPruneStale_RefusesOnAnUnreadableConfig(t *testing.T) {
	prunableConfig(t)
	loadErr = os.ErrPermission
	t.Cleanup(func() { loadErr = nil })

	if _, err := PruneStale(); err == nil {
		t.Error("PruneStale() on an unreadable config error = nil, want a refusal")
	}
}
