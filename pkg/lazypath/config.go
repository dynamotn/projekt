package lazypath

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"

	"github.com/OpenPeeDeeP/xdg"
	"github.com/spf13/viper"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

var (
	CfgFile string
	c       Config
	// loadErr records why the existing config file could not be read. Commands
	// must refuse to run, and above all refuse to write, while it is set:
	// writing would replace the unreadable file with the empty in-memory config.
	loadErr error
)

// LoadError returns the error that prevented the config file from being read,
// or nil when the configuration is usable.
func LoadError() error {
	return loadErr
}

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

// Validate checks if the configuration is valid.
//
// Problems that make a folder unusable are returned as an error; problems that
// only affect one entry (a missing directory, an unknown git server) are logged
// so the rest of the configuration keeps working.
func (c *Config) Validate() error {
	var errs []error

	servers := make(map[string]struct{}, len(c.GitServers))
	for i, server := range c.GitServers {
		if server.Name == "" {
			errs = append(errs, fmt.Errorf("git server at index %d has empty name", i))
			continue
		}
		if _, dup := servers[server.Name]; dup {
			errs = append(errs, fmt.Errorf("git server %q is defined more than once", server.Name))
		}
		servers[server.Name] = struct{}{}
	}

	paths := make(map[string]struct{}, len(c.Folders))
	for i, folder := range c.Folders {
		if folder.Path == "" {
			errs = append(errs, fmt.Errorf("folder at index %d has empty path", i))
			continue
		}

		key := cleanPath(folder.Path)
		if _, dup := paths[key]; dup {
			errs = append(errs, fmt.Errorf("folder %s is configured more than once", folder.Path))
		}
		paths[key] = struct{}{}

		if !filepath.IsAbs(folder.Path) {
			cli.Warn("Folder path is not absolute, it will resolve against the current directory: %s", folder.Path)
		}
		if _, err := os.Stat(folder.Path); err != nil {
			if os.IsNotExist(err) {
				cli.Debug("Folder path does not exist: %s", folder.Path)
			} else {
				cli.Warn("Cannot access folder %s: %v", folder.Path, err)
			}
		}
		if folder.IsWorkspace {
			if folder.Name != "" {
				cli.Warn("Folder %s is a workspace, its name %q is ignored", folder.Path, folder.Name)
			}
			if _, err := regexp.Compile(folder.GetRegexMatch()); err != nil {
				errs = append(errs, fmt.Errorf("folder %s has an invalid regex %q: %w", folder.Path, folder.GetRegexMatch(), err))
			}
		}
		if folder.Git != nil {
			if _, ok := servers[folder.Git.Host]; !ok {
				cli.Warn("Folder %s references unknown git server %q", folder.Path, folder.Git.Host)
			}
		}
	}

	return errors.Join(errs...)
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
	loadErr = nil
}

// InitConfig initializes the configuration from file or creates a new one
func InitConfig() {
	if CfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(CfgFile)
	} else {
		// Search config in the XDG config home.
		viper.AddConfigPath(filepath.Join(xdg.ConfigHome(), "projekt"))
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
		CfgFile = filepath.Join(xdg.ConfigHome(), "projekt", "config.yaml")
	}

	viper.AutomaticEnv()

	loadErr = nil

	err := viper.ReadInConfig()
	if err == nil {
		cli.Debug("Using config file: %v", viper.ConfigFileUsed())
		return
	}

	if !isConfigMissing(err) {
		// The file exists but is unusable (malformed YAML, bad permissions...).
		// Never recreate it here: that would silently wipe the user config.
		loadErr = fmt.Errorf("failed to read config file %s: %w", CfgFile, err)
		return
	}

	if err := createEmptyConfig(CfgFile); err != nil {
		loadErr = fmt.Errorf("failed to create config file %s: %w", CfgFile, err)
	}
}

// isConfigMissing reports whether the error only means "there is no config file yet".
func isConfigMissing(err error) bool {
	var notFound viper.ConfigFileNotFoundError
	if errors.As(err, &notFound) {
		return true
	}
	return errors.Is(err, os.ErrNotExist)
}

// createEmptyConfig creates an empty config file, never overwriting an existing one.
func createEmptyConfig(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create folder: %w", err)
	}

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			// Another process created it in the meantime, leave it alone.
			return nil
		}
		return err
	}
	return f.Close()
}
