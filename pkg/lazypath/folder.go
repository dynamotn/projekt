package lazypath

import (
	"path/filepath"
	"strings"

	"github.com/samber/lo"
	"github.com/spf13/viper"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

const defaultRegexWorkspace = `^[^.].+`

type GitRepo struct {
	Name string `yaml:"name" mapstructure:"name"`
	Path string `yaml:"path" mapstructure:"path"`
}

type GitConfig struct {
	Host  string    `yaml:"host" mapstructure:"host"`
	Group string    `yaml:"group" mapstructure:"group"`
	Repos []GitRepo `yaml:"repos" mapstructure:"repos"`
}

type Folder struct {
	Path        string     `yaml:"path" mapstructure:"path"`
	Name        string     `yaml:"name,omitempty" mapstructure:"name"`
	Prefix      string     `yaml:"prefix" mapstructure:"prefix"`
	IsWorkspace bool       `yaml:"is_workspace" mapstructure:"is_workspace"`
	RegexMatch  string     `yaml:"regex" mapstructure:"regex"`
	Priority    uint16     `yaml:"priority" mapstructure:"priority"`
	Git         *GitConfig `yaml:"git,omitempty" mapstructure:"git,omitempty"`
}

// ShortName returns the name the folder itself is reachable by, without the
// prefix. It is the configured name, or the last element of the path.
//
// A workspace has no short name of its own; its children are named after their
// own directory.
func (f *Folder) ShortName() string {
	if f.Name != "" {
		return f.Name
	}
	// Clean first: the base of a path such as "/home/me/dotfiles/home/.."
	// is "..", which is not a usable name.
	return filepath.Base(cleanPath(f.Path))
}

func (f *Folder) GetRegexMatch() string {
	if !f.IsWorkspace {
		return ""
	}
	if f.RegexMatch != "" {
		return f.RegexMatch
	}
	return defaultRegexWorkspace
}

// cleanPath makes two spellings of the same path comparable.
func cleanPath(path string) string {
	trimmed := strings.TrimRight(path, "/")
	if trimmed == "" {
		return path
	}
	return filepath.Clean(trimmed)
}

// CheckFolderExist checks if a folder path exists in the configuration
func CheckFolderExist(path string) (bool, int) {
	unmarshalConfig()

	normalizedPath := cleanPath(path)
	_, index, ok := lo.FindIndexOf(c.Folders, func(folder Folder) bool {
		return cleanPath(folder.Path) == normalizedPath
	})

	return ok, index
}

// AddToConfig adds the folder to the configuration file
func (f *Folder) AddToConfig() error {
	if loadErr != nil {
		return loadErr
	}
	unmarshalConfig()
	isExisted, _ := CheckFolderExist(f.Path)

	if isExisted {
		cli.Info("%s is already existed!", f.Path)
		return nil
	}

	c.Folders = append(c.Folders, *f)
	viper.Set("folders", c.Folders)
	err := viper.WriteConfig()
	if err != nil {
		cli.Error("Failed to write config %v", err)
		return err
	}

	cli.Info("Added %s to config", f.Path)
	return nil
}

// RemoveFromConfig removes a folder from the configuration by path
func RemoveFromConfig(path string) error {
	if loadErr != nil {
		return loadErr
	}
	unmarshalConfig()

	isExisted, index := CheckFolderExist(path)

	if !isExisted {
		cli.Info("%s wasn't added as project!", path)
		return nil
	}

	c.Folders = append(c.Folders[:index], c.Folders[index+1:]...)
	viper.Set("folders", c.Folders)
	err := viper.WriteConfig()
	if err != nil {
		cli.Error("Failed to write config %v", err)
		return err
	}

	cli.Info("Removed %s from config", path)
	return nil
}
