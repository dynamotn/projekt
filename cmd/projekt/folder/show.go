package folder

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/showutil"
)

const showLongHelp = `Show everything projekt knows about one project.

Path, tags, description, the whole note, the working trees, the git state,
the hooks a jump runs and the templates the project was rendered from, in one
place. Without a name it is the project the current folder is in. With
'-o json' it is one object, with the empty parts left out.

  projekt folder show
  projekt folder show billing
  projekt folder show billing -o json | jq .note`

func NewFolderShowCmd(out io.Writer) *cobra.Command {
	o := &showutil.Options{}
	output := string(cli.OutputTable)

	cmd := &cobra.Command{
		Use:               "show [short name]",
		Short:             "Show everything projekt knows about one project",
		Long:              showLongHelp,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeOneShortName,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := cli.ParseOutputFormat(output)
			if err != nil {
				return err
			}
			o.Output = format

			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			return showutil.Show(out, name, *o)
		},
	}

	f := cmd.Flags()
	f.BoolVar(&o.NoStatus, "no-status", false, "Skip the git state")
	f.BoolVar(&o.NoHeaders, "no-headers", false, "Don't print headers")
	f.BoolVar(&o.NoColor, "no-color", false, "Don't use color")
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

// completeOneShortName offers names for the one argument.
func completeOneShortName(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return compListShortNames(toComplete)
}
