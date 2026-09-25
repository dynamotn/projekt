package lazypath

import (
	"fmt"
	"path/filepath"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
)

// AddRepoToFolder records a repository under a folder's git section, so that
// `folder sync` reproduces it on the next machine and `folder check` notices
// when it goes missing.
//
// The folder must be one of this machine's own entries: a workspace declared
// in an included file belongs to that file.
func AddRepoToFolder(folderPath, repoName, repoPath string) error {
	if loadErr != nil {
		return loadErr
	}
	unmarshalConfig()

	ok, index := CheckFolderExist(folderPath)
	if !ok {
		return fmt.Errorf("%s is not a folder in %s", folderPath, ConfigFile())
	}
	if c.Folders[index].Git == nil {
		return fmt.Errorf("%s has no git section to add a repository to", folderPath)
	}

	for _, repo := range c.Folders[index].Git.Repos {
		if repo.Name == repoName {
			cli.Debug("%s already lists the repository %s", folderPath, repoName)
			return nil
		}
	}

	repo := GitRepo{Name: repoName}
	// The path is only worth recording when it differs from the name, which is
	// what `folder sync` assumes when it is absent.
	if repoPath != "" && filepath.Base(filepath.Clean(repoPath)) != repoName {
		repo.Path = repoPath
	}
	c.Folders[index].Git.Repos = append(c.Folders[index].Git.Repos, repo)

	if err := saveConfig("folders"); err != nil {
		cli.Error("Failed to write config %v", err)
		return err
	}
	refreshEffective()

	cli.Info("Added repository %s to %s in config", repoName, folderPath)
	return nil
}
