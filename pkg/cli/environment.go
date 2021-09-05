package cli

import (
	"os"
	"strconv"

	"github.com/spf13/pflag"
)

var (
	env EnvSettings
)

type EnvSettings struct {
	//Debug indicates whether or not Projekt is running in Debug mode.
	Debug bool
}

func init() {
	env.Debug, _ = strconv.ParseBool(os.Getenv("PROJEKT_DEBUG"))
}

func (e EnvSettings) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVarP(&e.Debug, "debug", "d", false, "Enable verbose ouput")
}

func GetEnv() EnvSettings {
	return env
}
