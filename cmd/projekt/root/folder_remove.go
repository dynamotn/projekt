package root

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/folderutil"
)

func NewFolderRemoveCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove [folder path]",
		Short: "Remove your project folder to config",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return folderutil.RemoveFolderFromConfig(args[0])
		},
	}

	return cmd
}
