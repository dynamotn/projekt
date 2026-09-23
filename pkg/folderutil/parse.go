package folderutil

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

// ParsedFolder represents a folder with its short name, path, and workspace
type ParsedFolder struct {
	ShortName string
	Path      string
	Workspace string
	// Tags are the tags of the folder this came from. A folder found inside a
	// workspace inherits the workspace's tags, because it has no configuration
	// entry of its own to carry them.
	Tags []string
}

// ParseConfig parses the configuration and returns a list of parsed folders.
//
// Folders are visited from the highest priority to the lowest, so that the
// higher priority folder wins when two folders resolve to the same short name.
func ParseConfig(c lazypath.Config) ([]ParsedFolder, error) {
	var result []ParsedFolder
	// shortNames tracks the short names already taken, so that detecting a
	// duplicate stays constant time instead of scanning the whole result.
	shortNames := make(map[string]struct{})

	for _, folder := range sortFoldersByPriority(c.Folders) {
		prefix := ""
		if folder.Prefix != "" {
			prefix = folder.Prefix + "-"
		}

		// Clean the configured path: it is what gets printed and cd'ed into,
		// and its last element is the fallback short name.
		folderPath := filepath.Clean(folder.Path)

		tags := folder.GetTags()

		if !folder.IsWorkspace {
			result = appendToParsedFolder(result, shortNames, prefix+folder.ShortName(), folderPath, folderPath, tags)
			continue
		}
		re, err := regexp.Compile(folder.GetRegexMatch())
		if err != nil {
			cli.Warn("Cannot compile regex for folder %s: %v", folder.Path, err)
			continue
		}
		entries, err := os.ReadDir(folderPath)
		if err != nil {
			cli.Warn("Cannot read folder %s: %v", folder.Path, err)
			continue
		}

		for _, entry := range entries {
			if !isDirOrLinkToDir(folderPath, entry) {
				cli.Debug("Not is directory or symlink to directory: %s", entry.Name())
				continue
			}
			if !re.MatchString(entry.Name()) {
				cli.Debug("Not Match: %s", entry.Name())
				continue
			}
			cli.Debug("Match: %s", entry.Name())
			result = appendToParsedFolder(result, shortNames, prefix+entry.Name(), filepath.Join(folderPath, entry.Name()), folderPath, tags)
		}
	}

	return appendWorktrees(result, shortNames, c.Worktrees), nil
}

// appendWorktrees adds the configured working trees, each under
// `<project>@<name>`.
//
// They go in last and through the same duplicate check as everything else, so
// a folder can never lose its name to one. Adding them here is what makes
// `pj myapp@feature`, its completion and the listings work without any of
// them knowing that working trees exist.
func appendWorktrees(list []ParsedFolder, shortNames map[string]struct{}, worktrees []lazypath.Worktree) []ParsedFolder {
	if len(worktrees) == 0 {
		return list
	}

	projects := make(map[string]ParsedFolder, len(list))
	for _, pFolder := range list {
		projects[pFolder.ShortName] = pFolder
	}

	for _, worktree := range worktrees {
		if err := worktree.Validate(); err != nil {
			cli.Warn("Skipping worktree %s: %v", worktree.ShortName(), err)
			continue
		}
		project, ok := projects[worktree.Project]
		if !ok {
			// The project was renamed or removed. `projekt config check` says
			// so; resolving a name that leads nowhere would be worse.
			cli.Debug("Worktree %s has no project named %q", worktree.ShortName(), worktree.Project)
			continue
		}
		// It carries the project's tags: it is the same project, on another
		// branch, so `--tags work` has to reach it too.
		list = appendToParsedFolder(list, shortNames, worktree.ShortName(),
			filepath.Clean(worktree.Path), project.Path, project.Tags)
	}

	return list
}

// sortFoldersByPriority returns the folders ordered by descending priority,
// keeping the configuration order between folders of equal priority.
func sortFoldersByPriority(folders []lazypath.Folder) []lazypath.Folder {
	sorted := make([]lazypath.Folder, len(folders))
	copy(sorted, folders)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Priority > sorted[j].Priority
	})
	return sorted
}

// isDirOrLinkToDir reports whether an entry is a directory, following symlinks.
func isDirOrLinkToDir(parent string, entry os.DirEntry) bool {
	if entry.IsDir() {
		return true
	}
	if entry.Type()&os.ModeSymlink == 0 {
		return false
	}
	// Resolve the symlink: a link to a regular file is not a project folder.
	info, err := os.Stat(filepath.Join(parent, entry.Name()))
	return err == nil && info.IsDir()
}

// shortNameSet indexes the short names already present in a list.
func shortNameSet(list []ParsedFolder) map[string]struct{} {
	set := make(map[string]struct{}, len(list))
	for _, pFolder := range list {
		set[pFolder.ShortName] = struct{}{}
	}
	return set
}

// appendToParsedFolder adds a folder unless its short name is already taken.
func appendToParsedFolder(list []ParsedFolder, shortNames map[string]struct{}, shortName string, path string, workspace string, tags []string) []ParsedFolder {
	// Check for duplicate short names
	if _, exists := shortNames[shortName]; exists {
		cli.Debug("Not Valid: " + path + " with existed short name " + shortName)
		return list
	}
	shortNames[shortName] = struct{}{}

	return append(list, ParsedFolder{
		ShortName: shortName,
		Path:      path,
		Workspace: workspace,
		Tags:      tags,
	})
}
