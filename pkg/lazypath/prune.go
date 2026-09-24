package lazypath

import (
	"fmt"
	"os"

	"github.com/spf13/viper"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

// Stale is one configuration entry whose folder is no longer on disk.
type Stale struct {
	// Kind is "folder" or "worktree", which is what it takes to say what will
	// be removed without showing the whole entry.
	Kind string
	// Name is how it was reached: the short name of a worktree, or the last
	// element of a folder's path.
	Name string
	// Path is what is not there any more.
	Path string
	// Tags are the folder's, for a listing that filters by them.
	Tags []string
}

// Kinds of stale entry.
const (
	StaleFolder   = "folder"
	StaleWorktree = "worktree"
)

// FindStale returns the configured folders and working trees whose path is
// gone.
//
// A path that cannot be read for another reason — a permission, a mount that
// is not there right now — is left alone: it is not the same thing as gone,
// and removing it would lose a configuration that is still wanted.
func FindStale() []Stale {
	config := GetConfig()

	var stale []Stale
	for _, folder := range config.Folders {
		if !isGone(folder.Path) {
			continue
		}
		stale = append(stale, Stale{
			Kind: StaleFolder,
			Name: folder.ShortName(),
			Path: folder.Path,
			Tags: folder.GetTags(),
		})
	}
	for _, worktree := range config.Worktrees {
		if !isGone(worktree.Path) {
			continue
		}
		stale = append(stale, Stale{
			Kind: StaleWorktree,
			Name: worktree.ShortName(),
			Path: worktree.Path,
		})
	}

	return stale
}

// isGone reports whether a path is definitely not there.
func isGone(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

// PruneStale removes every entry FindStale reports, in one write.
func PruneStale() ([]Stale, error) {
	if loadErr != nil {
		return nil, loadErr
	}
	unmarshalConfig()

	stale := FindStale()
	if len(stale) == 0 {
		return nil, nil
	}

	gone := make(map[string]struct{}, len(stale))
	for _, entry := range stale {
		gone[cleanPath(entry.Path)] = struct{}{}
	}

	folders := make([]Folder, 0, len(c.Folders))
	for _, folder := range c.Folders {
		if _, drop := gone[cleanPath(folder.Path)]; drop {
			continue
		}
		folders = append(folders, folder)
	}

	worktrees := make([]Worktree, 0, len(c.Worktrees))
	for _, worktree := range c.Worktrees {
		if _, drop := gone[cleanPath(worktree.Path)]; drop {
			continue
		}
		worktrees = append(worktrees, worktree)
	}

	c.Folders = folders
	c.Worktrees = worktrees
	// One write for the whole lot: a prune that failed halfway would leave a
	// configuration nobody asked for.
	viper.Set("folders", c.Folders)
	viper.Set("worktrees", c.Worktrees)
	if err := viper.WriteConfig(); err != nil {
		cli.Error("Failed to write config %v", err)
		return nil, fmt.Errorf("cannot write the configuration: %w", err)
	}
	refreshEffective()

	return stale, nil
}
