package folder

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
)

func NewFolderListCmd(out io.Writer) *cobra.Command {
	o := &folderutil.ListOption{}
	output := string(folderutil.OutputTable)

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all your project folders",
		Args:    cobra.NoArgs,
		Aliases: []string{"l"},
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := folderutil.ParseOutputFormat(output)
			if err != nil {
				return err
			}
			o.Output = format

			return folderutil.ListFolders(out, o)
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&o.IsPlain, "plain", "p", false, "Show only plain folders and their prefix instead of auto parse format")
	f.BoolVarP(&o.ShortOnly, "short-only", "s", false, "When show auto parse folders, show only short name of folders")
	f.BoolVarP(&o.NoHeaders, "no-headers", "", false, "Don't print headers")
	f.BoolVarP(&o.NoColor, "no-color", "", false, "Don't use color")
	cli.AddOutputFlag(cmd, &output, output)
	registerTagsFlag(cmd, &o.Tags, "list")

	return cmd
}
