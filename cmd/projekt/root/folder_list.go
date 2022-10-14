package root

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/folderutil"
)

type listOptions struct {
	plain bool
}

func NewFolderListCmd(out io.Writer) *cobra.Command {
	o := &listOptions{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all your project folders",
		Args:  cobra.NoArgs,
		Aliases: []string{"l"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return folderutil.ListFolders(out, o.plain)
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&o.plain, "plain", "p", false, "Show only plain folders and their prefix instead of auto parse format")

	return cmd
}
