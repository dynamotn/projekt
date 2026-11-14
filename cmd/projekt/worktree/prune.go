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

With ` + "`--merged`" + ` it also removes the working trees whose branch is merged into
the project's default branch and that hold no change and no untracked or ignored file.
The branch is kept, and git itself refuses a tree it finds dirty. Only the
trees of your own configuration file go; every tree left alone is named with
the reason. A project whose default branch cannot be found keeps all of its
trees.

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

	cmd.Flags().BoolVar(&o.Merged, "merged", false, "Also remove the clean working trees whose branch is merged into the default branch")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", false, "Say what would happen without changing anything")

	return cmd
}
