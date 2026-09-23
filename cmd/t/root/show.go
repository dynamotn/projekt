package root

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/tplutil"
)

func NewTemplateShowCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "show [template]",
		Short:             "Print the source of a template",
		Args:              cobra.ExactArgs(1),
		Aliases:           []string{"s", "cat"},
		ValidArgsFunction: completeTemplateNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			tpl, err := tplutil.Get(args[0])
			if err != nil {
				return err
			}
			return tplutil.ShowTemplate(out, tpl)
		},
	}

	return cmd
}
