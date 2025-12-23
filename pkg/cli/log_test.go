package cli

import (
	"os"
	"testing"
)

func TestLevelHierarchy(t *testing.T) {
	tests := []struct {
		name   string
		level  string
		value  int
		exists bool
	}{
		{
			name:   "trace level",
			level:  TRACE,
			value:  0,
			exists: true,
		},
		{
			name:   "debug level",
			level:  DEBUG,
			value:  1,
			exists: true,
		},
		{
			name:   "info level",
			level:  INFO,
			value:  2,
			exists: true,
		},
		{
			name:   "warn level",
			level:  WARN,
			value:  3,
			exists: true,
		},
		{
			name:   "error level",
			level:  ERROR,
			value:  4,
			exists: true,
		},
		{
			name:   "fatal level",
			level:  FATAL,
			value:  5,
			exists: true,
		},
		{
			name:   "invalid level",
			level:  "invalid",
			value:  0,
			exists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, exists := levelHierarchy[tt.level]
			if exists != tt.exists {
				t.Errorf("levelHierarchy[%s] exists = %v, want %v", tt.level, exists, tt.exists)
			}
			if tt.exists && value != tt.value {
				t.Errorf("levelHierarchy[%s] = %v, want %v", tt.level, value, tt.value)
			}
		})
	}
}

func TestIsLevelEnabled(t *testing.T) {
	tests := []struct {
		name         string
		currentLevel string
		targetLevel  string
		want         bool
	}{
		{
			name:         "trace when current is trace",
			currentLevel: TRACE,
			targetLevel:  TRACE,
			want:         true,
		},
		{
			name:         "debug when current is trace",
			currentLevel: TRACE,
			targetLevel:  DEBUG,
			want:         true,
		},
		{
			name:         "trace when current is info",
			currentLevel: INFO,
			targetLevel:  TRACE,
			want:         false,
		},
		{
			name:         "info when current is info",
			currentLevel: INFO,
			targetLevel:  INFO,
			want:         true,
		},
		{
			name:         "warn when current is info",
			currentLevel: INFO,
			targetLevel:  WARN,
			want:         true,
		},
		{
			name:         "error when current is warn",
			currentLevel: WARN,
			targetLevel:  ERROR,
			want:         true,
		},
		{
			name:         "debug when current is error",
			currentLevel: ERROR,
			targetLevel:  DEBUG,
			want:         false,
		},
		{
			name:         "fatal when current is fatal",
			currentLevel: FATAL,
			targetLevel:  FATAL,
			want:         true,
		},
		{
			name:         "error when current is fatal",
			currentLevel: FATAL,
			targetLevel:  ERROR,
			want:         false,
		},
		{
			name:         "invalid target level",
			currentLevel: INFO,
			targetLevel:  "invalid",
			want:         false,
		},
		{
			name:         "invalid current level defaults to info",
			currentLevel: "invalid",
			targetLevel:  INFO,
			want:         true,
		},
		{
			name:         "invalid current level blocks debug",
			currentLevel: "invalid",
			targetLevel:  DEBUG,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original value
			originalLogLevel := os.Getenv("LOG_LEVEL")
			defer func() {
				if originalLogLevel != "" {
					os.Setenv("LOG_LEVEL", originalLogLevel)
				} else {
					os.Unsetenv("LOG_LEVEL")
				}
				// Reset logger for next test
				logger = nil
			}()

			// Set test value
			os.Setenv("LOG_LEVEL", tt.currentLevel)

			// Reset global env to pick up new value
			env = &EnvSettings{
				LogLevel: tt.currentLevel,
			}

			got := isLevelEnabled(tt.targetLevel)
			if got != tt.want {
				t.Errorf("isLevelEnabled(%s) with current level %s = %v, want %v",
					tt.targetLevel, tt.currentLevel, got, tt.want)
			}
		})
	}
}

func TestInitLogging(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
	}{
		{
			name:     "trace level",
			logLevel: TRACE,
		},
		{
			name:     "debug level",
			logLevel: DEBUG,
		},
		{
			name:     "info level",
			logLevel: INFO,
		},
		{
			name:     "warn level",
			logLevel: WARN,
		},
		{
			name:     "error level",
			logLevel: ERROR,
		},
		{
			name:     "fatal level",
			logLevel: FATAL,
		},
		{
			name:     "invalid level defaults to info",
			logLevel: "invalid",
		},
		{
			name:     "empty level defaults to info",
			logLevel: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original values
			originalLogLevel := os.Getenv("LOG_LEVEL")
			originalLogger := logger
			defer func() {
				if originalLogLevel != "" {
					os.Setenv("LOG_LEVEL", originalLogLevel)
				} else {
					os.Unsetenv("LOG_LEVEL")
				}
				logger = originalLogger
			}()

			// Set test value
			os.Setenv("LOG_LEVEL", tt.logLevel)
			env = &EnvSettings{
				LogLevel: tt.logLevel,
			}
			logger = nil

			// Should not panic
			InitLogging()

			if logger == nil {
				t.Error("InitLogging() did not initialize logger")
			}
		})
	}
}

func TestLogFunctions(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
		logFunc  func(string, ...any)
		funcName string
	}{
		{
			name:     "Trace function",
			logLevel: TRACE,
			logFunc:  Trace,
			funcName: "Trace",
		},
		{
			name:     "Debug function",
			logLevel: DEBUG,
			logFunc:  Debug,
			funcName: "Debug",
		},
		{
			name:     "Info function",
			logLevel: INFO,
			logFunc:  Info,
			funcName: "Info",
		},
		{
			name:     "Warn function",
			logLevel: WARN,
			logFunc:  Warn,
			funcName: "Warn",
		},
		{
			name:     "Error function",
			logLevel: ERROR,
			logFunc:  Error,
			funcName: "Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original values
			originalLogLevel := os.Getenv("LOG_LEVEL")
			originalLogger := logger
			defer func() {
				if originalLogLevel != "" {
					os.Setenv("LOG_LEVEL", originalLogLevel)
				} else {
					os.Unsetenv("LOG_LEVEL")
				}
				logger = originalLogger
			}()

			// Set test value
			os.Setenv("LOG_LEVEL", tt.logLevel)
			env = &EnvSettings{
				LogLevel: tt.logLevel,
			}
			logger = nil

			// Should not panic and should initialize logger if needed
			tt.logFunc("test message: %s", "value")

			if logger == nil {
				t.Errorf("%s() did not initialize logger when logger was nil", tt.funcName)
			}
		})
	}
}

func TestLogFunctionsWithDisabledLevel(t *testing.T) {
	tests := []struct {
		name         string
		currentLevel string
		logFunc      func(string, ...any)
		funcName     string
		shouldLog    bool
	}{
		{
			name:         "Trace disabled when level is INFO",
			currentLevel: INFO,
			logFunc:      Trace,
			funcName:     "Trace",
			shouldLog:    false,
		},
		{
			name:         "Debug disabled when level is INFO",
			currentLevel: INFO,
			logFunc:      Debug,
			funcName:     "Debug",
			shouldLog:    false,
		},
		{
			name:         "Info enabled when level is INFO",
			currentLevel: INFO,
			logFunc:      Info,
			funcName:     "Info",
			shouldLog:    true,
		},
		{
			name:         "Warn enabled when level is INFO",
			currentLevel: INFO,
			logFunc:      Warn,
			funcName:     "Warn",
			shouldLog:    true,
		},
		{
			name:         "Error enabled when level is INFO",
			currentLevel: INFO,
			logFunc:      Error,
			funcName:     "Error",
			shouldLog:    true,
		},
		{
			name:         "Debug disabled when level is ERROR",
			currentLevel: ERROR,
			logFunc:      Debug,
			funcName:     "Debug",
			shouldLog:    false,
		},
		{
			name:         "Info disabled when level is ERROR",
			currentLevel: ERROR,
			logFunc:      Info,
			funcName:     "Info",
			shouldLog:    false,
		},
		{
			name:         "Warn disabled when level is ERROR",
			currentLevel: ERROR,
			logFunc:      Warn,
			funcName:     "Warn",
			shouldLog:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original values
			originalLogLevel := os.Getenv("LOG_LEVEL")
			originalLogger := logger
			defer func() {
				if originalLogLevel != "" {
					os.Setenv("LOG_LEVEL", originalLogLevel)
				} else {
					os.Unsetenv("LOG_LEVEL")
				}
				logger = originalLogger
			}()

			// Set test value
			os.Setenv("LOG_LEVEL", tt.currentLevel)
			env = &EnvSettings{
				LogLevel: tt.currentLevel,
			}
			logger = nil

			// Call log function - should not panic
			tt.logFunc("test message")

			// If should log, logger should be initialized
			// If should not log, logger might still be nil
			if tt.shouldLog && logger == nil {
				t.Errorf("%s() should have initialized logger when level is enabled", tt.funcName)
			}
		})
	}
}

func TestFatal_Coverage(t *testing.T) {
	// We cannot actually test Fatal executing as it would exit the test
	// But we can test the code path and ensure it doesn't panic when level check passes
	originalLogLevel := os.Getenv("LOG_LEVEL")
	originalLogger := logger
	defer func() {
		if originalLogLevel != "" {
			os.Setenv("LOG_LEVEL", originalLogLevel)
		} else {
			os.Unsetenv("LOG_LEVEL")
		}
		logger = originalLogger
	}()

	// Test that Fatal initializes logger when nil and level is enabled
	// We set level to FATAL so isLevelEnabled returns true
	os.Setenv("LOG_LEVEL", FATAL)
	env = &EnvSettings{
		LogLevel: FATAL,
	}
	logger = nil

	// We can't actually call Fatal as it would exit
	// Instead we test isLevelEnabled for FATAL
	if !isLevelEnabled(FATAL) {
		t.Error("isLevelEnabled(FATAL) should return true when log level is FATAL")
	}

	// Verify logger would be initialized
	if logger == nil {
		InitLogging()
		if logger == nil {
			t.Error("InitLogging() should initialize logger")
		}
	}
}
