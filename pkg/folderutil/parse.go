package folderutil

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

type ParsedFolder struct {
	ShortName string
	Path      string
	Workspace string
}

func isDirOrSymlinkToDir(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}

	if info.Mode()&os.ModeSymlink != 0 {
		realPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			return false, err
		}
		info, err = os.Stat(realPath)
		if err != nil {
			return false, err
		}
	}
	return info.IsDir(), nil
}

func ParseConfig(c lazypath.Config) ([]ParsedFolder, error) {
	var result []ParsedFolder
	var err error

	for _, folder := range c.Folders {
		prefix := ""
		if folder.Prefix != "" {
			prefix = folder.Prefix + "-"
		}

		if !folder.IsWorkspace {
			result = appendToParsedFolder(result, prefix, folder.Path, "")
		} else {
			re := regexp.MustCompile(folder.GetRegexMatch())
			dirEntry, err := os.ReadDir(folder.Path)
			if err != nil {
				cli.Warning("Cannot read folder "+folder.Path, err)
				continue
			}

			for _, entry := range dirEntry {
				fullPath := filepath.Join(folder.Path, entry.Name())
				ok, err := isDirOrSymlinkToDir(fullPath)
				if err != nil || !ok {
					cli.Debug("Not is directory or symlink: " + entry.Name())
					continue
				}
				if !re.MatchString(entry.Name()) {
					cli.Debug("Not Match: " + entry.Name())
					continue
				}
				cli.Debug("Match: " + entry.Name())
				result = appendToParsedFolder(result, prefix, folder.Path, entry.Name())
			}
		}
	}

	return result, err
}

func appendToParsedFolder(list []ParsedFolder, prefix string, folderPath string, childFolderName string) []ParsedFolder {
	result := list

	// Get existed short name
	existedName := make(map[string]string)
	for _, pFolder := range list {
		existedName[pFolder.ShortName] = pFolder.Path
	}

	shortName := prefix + childFolderName
	childFolderPath := strings.TrimRight(filepath.Join(folderPath, childFolderName), "/")
	if childFolderName == "" {
		shortName = prefix + filepath.Base(folderPath)
	}

	_, exists := existedName[shortName]
	if exists {
		cli.Debug("Not Valid: " + childFolderPath + " with existed short name " + shortName)
		return list
	}

	result = append(result, ParsedFolder{
		ShortName: shortName,
		Path:      childFolderPath,
		Workspace: folderPath,
	})
	return result
}
