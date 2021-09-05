package cli

import (
	"fmt"
	"log"
	"os"
)

func InitLogging() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func Debug(v ...interface{}) {
	if GetEnv().Debug {
		format := fmt.Sprintf("[debug] %s\n", "%+v")
		log.Output(2, fmt.Sprintf(format, v...))
	}
}

func Warning(v ...interface{}) {
	format := fmt.Sprintf("WARNING: %s\n", "%+v")
	fmt.Fprintf(os.Stderr, format, v...)
}

func Error(message string, v ...interface{}) {
	format := fmt.Sprintf("ERROR: %s\n%s\n", message, "%+v")
	log.Fatalf(format, v...)
}
