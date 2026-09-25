package folder

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

func NewFolderRemoveCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove [folder path]",
		Short:   "Remove your project folder to config",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"rm"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Normalize the same way `add` does, so a folder added as "~/code"
			// can be removed as "./code" or "/home/me/code/".
			path, err := lazypath.NormalizePath(args[0])
			if err != nil {
				return err
			}
			return folderutil.RemoveFolderFromConfig(path)
		},
	}

	return cmd
}
