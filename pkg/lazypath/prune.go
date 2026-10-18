package lazypath

import (
	"fmt"
	"os"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
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

// PruneStale removes every entry FindStale reports from this machine's own
// configuration file, in one write, and returns the ones it removed.
//
// A stale entry that comes from an included file is not this file's to
// change: it is left where it is and left out of what is reported, so that
// what the command says was forgotten is what actually was.
func PruneStale() ([]Stale, error) {
	if loadErr != nil {
		return nil, loadErr
	}
	unmarshalConfig()

	stale := FindStale()
	if len(stale) == 0 {
		return nil, nil
	}

	gone := make(map[string]Stale, len(stale))
	for _, entry := range stale {
		gone[Canonical(entry.Path)] = entry
	}

	var removed []Stale
	folders := make([]Folder, 0, len(c.Folders))
	for _, folder := range c.Folders {
		if entry, drop := gone[Canonical(folder.Path)]; drop {
			removed = append(removed, entry)
			continue
		}
		folders = append(folders, folder)
	}

	worktrees := make([]Worktree, 0, len(c.Worktrees))
	for _, worktree := range c.Worktrees {
		if entry, drop := gone[Canonical(worktree.Path)]; drop {
			removed = append(removed, entry)
			continue
		}
		worktrees = append(worktrees, worktree)
	}

	if len(removed) == 0 {
		return nil, nil
	}
	c.Folders = folders
	c.Worktrees = worktrees
	// One write for the whole lot: a prune that failed halfway would leave a
	// configuration nobody asked for.
	if err := saveConfig("folders", "worktrees"); err != nil {
		cli.Error("Failed to write config %v", err)
		return nil, fmt.Errorf("cannot write the configuration: %w", err)
	}
	refreshEffective()

	return removed, nil
}
