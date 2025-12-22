package lazypath

import (
	"strings"

	"github.com/samber/lo"
	"github.com/spf13/viper"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

const defaultRegexWorkspace = `^[^.].+`

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
			return defaultRegexWorkspace
		}
	}
}

func CheckFolderExist(path string) (bool, int) {
	unmarshalConfig()

	_, index, ok := lo.FindIndexOf(c.Folders, func(folder Folder) bool {
		return strings.TrimRight(folder.Path, "/") == strings.TrimRight(path, "/")
	})

	return ok, index
}

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
