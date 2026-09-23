package folder

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/folderutil"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

const recentLongHelp = `List the projects in the order you last jumped to them.

Every jump is remembered, one entry per project, in
$XDG_STATE_HOME/projekt/history.tsv. That is also what makes ` + "`pj -`" + ` work:
it takes you back to the project you were in before this one, and typing it
again brings you back, the way ` + "`cd -`" + ` does.

  pj -                               # back where you were
  projekt folder recent              # what you have been working on
  projekt folder recent --limit 5
  projekt folder recent --clear      # forget all of it`

func NewFolderRecentCmd(out io.Writer) *cobra.Command {
	o := &folderutil.RecentOptions{}
	output := string(cli.OutputTable)
	clear := false

	cmd := &cobra.Command{
		Use:     "recent",
		Short:   "List the projects you jumped to, most recent first",
		Long:    recentLongHelp,
		Args:    cobra.NoArgs,
		Aliases: []string{"r", "history"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if clear {
				if err := lazypath.ClearHistory(); err != nil {
					return err
				}
				_, err := fmt.Fprintln(out, "Jump history cleared")
				return err
			}

			format, err := cli.ParseOutputFormat(output)
			if err != nil {
				return err
			}
			o.Output = format

			return folderutil.ListRecent(out, o)
		},
	}

	f := cmd.Flags()
	f.IntVarP(&o.Limit, "limit", "l", 0, "Show only this many, most recent first")
	f.BoolVarP(&o.NamesOnly, "names-only", "n", false, "Show only the names")
	f.BoolVar(&clear, "clear", false, "Forget every jump")
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
