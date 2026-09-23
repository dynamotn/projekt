package cli

import (
	"os"

	"github.com/spf13/pflag"
)

// DefaultLogLevel is used when neither the environment nor a flag sets a level.
const DefaultLogLevel = INFO

var env = &EnvSettings{}

type EnvSettings struct {
	LogLevel string
}

func init() {
	env.LogLevel = logLevelFromEnv()
}

// logLevelFromEnv reads the log level from the environment.
func logLevelFromEnv() string {
	// Check for LOG_LEVEL first (standard), then PROJEKT_LOG_LEVEL (legacy)
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = os.Getenv("PROJEKT_LOG_LEVEL")
	}
	if logLevel == "" {
		logLevel = DefaultLogLevel
	}
	return logLevel
}

func (e *EnvSettings) AddFlags(fs *pflag.FlagSet) {
	if e.LogLevel == "" {
		e.LogLevel = DefaultLogLevel
	}
	// The current value is the flag default, otherwise registering the flag
	// would silently overwrite a level coming from LOG_LEVEL.
	fs.StringVarP(&e.LogLevel, "verbose", "v", e.LogLevel, "Log level, available options are: (trace, debug, info, warn, error, fatal)")
}

func GetEnv() *EnvSettings {
	return env
}
