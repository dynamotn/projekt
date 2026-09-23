package root

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/tplutil"
)

func NewTemplateAddCmd(out io.Writer) *cobra.Command {
	o := tplutil.AddOptions{}

	cmd := &cobra.Command{
		Use:     "add [file or folder]",
		Short:   "Save an existing file or folder as a template",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"a", "import"},
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Source = args[0]
			path, err := tplutil.Add(o)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(out, path)
			return err
		},
	}

	f := cmd.Flags()
	f.StringVarP(&o.Name, "name", "n", "", "Name of the template, defaults to the source name")
	f.BoolVarP(&o.Force, "force", "F", false, "Overwrite a template that already exists")

	return cmd
}
