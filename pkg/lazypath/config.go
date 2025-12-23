package lazypath

import (
	"fmt"
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

// Config represents the application configuration
type Config struct {
	Folders    []Folder    `yaml:"folders" mapstructure:"folders"`
	GitServers []GitServer `yaml:"gitServers" mapstructure:"gitServers"`
}

type GitServer struct {
	Name         string `yaml:"name" mapstructure:"name"`
	Type         string `yaml:"type" mapstructure:"type"`
	HTTPS        string `yaml:"https" mapstructure:"https"`
	SSH          string `yaml:"ssh" mapstructure:"ssh"`
	PreferGitSSH bool   `yaml:"preferGitSSH" mapstructure:"preferGitSSH"`
}

func unmarshalConfig() {
	if !reflect.DeepEqual(c, Config{}) {
		return
	}

	err := viper.Unmarshal(&c)
	if err != nil {
		cli.Error("Unable to decode into struct %v", err)
	}
	if err := c.Validate(); err != nil {
		cli.Warn("Config validation warning %v", err)
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	for i, folder := range c.Folders {
		if folder.Path == "" {
			return fmt.Errorf("folder at index %d has empty path", i)
		}
		if _, err := os.Stat(folder.Path); err != nil {
			if os.IsNotExist(err) {
				cli.Debug(fmt.Sprintf("Folder path does not exist: %s", folder.Path))
			}
		}
	}
	return nil
}

// GetConfig returns the current configuration
func GetConfig() Config {
	unmarshalConfig()
	return c
}

// SetTestConfig sets the configuration for testing purposes
func SetTestConfig(config Config) {
	c = config
}

// ResetTestConfig resets the configuration to empty state
func ResetTestConfig() {
	c = Config{}
}

// InitConfig initializes the configuration from file or creates a new one
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
