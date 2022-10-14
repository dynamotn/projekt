package root

import (
	"io"
	"strings"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/folderutil"
)

func NewFolderRemoveCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove [folder path]",
		Short:   "Remove your project folder to config",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"rm"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return folderutil.RemoveFolderFromConfig(strings.TrimRight(args[0], "/"))
		},
	}

	return cmd
}
