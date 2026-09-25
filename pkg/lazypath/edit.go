package lazypath

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
)

// FindOwnFolder returns this machine's configuration entry for a path, and
// where it sits in the list.
//
// Only this machine's own entries can be changed: one that comes from an
// include belongs to the file that declares it.
func FindOwnFolder(path string) (Folder, int, bool) {
	unmarshalConfig()

	key := cleanPath(path)
	for i, folder := range c.Folders {
		if cleanPath(folder.Path) == key {
			return folder, i, true
		}
	}
	return Folder{}, 0, false
}

// MoveFolder points a configuration entry at a new path, keeping everything
// else about it — the prefix, the tags, the priority — and follows the working
// trees that lived inside it.
func MoveFolder(from, to string) error {
	if loadErr != nil {
		return loadErr
	}
	unmarshalConfig()

	_, index, ok := FindOwnFolder(from)
	if !ok {
		return fmt.Errorf("%s is not a folder in %s", from, ConfigFile())
	}
	if _, _, taken := FindOwnFolder(to); taken {
		return fmt.Errorf("%s is already a folder in %s", to, ConfigFile())
	}

	// Moving a project renames it, unless it was given a name of its own: the
	// short name is the last element of the path. Its working trees hang off
	// that name, so they have to be told.
	oldName := fullShortName(c.Folders[index])
	c.Folders[index].Path = to
	newName := fullShortName(c.Folders[index])

	oldPrefix := cleanPath(from) + string(filepath.Separator)
	for i, worktree := range c.Worktrees {
		if worktree.Project == oldName && oldName != newName {
			c.Worktrees[i].Project = newName
			cli.Debug("Worktree %s is now %s%s%s", worktree.ShortName(), newName, WorktreeSeparator, worktree.Name)
		}
		// And one that lived inside the project moved with it.
		if strings.HasPrefix(cleanPath(worktree.Path), oldPrefix) {
			relative := strings.TrimPrefix(cleanPath(worktree.Path), oldPrefix)
			c.Worktrees[i].Path = filepath.Join(to, relative)
			cli.Debug("Worktree %s follows to %s", worktree.ShortName(), c.Worktrees[i].Path)
		}
	}

	viper.Set("folders", c.Folders)
	viper.Set("worktrees", c.Worktrees)
	if err := viper.WriteConfig(); err != nil {
		cli.Error("Failed to write config %v", err)
		return err
	}
	refreshEffective()

	cli.Info("Moved %s to %s in config", from, to)
	return nil
}

// SetFolderTags replaces the tags of a configuration entry.
func SetFolderTags(path string, tags []string) error {
	if loadErr != nil {
		return loadErr
	}
	unmarshalConfig()

	_, index, ok := FindOwnFolder(path)
	if !ok {
		return fmt.Errorf("%s is not a folder in %s", path, ConfigFile())
	}

	c.Folders[index].Tags = NormalizeTags(tags)

	viper.Set("folders", c.Folders)
	if err := viper.WriteConfig(); err != nil {
		cli.Error("Failed to write config %v", err)
		return err
	}
	refreshEffective()

	return nil
}

// fullShortName is the name a folder is reached by, prefix included, which is
// what a working tree records as its project.
func fullShortName(folder Folder) string {
	if folder.Prefix == "" {
		return folder.ShortName()
	}
	return folder.Prefix + "-" + folder.ShortName()
}
