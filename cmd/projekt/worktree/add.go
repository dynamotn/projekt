package worktree

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
)

const addLongHelp = `Create a working tree of a project and record it.

The branch is checked out when it exists and created from the current HEAD
when it does not, so starting a branch and picking one up again are the same
command.

The working tree is created in ` + "`<project>/.worktrees/<name>`" + `, which keeps it
beside the code. Git does not ignore that folder on its own; add ` + "`.worktrees/`" + `
to the repository's .git/info/exclude to keep it out of ` + "`git status`" + `.

Examples:

  projekt worktree add myapp feature/PROJ-123   # then pj myapp@PROJ-123
  projekt worktree add myapp main --name trunk
  projekt worktree add myapp hotfix --path ../myapp-hotfix
  projekt worktree add myapp feature/x --dry-run`

func NewWorktreeAddCmd(out io.Writer) *cobra.Command {
	o := folderutil.AddWorktreeOptions{}

	cmd := &cobra.Command{
		Use:     "add [project] [branch]",
		Short:   "Create a working tree of a project",
		Long:    addLongHelp,
		Args:    cobra.ExactArgs(2),
		Aliases: []string{"a", "new"},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return compListProjects(toComplete)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Project = args[0]
			o.Branch = args[1]

			_, err := folderutil.AddWorktree(out, o)
			return err
		},
	}

	f := cmd.Flags()
	f.StringVarP(&o.Name, "name", "n", "", "Name after the '@', by default the last element of the branch")
	f.StringVar(&o.Path, "path", "", "Where to create it, absolute or relative to the project")
	f.BoolVarP(&o.DryRun, "dry-run", "d", false, "Say what would happen without creating anything")

	return cmd
}
