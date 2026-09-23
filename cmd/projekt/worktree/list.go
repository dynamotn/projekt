package worktree

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/folderutil"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

const listLongHelp = `List the working trees, and what git makes of them.

The status column compares the configuration with the repository:

  ok              the working tree is there, on the branch it says
  MISSING         the folder is gone; remove it with --keep
  NOT A WORKTREE  the folder is there, but git does not count it as one
  BRANCH DRIFTED  it is checked out on another branch than the one recorded
  NO PROJECT      the project it hangs off no longer resolves

Reading that costs one git call per project; --no-status skips it.`

func NewWorktreeListCmd(out io.Writer) *cobra.Command {
	o := &folderutil.ListWorktreeOption{}
	output := string(cli.OutputTable)

	cmd := &cobra.Command{
		Use:     "list [project]",
		Short:   "List the working trees of your projects",
		Long:    listLongHelp,
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"l", "ls"},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return compListProjects(toComplete)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := cli.ParseOutputFormat(output)
			if err != nil {
				return err
			}
			o.Output = format
			if len(args) == 1 {
				o.Project = args[0]
			}

			return folderutil.ListWorktrees(out, o)
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&o.NamesOnly, "names-only", "n", false, "Show only the names")
	f.BoolVar(&o.NoStatus, "no-status", false, "Don't ask git about each working tree")
	f.BoolVar(&o.NoHeaders, "no-headers", false, "Don't print headers")
	f.BoolVar(&o.NoColor, "no-color", false, "Don't use color")
	f.StringVarP(&output, "output", "o", output,
		fmt.Sprintf("Output format, one of: %s", strings.Join(cli.OutputFormats, ", ")))
	f.StringSliceVarP(&o.Tags, "tags", "t", nil,
		"Only list working trees whose project carries all of these tags")

	if err := cmd.RegisterFlagCompletionFunc("output",
		func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return cli.OutputFormats, cobra.ShellCompDirectiveNoFileComp
		}); err != nil {
		cli.Warn("Cannot register completion for --output: %v", err)
	}
	if err := cmd.RegisterFlagCompletionFunc("tags",
		func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return compListTags(), cobra.ShellCompDirectiveNoFileComp
		}); err != nil {
		cli.Warn("Cannot register completion for --tags: %v", err)
	}

	return cmd
}

// compListTags completes a --tags value with the tags already in the config.
func compListTags() []string {
	seen := map[string]struct{}{}

	var result []string
	for _, folder := range lazypath.GetConfig().Folders {
		for _, tag := range folder.GetTags() {
			if _, dup := seen[tag]; dup {
				continue
			}
			seen[tag] = struct{}{}
			result = append(result, tag)
		}
	}
	return result
}
