package cli

import (
	"os"

	"github.com/spf13/pflag"
)

var (
	env = &EnvSettings{}
)

type EnvSettings struct {
	LogLevel string
}

func init() {
	env.LogLevel = os.Getenv("PROJEKT_LOG_LEVEL")
}

func (e *EnvSettings) AddFlags(fs *pflag.FlagSet) {
	fs.StringVarP(&e.LogLevel, "verbose", "v", "info", "Log level, available options are: (debug, info, error)")
}

func GetEnv() *EnvSettings {
	return env
}
