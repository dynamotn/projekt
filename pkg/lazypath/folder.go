package lazypath

import (
	"fmt"

	"github.com/spf13/viper"
	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

const DEFAULT_REGEX_WORKSPACE = ".+"

type Folder struct {
	Path        string
	Prefix      string
	IsWorkspace bool   `yaml:"is_workspace" mapstructure:"is_workspace"`
	RegexMatch  string `yaml:"regex" mapstructure:"regex"`
	Priority    uint16
}

func (f *Folder) GetRegexMatch() string {
	if !f.IsWorkspace {
		return ""
	} else {
		if f.RegexMatch != "" {
			return f.RegexMatch
		} else {
			return DEFAULT_REGEX_WORKSPACE
		}
	}
}

func CheckFolderExist(path string) bool {
	unmarshalConfig()

	result := false
	for _, folder := range c.Folders {
		if folder.Path == path {
			return true
		}
	}
	return result
}

func (f *Folder) AddToConfig() error {
	unmarshalConfig()

	if CheckFolderExist(f.Path) {
		fmt.Println(f.Path + " is already existed!")
		return nil
	}

	c.Folders = append(c.Folders, *f)
	viper.Set("folders", c.Folders)
	err := viper.WriteConfig()
	if err != nil {
		cli.Error("Failed to write config", err)
		return err
	}

	fmt.Println("Added " + f.Path + " to config")
	return nil
}
