package folder

import (
	"io"
	"strings"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
)

const describeLongHelp = `Say what a project is, in one line.

A short name says where a project is, not what it is. The description is kept
in the configuration, shown by 'folder list', offered beside the name when
you press tab after 'pj', and drawn next to the candidates 'pj' asks you to
choose from.

  projekt folder describe billing "The invoicing API, Go, owned by payments"
  projekt folder describe billing              # print it
  projekt folder describe billing --clear      # take it away

A project found inside a workspace has no entry of its own and carries the
workspace's description: describe the workspace.`

func NewFolderDescribeCmd(out io.Writer) *cobra.Command {
	var clear bool

	cmd := &cobra.Command{
		Use:     "describe <short name> [description...]",
		Short:   "Set or print the one-line description of a project",
		Long:    describeLongHelp,
		Args:    cobra.MinimumNArgs(1),
		Aliases: []string{"desc"},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return compListShortNames(toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return folderutil.DescribeFolder(out, args[0], strings.Join(args[1:], " "), clear)
		},
	}

	cmd.Flags().BoolVar(&clear, "clear", false, "Remove the description")

	return cmd
}
