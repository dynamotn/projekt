package cli

import (
	"fmt"
	"log"
	"os"

	"github.com/fatih/color"
)

func InitLogging() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func Debug(v ...interface{}) {
	if GetEnv().Debug {
		cyan := color.New(color.FgCyan).SprintFunc()
		format := fmt.Sprintf(cyan("[debug] %s\n"), "%+v")
		log.Output(2, fmt.Sprintf(format, v...))
	}
}

func Warning(message string, v ...interface{}) {
	yellow := color.New(color.FgYellow).SprintFunc()
	format := fmt.Sprintf(yellow("WARNING: %s\n%s\n"), message, "%+v")
	fmt.Fprintf(os.Stderr, format, v...)
}

func Error(message string, v ...interface{}) {
	format := fmt.Sprintf("ERROR: %s\n%s\n", message, "%+v")
	log.Fatalf(format, v...)
}
