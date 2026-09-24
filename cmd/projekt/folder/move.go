package folder

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/folderutil"
)

const moveLongHelp = `Move a project, and keep its configuration.

` + "`folder remove`" + ` and ` + "`folder add`" + ` lose the prefix, the tags and the priority
along the way, which is most of what the entry was for.

  projekt folder move myapp ~/work/myapp --dry-run
  projekt folder move myapp ~/work/myapp

Both halves of the job are this one command, because both happen: when the
folder is still at the old path it is moved, and when it has already been moved
by hand only the configuration catches up. The files move first, so a move that
fails leaves a configuration that is still true.

Working trees inside the project follow it.

A project found inside a workspace has no entry of its own — move the folder
and the workspace finds it where it is now.`

func NewFolderMoveCmd(out io.Writer) *cobra.Command {
	o := folderutil.MoveOptions{}

	cmd := &cobra.Command{
		Use:     "move [short name] [new path]",
		Short:   "Move a project and keep its configuration",
		Long:    moveLongHelp,
		Args:    cobra.ExactArgs(2),
		Aliases: []string{"mv", "rename"},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveDefault
			}
			return compListShortNames(toComplete)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return folderutil.MoveFolder(out, args[0], args[1], o)
		},
	}

	cmd.Flags().BoolVar(&o.DryRun, "dry-run", false, "Say what would happen without moving anything")

	return cmd
}
