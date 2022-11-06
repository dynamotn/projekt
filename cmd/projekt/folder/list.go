package folder

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/folderutil"
)

func NewFolderListCmd(out io.Writer) *cobra.Command {
	o := &folderutil.ListOption{}
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all your project folders",
		Args:    cobra.NoArgs,
		Aliases: []string{"l"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return folderutil.ListFolders(out, o)
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&o.IsPlain, "plain", "p", false, "Show only plain folders and their prefix instead of auto parse format")
	f.BoolVarP(&o.ShortOnly, "short-only", "s", false, "When show auto parse folders, show only short name of folders")
	f.BoolVarP(&o.NoHeaders, "no-headers", "", false, "Don't print headers")
	f.BoolVarP(&o.NoWarnings, "no-warnings", "", false, "Don't print warnings")

	return cmd
}
