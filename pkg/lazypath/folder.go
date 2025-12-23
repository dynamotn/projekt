package lazypath

import (
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
	Prefix      string     `yaml:"prefix" mapstructure:"prefix"`
	IsWorkspace bool       `yaml:"is_workspace" mapstructure:"is_workspace"`
	RegexMatch  string     `yaml:"regex" mapstructure:"regex"`
	Priority    uint16     `yaml:"priority" mapstructure:"priority"`
	Git         *GitConfig `yaml:"git,omitempty" mapstructure:"git,omitempty"`
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

// CheckFolderExist checks if a folder path exists in the configuration
func CheckFolderExist(path string) (bool, int) {
	unmarshalConfig()

	normalizedPath := strings.TrimRight(path, "/")
	_, index, ok := lo.FindIndexOf(c.Folders, func(folder Folder) bool {
		return strings.TrimRight(folder.Path, "/") == normalizedPath
	})

	return ok, index
}

// AddToConfig adds the folder to the configuration file
func (f *Folder) AddToConfig() error {
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
