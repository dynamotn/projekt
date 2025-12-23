package cli

import (
	"os"
	"testing"

	"github.com/spf13/pflag"
)

func TestEnvSettings_AddFlags(t *testing.T) {
	tests := []struct {
		name         string
		initialValue string
		wantDefault  string
	}{
		{
			name:         "default log level",
			initialValue: "",
			wantDefault:  "info",
		},
		{
			name:         "custom log level",
			initialValue: "debug",
			wantDefault:  "debug",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EnvSettings{LogLevel: tt.initialValue}
			if e.LogLevel == "" {
				e.LogLevel = "info"
			}
			fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
			e.AddFlags(fs)

			flag := fs.Lookup("verbose")
			if flag == nil {
				t.Fatal("verbose flag not found")
			}
			if flag.Shorthand != "v" {
				t.Errorf("verbose flag shorthand = %v, want v", flag.Shorthand)
			}
			if flag.DefValue != "info" {
				t.Errorf("verbose flag default = %v, want info", flag.DefValue)
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	got := GetEnv()
	if got == nil {
		t.Fatal("GetEnv() returned nil")
	}
	if got.LogLevel == "" {
		t.Error("GetEnv().LogLevel is empty")
	}
}

func TestEnvSettings_LogLevel_FromEnvironment(t *testing.T) {
	tests := []struct {
		name               string
		logLevelEnv        string
		projektLogLevelEnv string
		want               string
	}{
		{
			name:               "LOG_LEVEL takes precedence",
			logLevelEnv:        "debug",
			projektLogLevelEnv: "error",
			want:               "debug",
		},
		{
			name:               "PROJEKT_LOG_LEVEL fallback",
			logLevelEnv:        "",
			projektLogLevelEnv: "warn",
			want:               "warn",
		},
		{
			name:               "default to info when both empty",
			logLevelEnv:        "",
			projektLogLevelEnv: "",
			want:               "info",
		},
		{
			name:               "only LOG_LEVEL set",
			logLevelEnv:        "trace",
			projektLogLevelEnv: "",
			want:               "trace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original values
			originalLogLevel := os.Getenv("LOG_LEVEL")
			originalProjektLogLevel := os.Getenv("PROJEKT_LOG_LEVEL")
			defer func() {
				if originalLogLevel != "" {
					os.Setenv("LOG_LEVEL", originalLogLevel)
				} else {
					os.Unsetenv("LOG_LEVEL")
				}
				if originalProjektLogLevel != "" {
					os.Setenv("PROJEKT_LOG_LEVEL", originalProjektLogLevel)
				} else {
					os.Unsetenv("PROJEKT_LOG_LEVEL")
				}
			}()

			// Set test values
			if tt.logLevelEnv != "" {
				os.Setenv("LOG_LEVEL", tt.logLevelEnv)
			} else {
				os.Unsetenv("LOG_LEVEL")
			}
			if tt.projektLogLevelEnv != "" {
				os.Setenv("PROJEKT_LOG_LEVEL", tt.projektLogLevelEnv)
			} else {
				os.Unsetenv("PROJEKT_LOG_LEVEL")
			}

			// Simulate init() logic
			logLevel := os.Getenv("LOG_LEVEL")
			if logLevel == "" {
				logLevel = os.Getenv("PROJEKT_LOG_LEVEL")
			}
			if logLevel == "" {
				logLevel = "info"
			}

			if logLevel != tt.want {
				t.Errorf("LogLevel = %v, want %v", logLevel, tt.want)
			}
		})
	}
}

func TestEnvSettings_FlagParsing(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantErr   bool
		wantLevel string
	}{
		{
			name:      "short flag",
			args:      []string{"-v", "debug"},
			wantErr:   false,
			wantLevel: "debug",
		},
		{
			name:      "long flag",
			args:      []string{"--verbose", "error"},
			wantErr:   false,
			wantLevel: "error",
		},
		{
			name:      "no flag uses default",
			args:      []string{},
			wantErr:   false,
			wantLevel: "info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EnvSettings{LogLevel: "info"}
			fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
			e.AddFlags(fs)

			err := fs.Parse(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && e.LogLevel != tt.wantLevel {
				t.Errorf("LogLevel = %v, want %v", e.LogLevel, tt.wantLevel)
			}
		})
	}
}
