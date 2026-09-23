package lazypath

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// useConfigFile points the package at a configuration file a test may write.
func useConfigFile(t *testing.T) string {
	t.Helper()

	configFile := filepath.Join(t.TempDir(), "config.yaml")
	previous := CfgFile
	CfgFile = configFile
	c = Config{}
	InitConfig()
	t.Cleanup(func() {
		CfgFile = previous
		c = Config{}
		loadErr = nil
	})
	return configFile
}

func TestWorktreeShortName(t *testing.T) {
	worktree := Worktree{Project: "myapp", Name: "PROJ-123"}

	if got := worktree.ShortName(); got != "myapp@PROJ-123" {
		t.Errorf("ShortName() = %v, want myapp@PROJ-123", got)
	}
}

func TestSplitWorktreeName(t *testing.T) {
	cases := map[string][2]string{
		"myapp@PROJ-123": {"myapp", "PROJ-123"},
		"a@b@c":          {"a", "b@c"},
	}
	for name, want := range cases {
		project, worktree, ok := SplitWorktreeName(name)
		if !ok || project != want[0] || worktree != want[1] {
			t.Errorf("SplitWorktreeName(%q) = %q, %q, %v", name, project, worktree, ok)
		}
	}

	for _, name := range []string{"myapp", "", "@name", "project@"} {
		if _, _, ok := SplitWorktreeName(name); ok {
			t.Errorf("SplitWorktreeName(%q) = ok, want not a worktree name", name)
		}
	}
}

func TestWorktreeValidate(t *testing.T) {
	good := Worktree{Project: "myapp", Name: "PROJ-123", Branch: "feature/x", Path: "/tmp/wt"}
	if err := good.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}

	cases := map[string]Worktree{
		"no project":           {Name: "n", Branch: "b", Path: "/tmp/wt"},
		"no name":              {Project: "p", Branch: "b", Path: "/tmp/wt"},
		"separator in name":    {Project: "p", Name: "a@b", Branch: "b", Path: "/tmp/wt"},
		"path in name":         {Project: "p", Name: "a/b", Branch: "b", Path: "/tmp/wt"},
		"no branch":            {Project: "p", Name: "n", Path: "/tmp/wt"},
		"no path":              {Project: "p", Name: "n", Branch: "b"},
		"path is not absolute": {Project: "p", Name: "n", Branch: "b", Path: "relative"},
	}
	for name, worktree := range cases {
		if err := worktree.Validate(); err == nil {
			t.Errorf("Validate() of %s error = nil, want an error", name)
		}
	}
}

func TestWorktreeAddAndRemove(t *testing.T) {
	configFile := useConfigFile(t)

	worktree := Worktree{Project: "myapp", Name: "PROJ-123", Branch: "feature/x", Path: "/tmp/wt"}
	if err := worktree.AddToConfig(); err != nil {
		t.Fatalf("AddToConfig() error = %v", err)
	}

	if got := len(GetConfig().Worktrees); got != 1 {
		t.Fatalf("config has %d worktrees, want 1", got)
	}
	if found, _, ok := FindWorktree("myapp", "PROJ-123"); !ok || found.Branch != "feature/x" {
		t.Errorf("FindWorktree() = %#v, %v", found, ok)
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("reading the config: %v", err)
	}
	if !strings.Contains(string(data), "PROJ-123") {
		t.Errorf("the config file does not mention the worktree: %q", data)
	}

	// The same one twice is refused rather than silently duplicated.
	if err := worktree.AddToConfig(); err == nil {
		t.Error("AddToConfig() twice error = nil, want an error")
	}

	if err := RemoveWorktreeFromConfig("myapp", "PROJ-123"); err != nil {
		t.Fatalf("RemoveWorktreeFromConfig() error = %v", err)
	}
	if got := len(GetConfig().Worktrees); got != 0 {
		t.Errorf("config has %d worktrees, want none", got)
	}
	if err := RemoveWorktreeFromConfig("myapp", "PROJ-123"); err == nil {
		t.Error("RemoveWorktreeFromConfig() of an unknown worktree error = nil, want an error")
	}
}

func TestDiagnose_Worktrees(t *testing.T) {
	existing := t.TempDir()

	config := Config{Worktrees: []Worktree{
		{Project: "myapp", Name: "ok", Branch: "main", Path: existing},
		{Project: "myapp", Name: "ok", Branch: "main", Path: existing},
		{Project: "myapp", Name: "gone", Branch: "main", Path: filepath.Join(existing, "not-here")},
		{Project: "", Name: "broken", Branch: "main", Path: existing},
	}}

	var errors, warnings []string
	for _, diag := range config.Diagnose() {
		if diag.Severity == SeverityError {
			errors = append(errors, diag.Message)
			continue
		}
		warnings = append(warnings, diag.Message)
	}

	joined := strings.Join(errors, "\n")
	for _, want := range []string{"more than once", "has no project"} {
		if !strings.Contains(joined, want) {
			t.Errorf("errors = %q, want one about %q", joined, want)
		}
	}
	if !strings.Contains(strings.Join(warnings, "\n"), "does not exist on disk") {
		t.Errorf("warnings = %q, want one about the missing folder", warnings)
	}
}
