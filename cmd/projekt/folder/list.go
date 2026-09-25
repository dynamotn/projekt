package folder

import (
	"fmt"
	"io"
	"strings"

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
	f.StringVarP(&output, "output", "o", output,
		fmt.Sprintf("Output format, one of: %s", strings.Join(folderutil.OutputFormats, ", ")))
	registerTagsFlag(cmd, &o.Tags, "list")

	if err := cmd.RegisterFlagCompletionFunc("output",
		func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return folderutil.OutputFormats, cobra.ShellCompDirectiveNoFileComp
		}); err != nil {
		cli.Warn("Cannot register completion for --output: %v", err)
	}

	return cmd
}
