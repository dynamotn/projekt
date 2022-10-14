package lazypath

import (
	"fmt"
	"strings"

	"github.com/samber/lo"
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

func CheckFolderExist(path string) (bool, int) {
	unmarshalConfig()

	_, index, ok := lo.FindIndexOf[Folder](c.Folders, func(folder Folder) bool {
		return strings.TrimRight(folder.Path, "/") == strings.TrimRight(path, "/")
	})

	return ok, index
}

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
