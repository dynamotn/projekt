package lazypath

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/spf13/viper"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

var (
	CfgFile string
	c       config
)

type folder struct {
	Path        string
	Prefix      string
	IsWorkspace bool `yaml:"is_workspace" mapstructure:"is_workspace"`
}

type config struct {
	Folders []folder
}

func unmarshalConfig() {
	if !reflect.DeepEqual(c, config{}) {
		return
	}

	err := viper.Unmarshal(&c)
	if err != nil {
		cli.Error("Unable to decode into struct", err)
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

func AddFolderConfig(path string, prefix string, asWorkspace bool) {
	unmarshalConfig()

	if CheckFolderExist(path) {
		fmt.Println(path + " is already existed!")
		return
	}

	c.Folders = append(c.Folders, folder{
		Path:        path,
		Prefix:      prefix,
		IsWorkspace: asWorkspace,
	})

	viper.Set("folders", c.Folders)
	err := viper.WriteConfig()
	if err != nil {
		cli.Error("Failed to write config", err)
	}
	fmt.Println("Added " + path + " to config")
}

func InitConfig() {
	if CfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(CfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		if err != nil {
			cli.Error("Failed to detech home user", err)
		}

		// Search config in home directory with name ".cobra" (without extension).
		viper.AddConfigPath(home + "/.projekt")
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
		CfgFile = home + "/.projekt/config.yaml"
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	} else {
		err := os.MkdirAll(filepath.Dir(CfgFile), os.ModePerm)
		if err != nil && !os.IsExist(err) {
			cli.Error("Failed to create folder", err)
		}

		_, err = os.Create(CfgFile)
		if err != nil {
			cli.Error("Failed to create file", err)
		}
	}
}
