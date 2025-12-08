package lazypath

import (
	"fmt"
	"strings"

	"github.com/samber/lo"
	"github.com/spf13/viper"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

const defaultRegexWorkspace = ".+"

// Folder represents a project folder configuration
type Folder struct {
	Path        string
	Prefix      string
	IsWorkspace bool   `yaml:"is_workspace" mapstructure:"is_workspace"`
	RegexMatch  string `yaml:"regex" mapstructure:"regex"`
	Priority    uint16
}

// GetRegexMatch returns the regex pattern for matching folders
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

// RemoveFromConfig removes a folder from the configuration by path
func RemoveFromConfig(path string) error {
	unmarshalConfig()

	isExisted, index := CheckFolderExist(path)

	if !isExisted {
		fmt.Println(path + " wasn't added as project!")
		return nil
	}

	c.Folders = append(c.Folders[:index], c.Folders[index+1:]...)
	viper.Set("folders", c.Folders)
	err := viper.WriteConfig()
	if err != nil {
		cli.Error("Failed to write config", err)
		return err
	}

	fmt.Println("Removed " + path + " from config")
	return nil
}
