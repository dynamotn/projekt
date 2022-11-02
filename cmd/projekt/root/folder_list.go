package root

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/folderutil"
)

type listOptions struct {
	plain     bool
	shortOnly bool
	noHeaders bool
}

func NewFolderListCmd(out io.Writer) *cobra.Command {
	o := &listOptions{}
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all your project folders",
		Args:    cobra.NoArgs,
		Aliases: []string{"l"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return folderutil.ListFolders(out, o.plain, o.shortOnly, o.noHeaders)
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&o.plain, "plain", "p", false, "Show only plain folders and their prefix instead of auto parse format")
	f.BoolVarP(&o.shortOnly, "short-only", "s", false, "When show auto parse folders, show only short name of folders")
	f.BoolVarP(&o.noHeaders, "no-headers", "", false, "Don't print headers")

	return cmd
}
