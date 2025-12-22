package cli

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.SugaredLogger

// Log levels following log4j hierarchy
const (
	TRACE = "trace"
	DEBUG = "debug"
	INFO  = "info"
	WARN  = "warn"
	ERROR = "error"
	FATAL = "fatal"
)

// levelHierarchy maps log levels to numeric values for comparison
var levelHierarchy = map[string]int{
	TRACE: 0,
	DEBUG: 1,
	INFO:  2,
	WARN:  3,
	ERROR: 4,
	FATAL: 5,
}

func InitLogging() {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	// Set log level based on environment
	level := strings.ToLower(GetEnv().LogLevel)
	switch level {
	case TRACE, DEBUG:
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case INFO:
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case WARN:
		config.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case ERROR:
		config.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	case FATAL:
		config.Level = zap.NewAtomicLevelAt(zap.FatalLevel)
	default:
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	var err error
	var builder *zap.Logger
	builder, err = config.Build(zap.AddCallerSkip(1))
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}
	logger = builder.Sugar()
}

// isLevelEnabled checks if the given level should be logged based on current log level
func isLevelEnabled(level string) bool {
	currentLevel := strings.ToLower(GetEnv().LogLevel)
	currentLevelValue, exists := levelHierarchy[currentLevel]
	if !exists {
		currentLevelValue = levelHierarchy[INFO] // default to INFO
	}

	targetLevelValue, exists := levelHierarchy[level]
	if !exists {
		return false
	}

	return targetLevelValue >= currentLevelValue
}

// Trace logs formatted message at TRACE level
func Trace(format string, v ...any) {
	if isLevelEnabled(TRACE) {
		if logger == nil {
			InitLogging()
		}
		logger.Debugf(format, v...)
	}
}

// Debug logs formatted message at DEBUG level
func Debug(format string, v ...any) {
	if isLevelEnabled(DEBUG) {
		if logger == nil {
			InitLogging()
		}
		logger.Debugf(format, v...)
	}
}

// Info logs formatted message at INFO level
func Info(format string, v ...any) {
	if isLevelEnabled(INFO) {
		if logger == nil {
			InitLogging()
		}
		logger.Infof(format, v...)
	}
}

// Warn logs formatted message at WARN level
func Warn(format string, v ...any) {
	if isLevelEnabled(WARN) {
		if logger == nil {
			InitLogging()
		}
		logger.Warnf(format, v...)
	}
}

// Error logs formatted message at ERROR level
func Error(format string, v ...any) {
	if isLevelEnabled(ERROR) {
		if logger == nil {
			InitLogging()
		}
		logger.Errorf(format, v...)
	}
}

// Fatal logs formatted message at FATAL level and exit
func Fatal(format string, v ...any) {
	if isLevelEnabled(FATAL) {
		if logger == nil {
			InitLogging()
		}
		logger.Fatalf(format, v...)
	}
}
