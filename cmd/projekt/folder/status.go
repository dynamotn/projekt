package folder

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/folderutil"
)

const statusLongHelp = `Show what every project folder looks like right now.

The Friday afternoon question: what did I leave half-done, and where. One row
per project, with the branch, how many files git would mention, how far the
branch is from its upstream, how many stashes are waiting and how long ago the
last commit was.

  projekt folder status              # everything
  projekt folder status --dirty      # only what wants attention
  projekt folder status -t work
  projekt folder status -o json | jq -r '.[] | select(.behind > 0) | .name'

Unlike 'projekt folder check', which verifies the repositories your config
declares, this looks at every folder it can reach — including the ones inside a
workspace, which have no config entry of their own, and your working trees.`

func NewFolderStatusCmd(out io.Writer) *cobra.Command {
	o := &folderutil.StatusOptions{}
	output := string(cli.OutputTable)

	cmd := &cobra.Command{
		Use:     "status",
		Short:   "Show the git state of every project folder",
		Long:    statusLongHelp,
		Args:    cobra.NoArgs,
		Aliases: []string{"st"},
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := cli.ParseOutputFormat(output)
			if err != nil {
				return err
			}
			o.Output = format

			return folderutil.StatusFolders(out, o)
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&o.DirtyOnly, "dirty", "d", false, "Show only the folders with something to attend to")
	f.BoolVarP(&o.NamesOnly, "names-only", "n", false, "Show only the names")
	f.IntVar(&o.Forks, "forks", folderutil.DefaultSyncForks, "How many folders to read at once")
	f.BoolVar(&o.NoHeaders, "no-headers", false, "Don't print headers")
	f.BoolVar(&o.NoColor, "no-color", false, "Don't use color")
	f.StringVarP(&output, "output", "o", output,
		fmt.Sprintf("Output format, one of: %s", strings.Join(cli.OutputFormats, ", ")))
	registerTagsFlag(cmd, &o.Tags, "report on")

	if err := cmd.RegisterFlagCompletionFunc("output",
		func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return cli.OutputFormats, cobra.ShellCompDirectiveNoFileComp
		}); err != nil {
		cli.Warn("Cannot register completion for --output: %v", err)
	}

	return cmd
}
