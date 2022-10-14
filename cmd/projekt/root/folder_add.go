package root

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/folderutil"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

func NewFolderAddCmd(out io.Writer) *cobra.Command {
	o := &lazypath.Folder{}

	cmd := &cobra.Command{
		Use:   "add [folder path]",
		Short: "Add your project folder to config",
		Args:  cobra.ExactArgs(1),
		Aliases: []string{"a"},
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Path = args[0]
			return folderutil.ImportFolderToConfig(o)
		},
	}

	f := cmd.Flags()
	f.StringVarP(&o.Prefix, "prefix", "p", "", "Prefix of folder when call 'pj' or 'project folder go'")
	f.BoolVarP(&o.IsWorkspace, "as-workspace", "W", false, "Set folder as a workspace, like a parent folder of your projects")
	f.StringVarP(&o.RegexMatch, "regex", "R", "", "Go Regex match string to filter folder in workspace. Only work with '-W true'")
	f.Uint16VarP(&o.Priority, "priority", "P", 0, "Priority number of folder")

	return cmd
}
