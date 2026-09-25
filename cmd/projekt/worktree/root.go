// Package worktree holds the `projekt worktree` commands, which create and
// keep track of the extra working trees of a project.
package worktree

import (
	"io"
	"sort"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

const worktreeLongHelp = `Work on two branches of a project at once.

A working tree is another checkout of the same repository, on another branch.
` + "`projekt worktree add`" + ` creates one and records it, so that it is reachable by
name like any other project:

  projekt worktree add myapp feature/PROJ-123
  pj myapp@PROJ-123

The name after the "@" comes from the last element of the branch, because that
is the part that tells two branches apart, and it is what you have to type.
Working trees live in ` + "`<project>/.worktrees/`" + ` unless --path says otherwise.

Nothing in the shell integration knows about working trees: they resolve
through the same short names as everything else, so ` + "`pj`" + ` and its completion
pick them up on their own.`

func NewWorktreeCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "worktree",
		Short:   "Work on two branches of a project at once",
		Long:    worktreeLongHelp,
		Aliases: []string{"wt", "w"},
	}

	cmd.AddCommand(
		NewWorktreeAddCmd(out),
		NewWorktreeListCmd(out),
		NewWorktreeRemoveCmd(out),
		NewWorktreePruneCmd(out),
	)

	cli.SetColorAndStyles(cmd)
	return cmd
}

// compListProjects completes a project: the folders, without the working trees
// themselves, since a working tree does not have working trees of its own.
func compListProjects(_ string) ([]string, cobra.ShellCompDirective) {
	parsed, err := folderutil.ParseConfig(lazypath.GetConfig())
	if err != nil {
		cli.Error("Can't parse config: %v", err)
		return nil, cobra.ShellCompDirectiveError
	}

	var result []string
	for _, folder := range parsed {
		if _, _, isWorktree := lazypath.SplitWorktreeName(folder.ShortName); isWorktree {
			continue
		}
		result = append(result, folder.ShortName)
	}

	return result, cobra.ShellCompDirectiveNoFileComp
}

// compListWorktrees completes a working tree by the name it is reached by.
func compListWorktrees(_ string) ([]string, cobra.ShellCompDirective) {
	var result []string
	for _, worktree := range lazypath.GetConfig().Worktrees {
		result = append(result, worktree.ShortName())
	}
	sort.Strings(result)

	return result, cobra.ShellCompDirectiveNoFileComp
}
