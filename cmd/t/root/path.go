package root

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/tplutil"
)

func NewTemplatePathCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "path [template]",
		Short:   "Print the path of the template folder, or of one template",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"p", "where"},
		// The path is what an editor is pointed at, so it is printed alone and
		// stays usable as `cd (t path)`.
		ValidArgsFunction: completeTemplateNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				tpl, err := tplutil.Get(args[0])
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(out, tpl.Path)
				return err
			}

			dir, err := tplutil.Dir()
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(out, dir)
			return err
		},
	}

	return cmd
}
