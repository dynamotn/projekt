package cli

import (
	"os"

	"github.com/spf13/pflag"
)

var (
	env = &EnvSettings{}
)

// EnvSettings contains environment configuration settings
type EnvSettings struct {
	LogLevel string
}

// Valid log levels
const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

func init() {
	env.LogLevel = os.Getenv("PROJEKT_LOG_LEVEL")
}

// AddFlags adds environment flags to the flag set
func (e *EnvSettings) AddFlags(fs *pflag.FlagSet) {
	fs.StringVarP(&e.LogLevel, "verbose", "v", LogLevelInfo, "Log level, available options are: (debug, info, warn, error)")
}

// IsDebug returns true if debug logging is enabled
func (e *EnvSettings) IsDebug() bool {
	return e.LogLevel == LogLevelDebug
}

// IsInfoOrAbove returns true if info or higher logging is enabled
func (e *EnvSettings) IsInfoOrAbove() bool {
	return e.LogLevel == LogLevelDebug || e.LogLevel == LogLevelInfo || e.LogLevel == LogLevelWarn
}

// GetEnv returns the current environment settings
func GetEnv() *EnvSettings {
	return env
}
