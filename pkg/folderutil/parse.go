package folderutil

import (
	"os"
	"path/filepath"
	"regexp"
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

// ParseConfig parses the configuration and returns a list of parsed folders
func ParseConfig(c lazypath.Config) ([]ParsedFolder, error) {
	var result []ParsedFolder

	for _, folder := range c.Folders {
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
			if !entry.IsDir() {
				info, err := entry.Info()
				if err != nil || info.Mode()&os.ModeSymlink == 0 {
					cli.Debug("Not is directory or symlink: %s", entry.Name())
					continue
				}
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
