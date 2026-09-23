package worktree

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/folderutil"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

const removeLongHelp = `Remove a working tree and forget about it.

The branch is left alone. Putting away the folder it was checked out in and
deleting the work are two different decisions, and only one of them is this
command's.

Git refuses to remove a working tree with changes in it; --force says to do it
anyway. --keep leaves the files where they are and only drops the entry, which
is what a working tree already removed by hand needs.

Examples:

  projekt worktree remove myapp@PROJ-123
  projekt worktree remove myapp@PROJ-123 --force
  projekt worktree remove myapp@PROJ-123 --keep`

func NewWorktreeRemoveCmd(out io.Writer) *cobra.Command {
	o := folderutil.RemoveWorktreeOptions{}

	cmd := &cobra.Command{
		Use:     "remove [project@name]",
		Short:   "Remove a working tree of a project",
		Long:    removeLongHelp,
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"rm", "delete"},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return compListWorktrees(toComplete)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			project, name, ok := lazypath.SplitWorktreeName(args[0])
			if !ok {
				return fmt.Errorf("%q is not a worktree name, expected project%sname", args[0], lazypath.WorktreeSeparator)
			}
			o.Project, o.Name = project, name

			return folderutil.RemoveWorktree(out, o)
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&o.Force, "force", "F", false, "Remove it even when it has changes in it")
	f.BoolVar(&o.Keep, "keep", false, "Leave the files alone, only drop the config entry")

	return cmd
}
