package root

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/folderutil"
)

func NewFolderGetCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [short name]",
		Short: "Get project folder by short name",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return compListShortNames(toComplete)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return folderutil.FindFolderByShortName(out, args[0])
		},
	}

	return cmd
}

func NewFastPjCmd(out io.Writer) *cobra.Command {
	cmd := NewFolderGetCmd(out)

	cmd.Use = "pj"
	cmd.SilenceUsage = true

	cli.SetColorAndStyles(cmd)

	return cmd
}
