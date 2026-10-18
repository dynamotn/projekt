package templates

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"
)

//go:embed files/*
var f embed.FS

// Shells returns the shells an integration script is shipped for, sorted.
//
// It reads the embedded files rather than a second list, so that adding a
// shell is adding one file.
func Shells() []string {
	entries, err := fs.ReadDir(f, "files")
	if err != nil {
		// The files are embedded at build time; a failure here cannot happen
		// at run time, and an empty list would only hide it.
		panic(fmt.Sprintf("cannot read the embedded shell scripts: %v", err))
	}

	shells := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}
		shells = append(shells, strings.TrimSuffix(entry.Name(), ".txt"))
	}
	sort.Strings(shells)
	return shells
}

// GenCommands writes the integration script of a shell.
//
// The scripts are written as they are: nothing is interpolated into them,
// since every path is resolved by `projekt` itself when the function runs,
// and a shell script has no business being parsed as a Go template.
func GenCommands(shell string, w io.Writer) error {
	data, err := f.ReadFile("files/" + shell + ".txt")
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
