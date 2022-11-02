package templates

import (
	"embed"
	"io"
	"text/template"

	"github.com/Masterminds/sprig"
)

//go:embed files/*
var f embed.FS

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
