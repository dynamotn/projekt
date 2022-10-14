package root

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

func NewFolderCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "folder",
		Aliases: []string{"f", "fd", "fol"},
		Short:   "Manage your project folder",
	}

	cmd.AddCommand(
		NewFolderAddCmd(out),
		NewFolderListCmd(out),
		NewFolderGetCmd(out),
		NewFolderRemoveCmd(out),
	)

	cli.SetColorAndStyles(cmd)
	return cmd
}
