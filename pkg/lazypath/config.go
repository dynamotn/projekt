package lazypath

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

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

// Severity says how much a configuration problem matters.
type Severity int

const (
	// SeverityWarning marks something suspicious that still leaves the rest of
	// the configuration working.
	SeverityWarning Severity = iota
	// SeverityError marks something that makes an entry unusable.
	SeverityError
)

func (s Severity) String() string {
	if s == SeverityError {
		return "ERROR"
	}
	return "WARNING"
}

// Diagnostic is one problem found in the configuration.
type Diagnostic struct {
	Severity Severity
	Message  string
}

// Diagnose reports every problem in the configuration, errors and warnings
// alike, in the order they were found.
//
// Validate reduces this to the errors, which is what loading the config cares
// about; `projekt config check` shows the whole list, because a warning is
// exactly the kind of thing worth catching before it surprises someone.
func (c *Config) Diagnose() []Diagnostic {
	var diags []Diagnostic
	errorf := func(format string, v ...any) {
		diags = append(diags, Diagnostic{SeverityError, fmt.Sprintf(format, v...)})
	}
	warnf := func(format string, v ...any) {
		diags = append(diags, Diagnostic{SeverityWarning, fmt.Sprintf(format, v...)})
	}

	servers := make(map[string]struct{}, len(c.GitServers))
	for i, server := range c.GitServers {
		if server.Name == "" {
			errorf("git server at index %d has empty name", i)
			continue
		}
		if _, dup := servers[server.Name]; dup {
			errorf("git server %q is defined more than once", server.Name)
		}
		servers[server.Name] = struct{}{}

		if server.HTTPS == "" && server.SSH == "" {
			errorf("git server %q has neither an https nor an ssh URL", server.Name)
		}
	}

	paths := make(map[string]struct{}, len(c.Folders))
	for i, folder := range c.Folders {
		if folder.Path == "" {
			errorf("folder at index %d has empty path", i)
			continue
		}

		key := cleanPath(folder.Path)
		if _, dup := paths[key]; dup {
			errorf("folder %s is configured more than once", folder.Path)
		}
		paths[key] = struct{}{}

		if !filepath.IsAbs(folder.Path) {
			warnf("folder path is not absolute, it will resolve against the current directory: %s", folder.Path)
		}
		if _, err := os.Stat(folder.Path); err != nil {
			if os.IsNotExist(err) {
				warnf("folder path does not exist: %s", folder.Path)
			} else {
				warnf("cannot access folder %s: %v", folder.Path, err)
			}
		}
		if folder.IsWorkspace {
			if folder.Name != "" {
				warnf("folder %s is a workspace, its name %q is ignored", folder.Path, folder.Name)
			}
			if _, err := regexp.Compile(folder.GetRegexMatch()); err != nil {
				errorf("folder %s has an invalid regex %q: %v", folder.Path, folder.GetRegexMatch(), err)
			}
		}
		if folder.Git != nil {
			diags = append(diags, diagnoseGit(folder, servers)...)
		}
	}

	return diags
}

// diagnoseGit checks the git section of one folder.
func diagnoseGit(folder Folder, servers map[string]struct{}) []Diagnostic {
	var diags []Diagnostic
	errorf := func(format string, v ...any) {
		diags = append(diags, Diagnostic{SeverityError, fmt.Sprintf(format, v...)})
	}
	warnf := func(format string, v ...any) {
		diags = append(diags, Diagnostic{SeverityWarning, fmt.Sprintf(format, v...)})
	}

	if _, ok := servers[folder.Git.Host]; !ok {
		warnf("folder %s references unknown git server %q", folder.Path, folder.Git.Host)
	}

	// Two repositories checked out to the same place would fight over it on
	// every sync, so the collision is worth naming before it happens.
	targets := make(map[string]string, len(folder.Git.Repos))
	for i, repo := range folder.Git.Repos {
		if repo.Name == "" {
			errorf("folder %s has a repo at index %d with empty name", folder.Path, i)
			continue
		}

		target := repo.Path
		if target == "" {
			target = repo.Name
		}
		if previous, dup := targets[target]; dup {
			errorf("folder %s checks out both %q and %q into %q", folder.Path, previous, repo.Name, target)
		}
		targets[target] = repo.Name

		for name, url := range repo.Remotes {
			if strings.TrimSpace(name) == "" {
				errorf("folder %s, repo %s has a remote with an empty name", folder.Path, repo.Name)
				continue
			}
			if strings.TrimSpace(url) == "" {
				errorf("folder %s, repo %s has remote %q with an empty URL", folder.Path, repo.Name, name)
			}
		}

		for j, worktree := range repo.Worktrees {
			if strings.TrimSpace(worktree.Path) == "" {
				errorf("folder %s, repo %s has a worktree at index %d with empty path", folder.Path, repo.Name, j)
				continue
			}
			// Without a branch git would name one after the path, which is
			// almost never the branch that was meant.
			if strings.TrimSpace(worktree.Branch) == "" {
				errorf("folder %s, repo %s has worktree %q with no branch", folder.Path, repo.Name, worktree.Path)
			}
			if previous, dup := targets[worktree.Path]; dup {
				errorf("folder %s checks out both %q and worktree %q into %q", folder.Path, previous, repo.Name, worktree.Path)
			}
			targets[worktree.Path] = repo.Name + " worktree"
		}
	}

	return diags
}

// Validate checks if the configuration is valid.
//
// Problems that make a folder unusable are returned as an error; problems that
// only affect one entry (a missing directory, an unknown git server) are logged
// so the rest of the configuration keeps working.
func (c *Config) Validate() error {
	var errs []error

	for _, diag := range c.Diagnose() {
		if diag.Severity == SeverityError {
			errs = append(errs, errors.New(diag.Message))
			continue
		}
		cli.Warn("%s", diag.Message)
	}

	return errors.Join(errs...)
}

// GetConfig returns the current configuration
func GetConfig() Config {
	unmarshalConfig()
	return c
}

// ConfigFile returns the path of the config file in use. It is the file viper
// actually read when there was one, and otherwise the path a first write would
// create.
func ConfigFile() string {
	if used := viper.ConfigFileUsed(); used != "" {
		return used
	}
	return CfgFile
}

// ReloadConfig re-reads the config file from disk and returns why it could not
// be read, if it could not. It is what an editor session needs afterwards: the
// unmarshalled config is cached, so without this the process would keep serving
// what the file said before the edit.
func ReloadConfig() error {
	c = Config{}
	loadErr = nil

	if err := viper.ReadInConfig(); err != nil {
		if !isConfigMissing(err) {
			loadErr = fmt.Errorf("failed to read config file %s: %w", ConfigFile(), err)
		}
		return loadErr
	}

	return nil
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
