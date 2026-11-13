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

	listed := false
	err := updateConfig(func(config *Config) error {
		_, index, ok := config.findFolder(folderPath)
		if !ok {
			return fmt.Errorf("%s is not a folder in %s", folderPath, ConfigFile())
		}
		git := config.Folders[index].Git
		if git == nil {
			return fmt.Errorf("%s has no git section to add a repository to", folderPath)
		}

		for _, repo := range git.Repos {
			if repo.Name == repoName {
				listed = true
				return errNoChange
			}
		}

		repo := GitRepo{Name: repoName}
		// The path is only worth recording when it differs from the name,
		// which is what `folder sync` assumes when it is absent.
		if repoPath != "" && filepath.Base(filepath.Clean(repoPath)) != repoName {
			repo.Path = repoPath
		}
		git.Repos = append(git.Repos, repo)
		return nil
	}, "folders")
	if err != nil {
		cli.Error("Failed to write config %v", err)
		return err
	}

	if listed {
		cli.Debug("%s already lists the repository %s", folderPath, repoName)
		return nil
	}
	cli.Info("Added repository %s to %s in config", repoName, folderPath)
	return nil
}
