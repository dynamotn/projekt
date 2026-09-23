// Package tplutil renders Go templates from a user owned template store.
//
// A template is either one single file or a whole folder tree living in
// $XDG_DATA_HOME/projekt/templates. Both the file contents and, for a folder
// template, every path segment go through text/template with the sprig
// function set, so a template can name the files it creates.
//
// Example usage:
//
//	tpl, err := tplutil.Get("license")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	written, err := tplutil.Render(tplutil.RenderOptions{
//	    Template: tpl,
//	    Dest:     "LICENSE",
//	    Values:   tplutil.Values{"author": "me"},
//	})
package tplutil

import (
	"io"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

// ListOption contains options for listing templates.
type ListOption struct {
	// NamesOnly keeps the name column alone, the listing a script wants.
	NamesOnly bool
	cli.ListOutputOption
}

// ListTemplates displays the template store in the requested format.
func ListTemplates(out io.Writer, o *ListOption) error {
	templates, err := List()
	if err != nil {
		return err
	}

	if len(templates) == 0 && o.Output != cli.OutputJSON {
		dir, err := Dir()
		if err != nil {
			return err
		}
		// An empty store is a normal first run, not a failure: say where to put
		// templates instead of printing an empty table. JSON is read by a
		// program, which wants the empty array rather than a message.
		cli.Warn("No template found in %s", dir)
		return nil
	}

	view := cli.ListView{Columns: []cli.ListColumn{
		{Header: "NAME", Key: "name"},
		{Header: "KIND", Key: "kind"},
		{Header: "PATH", Key: "path"},
	}}
	if o.NamesOnly {
		view.Columns = view.Columns[:1]
	}
	for _, tpl := range templates {
		if o.NamesOnly {
			view.AppendRow(tpl.Name)
			continue
		}
		view.AppendRow(tpl.Name, string(tpl.Kind), tpl.Path)
	}

	return cli.EncodeList(out, view, o.ListOutputOption)
}
