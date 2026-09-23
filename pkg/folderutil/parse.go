package folderutil

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

// ParsedFolder represents a folder with its short name, path, and workspace
type ParsedFolder struct {
	ShortName string
	Path      string
	Workspace string
}

// ParseConfig parses the configuration and returns a list of parsed folders.
//
// Folders are visited from the highest priority to the lowest, so that the
// higher priority folder wins when two folders resolve to the same short name.
func ParseConfig(c lazypath.Config) ([]ParsedFolder, error) {
	var result []ParsedFolder

	for _, folder := range sortFoldersByPriority(c.Folders) {
		prefix := ""
		if folder.Prefix != "" {
			prefix = folder.Prefix + "-"
		}

		if !folder.IsWorkspace {
			result = appendToParsedFolder(result, prefix, folder.Path, "")
			continue
		}
		re, err := regexp.Compile(folder.GetRegexMatch())
		if err != nil {
			cli.Warn("Cannot compile regex for folder %s: %v", folder.Path, err)
			continue
		}
		entries, err := os.ReadDir(folder.Path)
		if err != nil {
			cli.Warn("Cannot read folder %s: %v", folder.Path, err)
			continue
		}

		for _, entry := range entries {
			if !isDirOrLinkToDir(folder.Path, entry) {
				cli.Debug("Not is directory or symlink to directory: %s", entry.Name())
				continue
			}
			if !re.MatchString(entry.Name()) {
				cli.Debug("Not Match: %s", entry.Name())
				continue
			}
			cli.Debug("Match: %s", entry.Name())
			result = appendToParsedFolder(result, prefix, folder.Path, entry.Name())
		}
	}

	return result, nil
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

func appendToParsedFolder(list []ParsedFolder, prefix string, folderPath string, childFolderName string) []ParsedFolder {
	shortName := prefix + childFolderName
	if childFolderName == "" {
		shortName = prefix + filepath.Base(folderPath)
	}

	// Check for duplicate short names
	for _, pFolder := range list {
		if pFolder.ShortName == shortName {
			childFolderPath := strings.TrimRight(filepath.Join(folderPath, childFolderName), "/")
			cli.Debug("Not Valid: " + childFolderPath + " with existed short name " + shortName)
			return list
		}
	}

	childFolderPath := strings.TrimRight(filepath.Join(folderPath, childFolderName), "/")
	return append(list, ParsedFolder{
		ShortName: shortName,
		Path:      childFolderPath,
		Workspace: folderPath,
	})
}
