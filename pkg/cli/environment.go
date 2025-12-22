package cli

import (
	"os"

	"github.com/spf13/pflag"
)

var env = &EnvSettings{}

type EnvSettings struct {
	LogLevel string
}

func init() {
	// Check for LOG_LEVEL first (standard), then PROJEKT_LOG_LEVEL (legacy)
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = os.Getenv("PROJEKT_LOG_LEVEL")
	}
	if logLevel == "" {
		logLevel = "info" // default level
	}
	env.LogLevel = logLevel
}

func (e *EnvSettings) AddFlags(fs *pflag.FlagSet) {
	fs.StringVarP(&e.LogLevel, "verbose", "v", "info", "Log level, available options are: (trace, debug, info, warn, error, fatal)")
}

func GetEnv() *EnvSettings {
	return env
}
