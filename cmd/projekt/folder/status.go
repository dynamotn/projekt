package folder

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
)

const statusLongHelp = `Show what every project folder looks like right now.

The Friday afternoon question: what did I leave half-done, and where. One row
per project, with the branch, how many files git would mention, how far the
branch is from its upstream, how many stashes are waiting and how long ago the
last commit was.

  projekt folder status              # everything
  projekt folder status --dirty      # only what wants attention
  projekt folder status --notes      # with the first line of each note
  projekt folder status --since 7d   # what I touched this week
  projekt folder status --branches   # how far each branch is from main
  projekt folder status --requests   # open MRs/PRs, through glab or gh
  projekt folder status -t work
  projekt folder status -o json | jq -r '.[] | select(.behind > 0) | .name'

--since keeps the folders with a commit, or a jump, that recent: 12h, 7d, 2w,
3mo. --branches adds the default branch, how many of its commits the
checked-out branch has not got, and how long ago the two parted. --requests
asks glab or gh — whichever the origin calls for — how many merge or pull
requests the branch has open; it is the one column that goes to the network,
so it is off unless asked for, and reads n/a without the tool.

Unlike 'projekt folder check', which verifies the repositories your config
declares, this looks at every folder it can reach — including the ones inside a
workspace, which have no config entry of their own, and your working trees.`

func NewFolderStatusCmd(out io.Writer) *cobra.Command {
	o := &folderutil.StatusOptions{}
	output := string(cli.OutputTable)
	since := ""

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
			if o.Since, err = folderutil.ParseSince(since); err != nil {
				return err
			}

			return folderutil.StatusFolders(out, o)
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&o.DirtyOnly, "dirty", "d", false, "Show only the folders with something to attend to")
	f.BoolVarP(&o.NamesOnly, "names-only", "n", false, "Show only the names")
	f.BoolVar(&o.Notes, "notes", false, "Add the first line of each project's note (see folder note)")
	f.StringVar(&since, "since", "", "Keep only the folders with a commit or a jump this recent: 12h, 7d, 2w, 3mo")
	f.BoolVar(&o.Branches, "branches", false, "Add the default branch, how far behind it the branch is, and how old the branch is")
	f.BoolVar(&o.Requests, "requests", false, "Add the open merge or pull requests of the branch, counted through glab or gh")
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
