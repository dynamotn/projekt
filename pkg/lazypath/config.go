package lazypath

import (
	"os"
	"path/filepath"
	"reflect"

	"github.com/OpenPeeDeeP/xdg"
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
		cli.Error("Unable to decode into struct %v", err)
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
		// Search config in home directory with name ".cobra" (without extension).
		viper.AddConfigPath(filepath.Join(xdg.ConfigHome(), "projekt"))
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
		CfgFile = filepath.Join(xdg.ConfigHome(), "projekt", "config.yaml")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		cli.Debug("Using config file: %v", viper.ConfigFileUsed())
	} else {
		err := os.MkdirAll(filepath.Dir(CfgFile), os.ModePerm)
		if err != nil && !os.IsExist(err) {
			cli.Error("Failed to create folder %v", err)
		}

		_, err = os.Create(CfgFile)
		if err != nil {
			cli.Error("Failed to create file %v", err)
		}
	}
}
