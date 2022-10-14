package folderutil

import (
	"io/ioutil"
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
			fileInfo, err := ioutil.ReadDir(folder.Path)
			if err != nil {
				cli.Warning("Cannot read folder "+folder.Path, err)
				continue
			}

			for _, f := range fileInfo {
				if !f.IsDir() {
					continue
				}
				if !re.MatchString(f.Name()) {
					cli.Debug("Not Match: " + f.Name())
					continue
				}
				cli.Debug("Match: " + f.Name())
				result = appendToParsedFolder(result, prefix, folder.Path, f.Name())
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
