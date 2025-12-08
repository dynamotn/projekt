package cli

import (
	"testing"
)

func TestEnvSettings_IsDebug(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
		want     bool
	}{
		{
			name:     "debug level",
			logLevel: LogLevelDebug,
			want:     true,
		},
		{
			name:     "info level",
			logLevel: LogLevelInfo,
			want:     false,
		},
		{
			name:     "error level",
			logLevel: LogLevelError,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EnvSettings{
				LogLevel: tt.logLevel,
			}
			if got := e.IsDebug(); got != tt.want {
				t.Errorf("EnvSettings.IsDebug() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnvSettings_IsInfoOrAbove(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
		want     bool
	}{
		{
			name:     "debug level",
			logLevel: LogLevelDebug,
			want:     true,
		},
		{
			name:     "info level",
			logLevel: LogLevelInfo,
			want:     true,
		},
		{
			name:     "warn level",
			logLevel: LogLevelWarn,
			want:     true,
		},
		{
			name:     "error level",
			logLevel: LogLevelError,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EnvSettings{
				LogLevel: tt.logLevel,
			}
			if got := e.IsInfoOrAbove(); got != tt.want {
				t.Errorf("EnvSettings.IsInfoOrAbove() = %v, want %v", got, tt.want)
			}
		})
	}
}
