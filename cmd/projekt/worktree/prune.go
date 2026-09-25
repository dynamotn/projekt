package worktree

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
)

const pruneLongHelp = `Forget the working trees whose folder is gone.

A working tree removed with ` + "`rm -rf`" + ` leaves two things behind: git still
lists it until ` + "`git worktree prune`" + ` runs, and projekt still offers the name.
Neither half is much use without the other.

  projekt worktree prune --dry-run
  projekt worktree prune

` + "`projekt folder prune`" + ` drops the configuration entry too, but only this one
tells git about it.`

func NewWorktreePruneCmd(out io.Writer) *cobra.Command {
	o := folderutil.PruneWorktreeOptions{}

	cmd := &cobra.Command{
		Use:     "prune",
		Short:   "Forget the working trees whose folder is gone",
		Long:    pruneLongHelp,
		Args:    cobra.NoArgs,
		Aliases: []string{"clean"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return folderutil.PruneWorktrees(out, o)
		},
	}

	cmd.Flags().BoolVar(&o.DryRun, "dry-run", false, "Say what would happen without changing anything")

	return cmd
}
