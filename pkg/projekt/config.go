package projekt

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	CfgFile string
	c config
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
		log.Fatalf("unable to decode into struct, %v", err)
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
		Path: path,
		Prefix: prefix,
		IsWorkspace: asWorkspace,
	})

	viper.Set("folders", c.Folders)
	err := viper.WriteConfig()
	if err != nil {
		log.Fatal(err)
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
		cobra.CheckErr(err)

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
			log.Fatal(err)
    }

		_, err = os.Create(CfgFile)
    if err != nil {
			log.Fatal(err)
    }
	}
}
