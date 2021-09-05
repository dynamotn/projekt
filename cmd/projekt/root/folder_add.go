package root

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/folderutil"
)

var (
	prefix      string
	asWorkspace bool
)

func NewFolderAddCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [folder path]",
		Short: "Add your project folder to cache",
		Long:  "Add your project folder to cache",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			folderutil.ImportFolderToConfig(args[0], prefix, asWorkspace)
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVarP(&prefix, "prefix", "p", "", "Prefix of folder(s) when call `pj` or `project folder go`")
	f.BoolVarP(&asWorkspace, "as-workspace", "W", false, "Set folder as a workspace, like a parent folder of your projects")

	return cmd
}
