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
	c       Config
)

type Config struct {
	Folders []Folder
}

func unmarshalConfig() {
	if !reflect.DeepEqual(c, Config{}) {
		return
	}

	err := viper.Unmarshal(&c)
	if err != nil {
		cli.Error("Unable to decode into struct", err)
	}
}

func GetConfig() Config {
	unmarshalConfig()
	return c
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
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
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
