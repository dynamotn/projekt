// Package folderutil provides utilities for managing and parsing project folders.
//
// It offers functionality to parse folder configurations, find folders by short names,
// and manage folder lists with support for workspaces and regex matching.
//
// Example usage:
//
//	config := lazypath.GetConfig()
//	folders, err := folderutil.ParseConfig(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	for _, folder := range folders {
//	    fmt.Printf("%s -> %s\n", folder.ShortName, folder.Path)
//	}
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
			cli.Warning("Invalid regex pattern for "+folder.Path, err)
			continue
		}

		entries, err := os.ReadDir(folder.Path)
		if err != nil {
			cli.Warning("Cannot read folder "+folder.Path, err)
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				info, err := entry.Info()
				if err != nil || info.Mode()&os.ModeSymlink == 0 {
					cli.Debug("Not is directory or symlink: " + entry.Name())
					continue
				}
			}
			if !re.MatchString(entry.Name()) {
				cli.Debug("Not Match: " + entry.Name())
				continue
			}
			cli.Debug("Match: " + entry.Name())
			result = appendToParsedFolder(result, prefix, folder.Path, entry.Name())
		}
	}

	return result, nil
}

func appendToParsedFolder(list []ParsedFolder, prefix, folderPath, childFolderName string) []ParsedFolder {
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
