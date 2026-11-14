package root

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/bplutil"
	"gitlab.com/dynamo-tools/projekt/pkg/cli"
)

func NewBoilerplateListCmd(out io.Writer) *cobra.Command {
	o := &bplutil.ListOption{}
	output := string(cli.OutputTable)

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all boilerplates of your boilerplate folder",
		Args:    cobra.NoArgs,
		Aliases: []string{"l", "ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := cli.ParseOutputFormat(output)
			if err != nil {
				return err
			}
			o.Output = format

			return bplutil.ListRecipes(out, o)
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&o.NamesOnly, "names-only", "n", false, "Show only the boilerplate names")
	f.BoolVarP(&o.NoHeaders, "no-headers", "", false, "Don't print headers")
	f.BoolVarP(&o.NoColor, "no-color", "", false, "Don't use color")
	cli.AddOutputFlag(cmd, &output, output)

	return cmd
}
