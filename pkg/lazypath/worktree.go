package lazypath

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/samber/lo"
	"github.com/spf13/viper"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

// WorktreeSeparator joins a project and one of its working trees into the one
// name everything else uses: `myapp@feature`.
const WorktreeSeparator = "@"

// Worktree is an extra working tree of a project, kept so that it can be
// jumped to by name like any other project folder.
//
// It hangs off the project's short name rather than off a folder entry,
// because the project may well be a child of a workspace and have no entry of
// its own.
type Worktree struct {
	// Project is the short name of the project this working tree belongs to,
	// exactly as `pj` knows it, prefix included.
	Project string `yaml:"project" mapstructure:"project"`
	// Name is the part after the "@". It defaults to the last element of the
	// branch when the working tree is created.
	Name string `yaml:"name" mapstructure:"name"`
	// Branch is what is checked out in it.
	Branch string `yaml:"branch" mapstructure:"branch"`
	// Path is where it lives on disk, absolute.
	Path string `yaml:"path" mapstructure:"path"`
}

// ShortName is the name this working tree is reached by.
func (w Worktree) ShortName() string {
	return w.Project + WorktreeSeparator + w.Name
}

// SplitWorktreeName splits `myapp@feature` into its two halves. It reports
// false for a name that is not a working tree's.
func SplitWorktreeName(name string) (project, worktree string, ok bool) {
	project, worktree, ok = strings.Cut(name, WorktreeSeparator)
	if !ok || project == "" || worktree == "" {
		return "", "", false
	}
	return project, worktree, true
}

// FindWorktree returns the configured working tree with this project and name.
func FindWorktree(project, name string) (Worktree, int, bool) {
	unmarshalConfig()

	return lo.FindIndexOf(c.Worktrees, func(w Worktree) bool {
		return w.Project == project && w.Name == name
	})
}

// Validate reports what makes a working tree unusable.
func (w Worktree) Validate() error {
	switch {
	case strings.TrimSpace(w.Project) == "":
		return fmt.Errorf("has no project")
	case strings.TrimSpace(w.Name) == "":
		return fmt.Errorf("has no name")
	case strings.Contains(w.Name, WorktreeSeparator):
		return fmt.Errorf("has a %q in its name, which is what separates it from the project", WorktreeSeparator)
	case strings.ContainsRune(w.Name, filepath.Separator):
		return fmt.Errorf("has a path separator in its name")
	case strings.TrimSpace(w.Branch) == "":
		return fmt.Errorf("has no branch")
	case strings.TrimSpace(w.Path) == "":
		return fmt.Errorf("has no path")
	case !filepath.IsAbs(w.Path):
		// A relative path would resolve differently on every later run.
		return fmt.Errorf("has a path that is not absolute: %s", w.Path)
	}
	return nil
}

// AddToConfig adds the working tree to the configuration file.
func (w *Worktree) AddToConfig() error {
	if loadErr != nil {
		return loadErr
	}
	unmarshalConfig()

	if _, _, found := FindWorktree(w.Project, w.Name); found {
		return fmt.Errorf("%s is already in the configuration", w.ShortName())
	}

	c.Worktrees = append(c.Worktrees, *w)
	viper.Set("worktrees", c.Worktrees)
	if err := viper.WriteConfig(); err != nil {
		cli.Error("Failed to write config %v", err)
		return err
	}

	cli.Info("Added worktree %s to config", w.ShortName())
	return nil
}

// RemoveWorktreeFromConfig drops a working tree from the configuration.
func RemoveWorktreeFromConfig(project, name string) error {
	if loadErr != nil {
		return loadErr
	}
	unmarshalConfig()

	_, index, found := FindWorktree(project, name)
	if !found {
		return fmt.Errorf("worktree %s%s%s is not in the configuration", project, WorktreeSeparator, name)
	}

	c.Worktrees = append(c.Worktrees[:index], c.Worktrees[index+1:]...)
	viper.Set("worktrees", c.Worktrees)
	if err := viper.WriteConfig(); err != nil {
		cli.Error("Failed to write config %v", err)
		return err
	}

	cli.Info("Removed worktree %s%s%s from config", project, WorktreeSeparator, name)
	return nil
}
