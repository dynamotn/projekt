package root

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/tplutil"
)

func NewTemplateListCmd(out io.Writer) *cobra.Command {
	o := &tplutil.ListOption{}
	output := string(cli.OutputTable)

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all templates of your template folder",
		Args:    cobra.NoArgs,
		Aliases: []string{"l", "ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := cli.ParseOutputFormat(output)
			if err != nil {
				return err
			}
			o.Output = format

			return tplutil.ListTemplates(out, o)
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&o.NamesOnly, "names-only", "n", false, "Show only the template names")
	f.BoolVarP(&o.NoHeaders, "no-headers", "", false, "Don't print headers")
	f.BoolVarP(&o.NoColor, "no-color", "", false, "Don't use color")
	f.StringVarP(&output, "output", "o", output,
		fmt.Sprintf("Output format, one of: %s", strings.Join(cli.OutputFormats, ", ")))

	if err := cmd.RegisterFlagCompletionFunc("output",
		func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return cli.OutputFormats, cobra.ShellCompDirectiveNoFileComp
		}); err != nil {
		cli.Warn("Cannot register completion for --output: %v", err)
	}

	return cmd
}
