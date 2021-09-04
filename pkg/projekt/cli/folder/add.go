package folder

import (
	"github.com/spf13/cobra"

	logic "gitlab.com/dynamo.foss/projekt/pkg/projekt/folder"
)

var (
	prefix string
	asWorkspace bool
	addCmd = &cobra.Command{
		Use:   "add [folder path]",
		Short: "Add your project folder to cache",
		Long:  "Add your project folder to cache",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			logic.ImportFolderToConfig(args[0], prefix, asWorkspace)
			return nil
		},
	}
)

func init() {
	f := addCmd.Flags()
	f.StringVarP(&prefix, "prefix", "p", "", "Prefix of folder(s) when call `pj` or `project folder go`")
	f.BoolVarP(&asWorkspace, "as-workspace", "W", false, "Set folder as a workspace, like a parent folder of your projects")
}
