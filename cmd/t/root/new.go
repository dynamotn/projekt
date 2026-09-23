package root

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/tplutil"
)

const newLongHelp = `Render a Go template from your template folder.

A template is either a single file or a whole folder of files. Both the file
contents and, for a folder template, every path segment go through
text/template with the sprig function set, so a template can name the files it
creates.

Variables available to a template:

  .Name      Output name, from --name or the destination file name
  .Project   Folder the files are written to, or .Name for a folder template
  .Dir       Absolute destination folder
  .Path      Absolute path of the file being rendered
  .Template  Name of the template
  .User      Current user name
  .Now       Current time, .Date (2006-01-02) and .Year
  .Values    Everything given with --set and --values

Examples:

  t new license LICENSE --set author='Jane Doe'
  t new go-cli ./myapp --set module=example.com/myapp
  t new dockerfile --dry-run`

func NewTemplateNewCmd(out io.Writer) *cobra.Command {
	var (
		sets       []string
		valueFiles []string
	)
	o := tplutil.RenderOptions{Out: out}

	cmd := &cobra.Command{
		Use:               "new [template] [destination]",
		Short:             "Create files from a template",
		Long:              newLongHelp,
		Args:              cobra.RangeArgs(1, 2),
		Aliases:           []string{"n", "create", "gen"},
		ValidArgsFunction: completeTemplateNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			tpl, err := tplutil.Get(args[0])
			if err != nil {
				return err
			}
			o.Template = tpl
			if len(args) > 1 {
				o.Dest = args[1]
			}

			fileValues, err := tplutil.LoadValuesFiles(valueFiles)
			if err != nil {
				return err
			}
			setValues, err := tplutil.ParseSet(sets)
			if err != nil {
				return err
			}
			// --set wins over --values, the same way a flag wins over a file.
			o.Values = tplutil.MergeValues(fileValues, setValues)

			written, err := tplutil.Render(o)
			if err != nil {
				return err
			}
			if o.DryRun {
				return nil
			}
			for _, path := range written {
				if _, err := fmt.Fprintln(out, path); err != nil {
					return err
				}
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVarP(&o.Name, "name", "n", "", "Name of the rendered file, also available as '.Name'")
	f.StringArrayVarP(&sets, "set", "s", nil, "Set a template value, like -s key=value or -s author.name=me (repeatable)")
	f.StringArrayVarP(&valueFiles, "values", "f", nil, "YAML file of template values (repeatable)")
	f.BoolVarP(&o.Force, "force", "F", false, "Overwrite files that already exist")
	f.BoolVarP(&o.DryRun, "dry-run", "d", false, "Print the rendered result instead of writing files")

	return cmd
}
