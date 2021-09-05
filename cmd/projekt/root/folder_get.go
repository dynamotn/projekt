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
		RunE: func(cmd *cobra.Command, args []string) error {
			return folderutil.FindFolderByShortName(out, args[0])
		},
	}

	//TODO: Add completion of list folder

	return cmd
}

func NewFastPjCmd(out io.Writer) *cobra.Command {
	cmd := NewFolderGetCmd(out)

	cmd.Use = "pj"
	cmd.SilenceUsage = true

	cli.SetColorAndStyles(cmd)

	return cmd
}
