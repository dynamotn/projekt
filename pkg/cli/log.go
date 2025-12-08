// Package cli provides command-line interface utilities.
//
// It includes logging, environment settings, output formatting, and version display
// functionality for CLI applications.
//
// Example usage:
//
//	cli.InitLogging()
//	cli.Debug("Debugging information")
//	cli.Info("Informational message")
//	cli.Warning("Warning message", err)
//	cli.Error("Fatal error", err) // This will exit
package cli

import (
	"fmt"
	"log"
	"os"

	"github.com/fatih/color"
)

// InitLogging initializes the logging configuration
func InitLogging() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

// Debug logs debug level messages
func Debug(v ...interface{}) {
	if GetEnv().IsDebug() {
		cyan := color.New(color.FgCyan).SprintFunc()
		format := fmt.Sprintf(cyan("[debug] %s\n"), "%+v")
		log.Output(2, fmt.Sprintf(format, v...))
	}
}

// Info logs informational messages
func Info(message string, v ...interface{}) {
	if GetEnv().IsInfoOrAbove() {
		blue := color.New(color.FgBlue).SprintFunc()
		format := fmt.Sprintf(blue("INFO: %s\n%s\n"), message, "%+v")
		fmt.Fprintf(os.Stdout, format, v...)
	}
}

// Warning logs warning level messages
func Warning(message string, v ...interface{}) {
	if GetEnv().IsInfoOrAbove() {
		yellow := color.New(color.FgYellow).SprintFunc()
		format := fmt.Sprintf(yellow("WARNING: %s\n%s\n"), message, "%+v")
		fmt.Fprintf(os.Stderr, format, v...)
	}
}

// Error logs error messages and exits the program
func Error(message string, v ...interface{}) {
	format := fmt.Sprintf("ERROR: %s\n%s\n", message, "%+v")
	log.Fatalf(format, v...)
}
