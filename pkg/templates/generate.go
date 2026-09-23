package templates

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
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

func GenCommands(shell string, w io.Writer) error {
	data, error := f.ReadFile("files/" + shell + ".txt")
	if error != nil {
		return error
	}

	vars := map[string]interface{}{}

	t := template.Must(template.New(shell).Funcs(sprig.FuncMap()).Parse(string(data)))
	error = t.Execute(w, vars)
	return error
}
