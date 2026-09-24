package folder

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/folderutil"
)

func NewFolderCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "folder",
		Aliases: []string{"f", "fd", "fol"},
		Short:   "Manage your project folder",
	}

	cmd.PersistentFlags().StringVar(&folderutil.ArchiveDir, "archive-dir", "",
		"Where `folder archive` moves a project (default is $XDG_DATA_HOME/projekt/archive)")

	cmd.AddCommand(
		NewFolderAddCmd(out),
		NewFolderListCmd(out),
		NewFolderGetCmd(out),
		NewFolderSelectCmd(out),
		NewFolderRecentCmd(out),
		NewFolderOpenCmd(out),
		NewFolderRemoveCmd(out),
		NewFolderPruneCmd(out),
		NewFolderArchiveCmd(out),
		NewFolderSyncCmd(out),
		NewFolderExecCmd(out),
		NewFolderCheckCmd(out),
		NewFolderStatusCmd(out),
	)

	cli.SetColorAndStyles(cmd)
	return cmd
}
