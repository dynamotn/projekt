package folder

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
)

const tagLongHelp = `Change the tags on a project.

Tags could only be set when a folder was added, which meant editing YAML to
change one — and the point of tags is that they change.

  projekt folder tag myapp go work     # add these
  projekt folder tag myapp -r work     # take this one away
  projekt folder tag myapp go -r old   # both at once
  projekt folder tag                   # what is in use, and how much

The result is sorted, so the configuration file does not churn on every edit.

A project found inside a workspace has no entry of its own and carries the
workspace's tags: tag the workspace.`

func NewFolderTagCmd(out io.Writer) *cobra.Command {
	var remove []string

	cmd := &cobra.Command{
		Use:     "tag [short name] [tags...]",
		Short:   "Change the tags on a project",
		Long:    tagLongHelp,
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{"tags"},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return compListShortNames(toComplete)
			}
			return compListTags(toComplete)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return folderutil.ListTags(out)
			}
			return folderutil.TagFolder(out, args[0], args[1:], remove)
		},
	}

	cmd.Flags().StringSliceVarP(&remove, "remove", "r", nil, "Tags to take away")

	if err := cmd.RegisterFlagCompletionFunc("remove",
		func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return compListTags(toComplete)
		}); err != nil {
		return cmd
	}

	return cmd
}
